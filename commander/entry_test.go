package commander

import (
	"context"
	"testing"
)

type TestConfig struct {
	Foo string `flag:"foo" env:"FOO"`
	Bar string `flag:"bar" env:"BAR" default:"bar"`
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
			cc.Run(context.Background(), tc.args)

			if gotConfig.Foo != tc.expected.Foo {
				t.Errorf("Foo: Expected %v, got %v", tc.expected.Foo, gotConfig.Foo)
			}

			if gotConfig.Bar != tc.expected.Bar {
				t.Errorf("Bar: Expected %v, got %v", tc.expected.Bar, gotConfig.Bar)
			}

		})
	}

}

func TestCommandFlagParse(t *testing.T) {

	booleans := map[string]struct{}{
		"b1": {},
		"b2": {},
		"b3": {},
	}

	for _, tc := range []struct {
		name              string
		src               []string
		expected          map[string]string
		expectedRemaining []string
	}{{
		name:              "simple",
		src:               []string{"--foo", "foo", "--bar=bar"},
		expected:          map[string]string{"foo": "foo", "bar": "bar"},
		expectedRemaining: []string{},
	}, {
		name: "booleans",
		src:  []string{"--foo", "foo", "--bar=bar", "--b1", "--b2=true", "--b3", "true", "f1", "f2"},
		expected: map[string]string{
			"foo": "foo",
			"bar": "bar",
			"b1":  "true",
			"b2":  "true",
			"b3":  "true",
		},
		expectedRemaining: []string{"f1", "f2"},
	}, {
		name:     "bool at end",
		src:      []string{"--b1"},
		expected: map[string]string{"b1": "true"},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			got, gotRemaining, err := parseFlags(tc.src, booleans)
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}

			if len(got) != len(tc.expected) {
				t.Errorf("Expected %v entries, got %v", len(tc.expected), len(got))
			}

			for k, v := range tc.expected {
				if got[k] != v {
					t.Errorf("Expected %v for %v, got %v", v, k, got[k])
				}
			}

			if len(gotRemaining) != len(tc.expectedRemaining) {
				t.Errorf("Expected %v remaining args, got %v", len(tc.expectedRemaining), len(gotRemaining))
			}

			for idx, v := range tc.expectedRemaining {
				if gotRemaining[idx] != v {
					t.Errorf("Expected %v for remaining arg %v, got %v", v, idx, gotRemaining[idx])
				}
			}
		})
	}

}
