package cliconf

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseField(t *testing.T) {

	type Nested struct {
		N1 string `flag:"n1" env:"N1" optional:"true"`
		N2 bool   `flag:"n2"`
	}

	type Input struct {
		Foo   string `flag:"foo" env:"FOO"`
		Bar   string `flag:"bar" env:"BAR" default:"bar"`
		Baz   bool   `flag:"baz" description:"baz description"`
		Doo   string `flag:"doo" env:"DOO" required:"false"`
		NoTag string
		Nested
	}

	byName := make(map[string]*field)

	allFields, err := findStructFields(reflect.ValueOf(Input{}))
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	for _, field := range allFields {
		byName[field.fieldName] = field
		t.Logf("Field: %s, %v", field.fieldName, field)
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

	n1, ok := byName["Nested.N1"]
	if !ok {
		t.Errorf("Expected 'Nested.N1' to be present")
	} else {
		assert.Equal(t, "n1", n1.flagName)
	}

}

func TestSetFromString(t *testing.T) {

	t.Run("string", func(t *testing.T) {
		val := ""
		if err := SetFromString(&val, "foo"); err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if val != "foo" {
			t.Errorf("Expected 'foo', got %v", val)
		}
	})

	t.Run("bytes", func(t *testing.T) {
		val := []byte{}
		if err := SetFromString(&val, "foo"); err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if string(val) != "foo" {
			t.Errorf("Expected 'foo', got %v", val)
		}
	})
}
