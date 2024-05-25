package cliconf

import (
	"reflect"
	"testing"
)

type TestConfig struct {
	Foo string `flag:"foo" env:"FOO" description:"foo description"`
	Bar string `flag:"bar" env:"BAR" default:"bar" description:"bar description"`
	Baz bool   `flag:"baz" description:"baz description"`
	NestedConfig
}

type NestedConfig struct {
	N1 string `flag:"n1" env:"N1" optional:"true"`
	N2 bool   `flag:"n2"`
}

func TestParseEntry(t *testing.T) {

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
		name: "nested",
		args: []string{"--foo=foo", "--bar=bar", "--n1=n1", "--n2"},
		expected: TestConfig{
			Foo: "foo",
			Bar: "bar",
			NestedConfig: NestedConfig{
				N1: "n1",
				N2: true,
			},
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

			gotConfig := &TestConfig{}

			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			if err := ParseCombined(reflect.ValueOf(gotConfig), tc.args); err != nil {
				t.Errorf("Expected no error, got %v", err)
				return
			}

			if gotConfig.Foo != tc.expected.Foo {
				t.Errorf("Foo: Expected %v, got %v", tc.expected.Foo, gotConfig.Foo)
			}

			if gotConfig.Bar != tc.expected.Bar {
				t.Errorf("Bar: Expected %v, got %v", tc.expected.Bar, gotConfig.Bar)
			}

			if gotConfig.NestedConfig.N1 != tc.expected.NestedConfig.N1 {
				t.Errorf("N1: Expected %v, got %v", tc.expected.NestedConfig.N1, gotConfig.NestedConfig.N1)
			}

			if gotConfig.NestedConfig.N2 != tc.expected.NestedConfig.N2 {
				t.Errorf("N2: Expected %v, got %v", tc.expected.NestedConfig.N2, gotConfig.NestedConfig.N2)
			}

		})
	}
}
