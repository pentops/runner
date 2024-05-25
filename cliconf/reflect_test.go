package cliconf

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBoolSearch(t *testing.T) {
	type Nested struct {
		NestBool bool `flag:"nest"`
	}

	type Input struct {
		Foo bool `flag:"foo" env:"FOO"`
		Bar bool
		Baz bool `flag:"baz" description:"baz description"`
		Nested
	}

	bools := findBooleanFlags(reflect.TypeOf(Input{}))

	if len(bools) != 3 {
		t.Errorf("Expected 3, got %v", len(bools))
	}

	if _, ok := bools["baz"]; !ok {
		t.Errorf("Expected 'baz' to be present")
	}
	if _, ok := bools["foo"]; !ok {
		t.Errorf("Expected 'foo' to be present")
	}
	if _, ok := bools["nest"]; !ok {
		t.Errorf("Expected 'nest' to be present")
	}
}

func TestParseField(t *testing.T) {

	type Input struct {
		Foo   string `flag:"foo" env:"FOO"`
		Bar   string `flag:"bar" env:"BAR" default:"bar"`
		Baz   bool   `flag:"baz" description:"baz description"`
		Doo   string `flag:"doo" env:"DOO" required:"false"`
		NoTag string
	}

	byName := make(map[string]*parsedTag)
	rt := reflect.TypeOf(Input{})
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		tag := parseField(field)
		if tag == nil {
			continue
		}
		byName[field.Name] = tag
		t.Logf("Field: %s, %v", field.Name, tag)
	}

	foo, ok := byName["Foo"]
	if !ok {
		t.Errorf("Expected 'Foo' to be present")
	} else {
		assert.Equal(t, "foo", foo.flagName)
		assert.Equal(t, "FOO", foo.envName)
		assert.Equal(t, false, foo.isBool)
		assert.Equal(t, false, foo.remaining)
		assert.Equal(t, false, foo.optional)
		assert.Nil(t, foo.defaultVal)
	}

	bar, ok := byName["Bar"]
	if !ok {
		t.Errorf("Expected 'Bar' to be present")
	} else {
		assert.Equal(t, "bar", *bar.defaultVal)
	}

	baz, ok := byName["Baz"]
	if !ok {
		t.Errorf("Expected 'Baz' to be present")
	} else {
		assert.Equal(t, "baz", baz.flagName)
		assert.Equal(t, "", baz.envName)
		assert.Equal(t, true, baz.isBool)
	}

	doo, ok := byName["Doo"]
	if !ok {
		t.Errorf("Expected 'Doo' to be present")
	} else {
		assert.Equal(t, true, doo.optional)
	}

}

func TestSetFromString(t *testing.T) {

	t.Run("string", func(t *testing.T) {
		val := ""
		SetFromString(&val, "foo")
		if val != "foo" {
			t.Errorf("Expected 'foo', got %v", val)
		}
	})

	t.Run("bytes", func(t *testing.T) {
		val := []byte{}
		SetFromString(&val, "foo")
		if string(val) != "foo" {
			t.Errorf("Expected 'foo', got %v", val)
		}
	})
}
