package cliconf

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

func toStructVal(rv reflect.Value) (reflect.Value, error) {
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			rv.Set(reflect.New(rv.Type().Elem()))
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return reflect.Value{}, fmt.Errorf("expected struct, got %v", rv.Kind())
	}
	return rv, nil
}

// findBooleanFlags returns a set of flag names that are boolean, i.e., take no
// parameter when set --bool vs --bool=true
func findBooleanFlags(rt reflect.Type) map[string]struct{} {
	booleans := make(map[string]struct{})
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		switch field.Type.Kind() {
		case reflect.Struct:
			subBools := findBooleanFlags(field.Type)
			for k := range subBools {
				booleans[k] = struct{}{}
			}
		case reflect.Bool:
			flagName := field.Tag.Get("flag")
			if flagName == "" {
				continue
			}
			booleans[flagName] = struct{}{}
		}
	}
	return booleans
}

type parsedTag struct {
	isBool     bool
	envName    string
	flagName   string
	remaining  bool
	optional   bool
	defaultVal *string
}

func parseField(field reflect.StructField) *parsedTag {
	tag := field.Tag
	envName := tag.Get("env")
	flagName := tag.Get("flag")
	if envName == "" && flagName == "" {
		return nil
	}

	if flagName == ",remaining" {
		return &parsedTag{
			remaining: true,
		}
	}

	defaultStr, ok := tag.Lookup("default")
	var defaultVal *string
	if ok {
		defaultVal = &defaultStr
	}

	optional := false
	if strings.ToLower(tag.Get("required")) == "false" {
		optional = true
	} else if strings.ToLower(tag.Get("optional")) == "true" {
		optional = true
	}

	return &parsedTag{
		isBool:     field.Type.Kind() == reflect.Bool,
		envName:    envName,
		flagName:   flagName,
		optional:   optional,
		defaultVal: defaultVal,
	}
}

// SetterFromEnv is used by SetFromString for custom types
type SetterFromRunner interface {
	FromRunnerString(string) error
}

// SetFromString attempts to translate a string to the given interface. Must be a pointer.
// Standard Types string, bool, int, int(8-64) float(32, 64), time.Duration and []string.
// Custom types must have method FromEnvString(string) error
func SetFromString(fieldInterface interface{}, stringVal string) error {

	if withSetter, ok := fieldInterface.(SetterFromRunner); ok {
		return withSetter.FromRunnerString(stringVal)
	}

	var err error

	switch field := fieldInterface.(type) {
	case *string:
		*field = stringVal
		return nil
	case *bool:
		bVal := strings.HasPrefix(strings.ToLower(stringVal), "t")
		*field = bVal
		return nil

	case *[]byte:
		*field = []byte(stringVal)
		return nil

	case *int:
		*field, err = strconv.Atoi(stringVal)
		return err
	case *int64:
		*field, err = strconv.ParseInt(stringVal, 10, 64)
		return err
	case *int32:
		field64, err := strconv.ParseInt(stringVal, 10, 32)
		*field = int32(field64)
		return err
	case *int16:
		field64, err := strconv.ParseInt(stringVal, 10, 16)
		*field = int16(field64)
		return err
	case *int8:
		field64, err := strconv.ParseInt(stringVal, 10, 8)
		*field = int8(field64)
		return err

	case *uint:
		field64, err := strconv.ParseUint(stringVal, 10, 64)
		*field = uint(field64)
		return err
	case *uint64:
		*field, err = strconv.ParseUint(stringVal, 10, 64)
		return err
	case *uint32:
		field64, err := strconv.ParseUint(stringVal, 10, 32)
		*field = uint32(field64)
		return err
	case *uint16:
		field64, err := strconv.ParseUint(stringVal, 10, 16)
		*field = uint16(field64)
		return err
	case *uint8:
		field64, err := strconv.ParseUint(stringVal, 10, 8)
		*field = uint8(field64)
		return err

	case *float64:
		*field, err = strconv.ParseFloat(stringVal, 64)
		return err
	case *float32:
		field64, err := strconv.ParseFloat(stringVal, 32)
		*field = float32(field64)
		return err

	case *time.Duration:
		val, err := time.ParseDuration(stringVal)
		if err != nil {
			return err
		}
		*field = val
		return nil

	// TODO: Support an array of anything. Using reflect?
	case *[]string:
		vals := strings.Split(stringVal, ",")
		out := make([]string, 0, len(vals))
		for _, val := range vals {
			stripped := strings.TrimSpace(val)
			if stripped == "" {
				continue
			}
			out = append(out, stripped)
		}
		*field = out
		return nil
	}

	return fmt.Errorf("unsupported type %T", fieldInterface)
}

type HelpLine struct {
	FlagName    string
	EnvName     string
	Description string
	Default     *string
	Required    bool
}

func GetHelpLines(rt reflect.Type) []HelpLine {
	lines := make([]HelpLine, 0, rt.NumField())
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		tag := parseField(field)
		if tag == nil {
			if field.Type.Kind() == reflect.Struct {
				subLines := GetHelpLines(field.Type)
				lines = append(lines, subLines...)
			}

			continue
		}

		lines = append(lines, HelpLine{
			FlagName:    tag.flagName,
			EnvName:     tag.envName,
			Description: field.Tag.Get("description"),
			Default:     tag.defaultVal,
			Required:    !tag.optional,
		})
	}
	return lines
}
