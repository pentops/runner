package commander

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/pentops/log.go/log"
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
	ctx := log.WithFields(context.Background(), map[string]interface{}{
		"app":     name,
		"version": version,
	})
	ok := cs.runMain(ctx, os.Stderr, os.Args)
	if !ok {
		os.Exit(1)
	}
}

func (cs *CommandSet) runMain(ctx context.Context, errOut io.Writer, args []string) bool {
	if len(args) < 2 {
		fmt.Fprintf(errOut, "Usage: %s <command> [options]\n", args[0])
		cs.printCommands(errOut, "  ")
		return false
	}

	command, ok := cs.findCommand(args[1])
	if !ok {
		fmt.Fprintf(errOut, "Unknown command: '%s'\n", args[1])
		cs.printCommands(errOut, "  ")
		return false
	}

	mainErr := command.command.Run(ctx, args[2:])
	if mainErr != nil {
		if helpError := new(HelpError); errors.As(mainErr, helpError) {
			fmt.Fprintf(errOut, "Usage: %s %s %s\n", args[0], args[1], helpError.Usage)
			for _, line := range helpError.Lines {
				fmt.Fprintf(errOut, "%s\n", line)
			}
			return false
		}
		if flagErr := new(FlagError); errors.As(mainErr, flagErr) {
			flagErrString := strings.Replace(flagErr.Error(), "$0", strings.Join(args[0:2], " "), -1)
			fmt.Fprintln(errOut, flagErrString)
			return false
		}
		fmt.Printf("A passthrough error: %T\n", mainErr)

		fmt.Fprintf(errOut, "Error running command: %s\n", mainErr)
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

	command, ok := cs.findCommand(args[0])
	if !ok {
		return HelpError{
			Lines: cs.listCommands("  "),
		}
	}

	mainErr := command.command.Run(ctx, args[1:])
	if mainErr != nil {
		if helpError := new(HelpError); errors.As(mainErr, helpError) {
			fmt.Printf("A Returning help error\n")
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
