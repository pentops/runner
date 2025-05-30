package commander

import (
	"testing"
)

func TestCLIParse(t *testing.T) {
	for _, tt := range []struct {
		name        string
		input       []string
		wantCommand string
		wantArgs    []string
	}{{
		name:        "simple command",
		input:       []string{"command", "arg1", "arg2"},
		wantCommand: "command",
		wantArgs:    []string{"arg1", "arg2"},
	}, {
		name:        "command with flags",
		input:       []string{"command", "--flag1=value1", "--flag2=value2", "arg1"},
		wantCommand: "command",
		wantArgs:    []string{"--flag1=value1", "--flag2=value2", "arg1"},
	}, {
		name:        "flag before command",
		input:       []string{"--flag1=value1", "command", "arg1"},
		wantCommand: "command",
		wantArgs:    []string{"--flag1=value1", "arg1"},
	}, {
		name:        "split flags",
		input:       []string{"command", "--flag1", "value1", "--flag2", "value2", "arg1"},
		wantCommand: "command",
		wantArgs:    []string{"--flag1", "value1", "--flag2", "value2", "arg1"},
	}, {
		name:        "split flags before command",
		input:       []string{"--flag1", "value1", "command", "--flag2", "value2", "arg1"},
		wantCommand: "command",
		wantArgs:    []string{"--flag1", "value1", "--flag2", "value2", "arg1"},
	}} {
		t.Run(tt.name, func(t *testing.T) {
			gotCommand, gotArgs := parseArgs(tt.input)
			if gotCommand != tt.wantCommand {
				t.Errorf("parseArgs() gotCommand = %v, want %v", gotCommand, tt.wantCommand)
			}
			if len(gotArgs) != len(tt.wantArgs) {
				t.Errorf("parseArgs() gotArgs length = %v, want %v", len(gotArgs), len(tt.wantArgs))
			}
			for i, arg := range gotArgs {
				if arg != tt.wantArgs[i] {
					t.Errorf("parseArgs() gotArgs[%d] = %v, want %v", i, arg, tt.wantArgs[i])
				}
			}
		})

	}
}
