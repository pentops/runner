package commander

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/pentops/runner/cliconf"
)

type Command[C any] struct {
	Callback func(context.Context, C) error
	CommandOption
}

type CommandOption struct {
	description     string
	outcomeCallback func(context.Context, error)
}

func WithDescription(description string) func(*CommandOption) {
	return func(co *CommandOption) {
		co.description = description
	}
}

func WithOutcomeCallback(outcomeCallback func(context.Context, error)) func(*CommandOption) {
	return func(co *CommandOption) {
		co.outcomeCallback = outcomeCallback
	}
}

func NewCommand[C any](callback func(context.Context, C) error, options ...func(*CommandOption)) *Command[C] {
	option := CommandOption{}
	for _, opt := range options {
		opt(&option)
	}

	return &Command[C]{
		Callback:      callback,
		CommandOption: option,
	}
}

func (cc *Command[C]) helpLines(prefix string) []string {
	config := new(C)
	rt := reflect.ValueOf(config).Elem().Type()
	helpTags := cliconf.GetHelpLines(rt)
	lines := make([][]string, 0, rt.NumField())
	for _, tag := range helpTags {
		description := tag.Description

		if tag.Default != nil {
			description += fmt.Sprintf(" (default: %s)", *tag.Default)
		}

		name := tag.Name()

		lines = append(lines, []string{name, description})
	}
	return evenJoin(prefix, lines)
}

func (cc *Command[C]) Help() string {
	lines := cc.helpLines("  ")
	return cc.description + "\n" + strings.Join(lines, "\n")
}

type HelpError struct {
	Usage string
	Lines []string
}

func (he HelpError) Error() string {
	return strings.Join(he.Lines, "\n")
}

func (cc *Command[C]) Run(ctx context.Context, args []string) error {
	config := new(C)
	configValue := reflect.ValueOf(config).Elem()

	parseError := cliconf.ParseCombined(configValue, args)
	if parseError != nil {
		if paramErrors := new(cliconf.ParamErrors); errors.As(parseError, paramErrors) {
			lines := make([]string, 0, len(*paramErrors))
			for _, err := range *paramErrors {
				name := err.Name()
				lines = append(lines, fmt.Sprintf("  %s : %s", name, err.Err))
			}

			lines = append(lines, "Flags and Env Vars:")
			lines = append(lines, cc.helpLines("  ")...)

			return HelpError{
				Usage: "[options]",
				Lines: lines,
			}
		}
		return parseError
	}

	mainErr := cc.Callback(ctx, *config)
	if cc.outcomeCallback != nil {
		cc.outcomeCallback(ctx, mainErr)
	}
	return mainErr
}
