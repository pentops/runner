package commander

import (
	"context"
	"fmt"
	"os"

	"github.com/pentops/log.go/log"
)

type Runnable interface {
	Run(ctx context.Context, args []string) error
	Help() string
}

type CommandSet struct {
	commands map[string]Runnable
}

func NewCommandSet() *CommandSet {
	return &CommandSet{commands: make(map[string]Runnable)}
}

func (cs *CommandSet) Add(name string, command Runnable) {
	cs.commands[name] = command
}

func (cs *CommandSet) help() string {
	var help string
	for name, command := range cs.commands {
		help += name + "\t" + command.Help() + "\n"
	}
	return help
}

func (cs *CommandSet) helpExit(ctx context.Context, args []string) error {
	helpString := cs.help()
	fmt.Println(helpString)
	os.Exit(1)
	return nil
}

// RunMain should run from the main command, it will handle OS Exits, and should
// be the only goroutine running.
func (cs *CommandSet) RunMain(name, version string) {
	ctx := log.WithFields(context.Background(), map[string]interface{}{
		"app":     name,
		"version": version,
	})
	args := os.Args

	if len(args) < 2 {
		cs.helpExit(ctx, args)
		return
	}
	command, ok := cs.commands[args[1]]
	if !ok {
		cs.helpExit(ctx, args)
		return
	}

	mainErr := command.Run(ctx, args[2:])
	if mainErr != nil {
		fmt.Println(mainErr)
		os.Exit(1)
	}
}
