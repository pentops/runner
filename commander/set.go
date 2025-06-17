package commander

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/pentops/log.go/log"
	"github.com/pentops/runner/cliconf"
)

type Runnable interface {
	Run(ctx context.Context, args []string) error
	Help() string
}

type CommandSet struct {
	commands []namedRunnable
}

type namedRunnable struct {
	name        string
	command     Runnable
	description string
}

func NewCommandSet() *CommandSet {
	return &CommandSet{}
}

func CommandWithDescription(description string) func(*namedRunnable) {
	return func(nr *namedRunnable) {
		nr.description = description
	}
}

func (cs *CommandSet) Add(name string, command Runnable, options ...func(*namedRunnable)) {
	nr := namedRunnable{
		name:        name,
		command:     command,
		description: "",
	}

	for _, opt := range options {
		opt(&nr)
	}

	cs.commands = append(cs.commands, nr)
}

type commandDescriptor interface {
	CommandDescriptions() [][]string
}

func (cs *CommandSet) CommandDescriptions() [][]string {
	descriptions := make([][]string, 0, len(cs.commands))
	for _, command := range cs.commands {
		descriptions = append(descriptions, []string{command.name, command.description})
		if wd, ok := command.command.(commandDescriptor); ok {
			for _, subCommand := range wd.CommandDescriptions() {
				subCommand[0] = " | " + subCommand[0]
				descriptions = append(descriptions, subCommand)
			}
		}
	}
	return descriptions
}

func (cs *CommandSet) Help() string {
	buf := &strings.Builder{}
	cs.printCommands(buf, "")
	out := buf.String()
	out = strings.TrimSuffix(out, "\n")
	return out
}

func (cs *CommandSet) printCommands(out io.Writer, prefix string) {
	lines := cs.listCommands(prefix)
	for _, line := range lines {
		fmt.Fprintln(out, line)
	}
}

func (cs *CommandSet) listCommands(prefix string) []string {
	lines := cs.CommandDescriptions()
	return evenJoin(prefix, lines)
}

func evenJoin(prefix string, lines [][]string) []string {
	maxLen := 0
	for _, command := range lines {
		if len(command[0]) > maxLen {
			maxLen = len(command[0])
		}
	}
	linesOut := make([]string, len(lines))

	for idx, command := range lines {
		linesOut[idx] = fmt.Sprintf(prefix+"%-*s - %s", maxLen, command[0], strings.Join(command[1:], "  "))
	}
	return linesOut
}

// RunMain should run from the main command, it will handle OS Exits, and should
// be the only goroutine running.
func (cs *CommandSet) RunMain(name, version string) {
	ctx := context.Background()
	ctx = log.WithFields(ctx, map[string]any{
		"app":     name,
		"version": version,
	})
	ctx, stop := signal.NotifyContext(ctx,
		os.Interrupt,
		os.Kill,
		os.Signal(syscall.SIGTERM),
	)

	ok := cs.runMain(ctx, os.Stderr, os.Args)
	stop()
	if !ok {
		os.Exit(1)
	}
}

// parseArgs returns the command name and remaining arguments - it moves any
// --key=value pairs to the start of the args slice, to give the illusion of
// 'global' flags. However, booleans need to be explicitly set to true, as in
// --foo=true or --foo false
func parseArgs(args []string) (string, []string) {
	if len(args) == 0 {
		return "", nil
	}
	out := make([]string, 0)
	rem := args[:]
	for len(rem) > 0 {
		if !strings.HasPrefix(rem[0], "-") {
			break
		}
		arg := rem[0]
		rem = rem[1:]
		out = append(out, arg)
		if strings.Contains(arg, "=") {
			// --arg=value
			continue
		}
		// --arg value
		if len(rem) == 0 {
			break
		}
		out = append(out, rem[0])
		rem = rem[1:]
	}
	if len(rem) == 0 {
		return "", out
	}
	cmd := rem[0]
	out = append(out, rem[1:]...)
	return cmd, out

}

func (cs *CommandSet) runMain(ctx context.Context, errOut io.Writer, args []string) bool {
	cliName := args[0]
	if len(args) < 2 {
		fmt.Fprintf(errOut, "Usage: %s <command> [options]\n", cliName)
		cs.printCommands(errOut, "  ")
		return false
	}

	commandName, remaining := parseArgs(args[1:])
	command, ok := cs.findCommand(commandName)
	if !ok {
		fmt.Fprintf(errOut, "Unknown command: '%s'\n", commandName)
		cs.printCommands(errOut, "  ")
		return false
	}

	mainErr := command.command.Run(ctx, remaining)
	if mainErr != nil {
		if helpError := new(HelpError); errors.As(mainErr, helpError) {
			fmt.Fprintf(errOut, "Usage: %s %s %s\n", cliName, commandName, helpError.Usage)
			for _, line := range helpError.Lines {
				fmt.Fprintf(errOut, "%s\n", line)
			}
			return false
		}
		if flagErr := new(cliconf.FlagError); errors.As(mainErr, flagErr) {
			flagErrString := strings.ReplaceAll(flagErr.Error(), "$0", strings.Join(args[0:2], " "))
			fmt.Fprintln(errOut, flagErrString)
			return false
		}

		fmt.Fprintf(errOut, "Command %q returned error\n%s\n", commandName, mainErr)
		return false
	}
	return true
}

func (cs *CommandSet) Run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return HelpError{
			Usage: "<command> [options]",
			Lines: cs.listCommands("  "),
		}
	}

	commandName, remaining := parseArgs(args)

	command, ok := cs.findCommand(commandName)
	if !ok {
		return HelpError{
			Lines: cs.listCommands("  "),
		}
	}

	mainErr := command.command.Run(ctx, remaining)
	if mainErr != nil {
		if helpError := new(HelpError); errors.As(mainErr, helpError) {
			helpError.Usage = command.name + " " + helpError.Usage
			return *helpError
		}
		return mainErr
	}
	return nil
}

func (cs *CommandSet) findCommand(name string) (*namedRunnable, bool) {
	for _, search := range cs.commands {
		if search.name == name {
			return &search, true
		}
	}
	return nil, false
}
