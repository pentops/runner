package cliconf

import "testing"

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
