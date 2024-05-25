package commander

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

type TestConfig struct {
	Foo string `flag:"foo" env:"FOO" description:"foo description"`
	Bar string `flag:"bar" env:"BAR" default:"bar" description:"bar description"`
}

func TestCommandHappy(t *testing.T) {

	for _, tc := range []struct {
		name     string
		args     []string
		env      map[string]string
		expected TestConfig
	}{{
		name: "flags",
		args: []string{"--foo=foo", "--bar=bar"},
		expected: TestConfig{
			Foo: "foo",
			Bar: "bar",
		},
	}, {
		name: "env",
		env: map[string]string{
			"FOO": "foo",
			"BAR": "bar",
		},
		expected: TestConfig{
			Foo: "foo",
			Bar: "bar",
		},
	}, {
		name: "flag overrides env",
		args: []string{"--foo=foo", "--bar=bar"},
		env: map[string]string{
			"FOO": "foo2",
			"BAR": "bar2",
		},
		expected: TestConfig{
			Foo: "foo",
			Bar: "bar",
		},
	}, {
		name: "default",
		args: []string{"--foo=foo"},
		expected: TestConfig{
			Foo: "foo",
			Bar: "bar",
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {

			var gotConfig TestConfig

			t.Setenv("FOO", "foo")
			cc := NewCommand(func(ctx context.Context, cfg TestConfig) error {
				t.Log(cfg)
				gotConfig = cfg
				return nil
			})

			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			if err := cc.Run(context.Background(), tc.args); err != nil {
				t.Errorf("Expected no error, got %v", err)
			}

			if gotConfig.Foo != tc.expected.Foo {
				t.Errorf("Foo: Expected %v, got %v", tc.expected.Foo, gotConfig.Foo)
			}

			if gotConfig.Bar != tc.expected.Bar {
				t.Errorf("Bar: Expected %v, got %v", tc.expected.Bar, gotConfig.Bar)
			}

		})
	}
}

func TestNested(t *testing.T) {

	var fooConfig *TestConfig
	var barConfig *TestConfig

	root := NewCommandSet()
	root.Add("foo", NewCommand(func(ctx context.Context, cfg TestConfig) error {
		fooConfig = &cfg
		return nil
	}))

	sub := NewCommandSet()
	sub.Add("bar", NewCommand(func(ctx context.Context, cfg TestConfig) error {
		barConfig = &cfg
		return nil
	}))
	sub.Add("baz", NewCommand(func(ctx context.Context, cfg TestConfig) error {
		return nil
	}))

	root.Add("sub", sub)

	err := root.Run(context.Background(), []string{"foo", "--foo=1"})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	err = root.Run(context.Background(), []string{"sub", "bar", "--foo=2"})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if fooConfig == nil {
		t.Errorf("Expected fooConfig to be set")
	} else if fooConfig.Foo != "1" {
		t.Errorf("Expected fooConfig.Foo to be 1, got %v", fooConfig.Foo)
	}

	if barConfig == nil {
		t.Errorf("Expected barConfig to be set")
	} else if barConfig.Foo != "2" {
		t.Errorf("Expected barConfig.Foo to be 2, got %v", barConfig.Foo)
	}

	err = root.Run(context.Background(), []string{"foo", "--foo", "foo", "--bad", "f"})
	if err == nil {
		t.Errorf("Expected error, got nil")
	}

}

func TestSetHelp(t *testing.T) {

	nilFunc := func(ctx context.Context, cfg TestConfig) error {
		return nil
	}

	root := NewCommandSet()
	root.Add("name", NewCommand(nilFunc), CommandWithDescription("foo description"))

	sub := NewCommandSet()
	sub.Add("sub-1", NewCommand(nilFunc), CommandWithDescription("sub-1 description"))
	sub.Add("sub-two", NewCommand(nilFunc), CommandWithDescription("sub-2 description"))

	doubleSub := NewCommandSet()
	doubleSub.Add("asdf", NewCommand(nilFunc), CommandWithDescription("asdf description"))
	sub.Add("nest", doubleSub, CommandWithDescription("nest description"))

	root.Add("longer-name", sub, CommandWithDescription("sub description"))

	t.Run("Root Help", func(t *testing.T) {
		compareLines(t, root.Help(),
			"name        - foo description",
			"longer-name - sub description",
			" | sub-1    - sub-1 description",
			" | sub-two  - sub-2 description",
			" | nest     - nest description",
			" |  | asdf  - asdf description",
		)
	})

	t.Run("No first arg", func(t *testing.T) {
		capture := &bytes.Buffer{}
		root.runMain(context.Background(), capture, []string{"test"})
		compareLines(t, capture.String(),
			"Usage: test <command> [options]",
			"  name        - foo description",
			"  longer-name - sub description",
			"   | sub-1    - sub-1 description",
			"   | sub-two  - sub-2 description",
			"   | nest     - nest description",
			"   |  | asdf  - asdf description",
			"",
		)
	})

	t.Run("Unknown command", func(t *testing.T) {
		capture := &bytes.Buffer{}
		root.runMain(context.Background(), capture, []string{"test", "unknown"})
		compareLines(t, capture.String(),
			"Unknown command: 'unknown'",
			"  name        - foo description",
			"  longer-name - sub description",
			"   | sub-1    - sub-1 description",
			"   | sub-two  - sub-2 description",
			"   | nest     - nest description",
			"   |  | asdf  - asdf description",
			"",
		)
	})

	t.Run("No sub command", func(t *testing.T) {
		capture := &bytes.Buffer{}
		root.runMain(context.Background(), capture, []string{"test", "longer-name"})
		compareLines(t, capture.String(),
			"Usage: test longer-name <command> [options]",
			"  sub-1   - sub-1 description",
			"  sub-two - sub-2 description",
			"  nest    - nest description",
			"   | asdf - asdf description",
			"",
		)
	})

	t.Run("Missing Flag Root", func(t *testing.T) {
		capture := &bytes.Buffer{}
		root.runMain(context.Background(), capture, []string{"test", "name"})
		compareLines(t, capture.String(),
			"Usage: test name [options]",
			"  --foo / $FOO : required",
			"Flags and Env Vars:",
			"  --foo / $FOO - foo description",
			"  --bar / $BAR - bar description (default: bar)",
			"",
		)
	})

	t.Run("Missing Flag Sub", func(t *testing.T) {
		capture := &bytes.Buffer{}
		root.runMain(context.Background(), capture, []string{"test", "longer-name", "sub-1"})
		compareLines(t, capture.String(),
			"Usage: test longer-name sub-1 [options]",
			"  --foo / $FOO : required",
			"Flags and Env Vars:",
			"  --foo / $FOO - foo description",
			"  --bar / $BAR - bar description (default: bar)",
			"",
		)
	})

}

func TestCommandHelp(t *testing.T) {

	nilFunc := func(ctx context.Context, cfg TestConfig) error {
		return nil
	}

	cc := NewCommand(nilFunc, WithDescription("foo description"))

	helpString := cc.Help()
	compareLines(t, helpString,
		"foo description",
		"  --foo / $FOO - foo description",
		"  --bar / $BAR - bar description (default: bar)",
	)

}

func compareLines(t *testing.T, got string, wantLines ...string) {
	gotLines := strings.Split(got, "\n")
	t.Log("Compare Lines")

	for idx, wantLine := range wantLines {
		t.Logf("Line %03d: '%v'", idx, wantLine)
		if len(gotLines) <= idx {
			t.Errorf("Missing Line")
		} else if gotLines[idx] != wantLine {
			t.Errorf(" GOT %03d: '%v'", idx, gotLines[idx])
		}

	}

	for idx := len(wantLines); idx < len(gotLines); idx++ {
		t.Errorf("Extra Line: '%v'", gotLines[idx])
	}

}
