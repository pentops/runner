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

func findStructFields(rv reflect.Value) ([]*field, error) {
	rt := rv.Type()

	fields := make([]*field, 0)

	for i := 0; i < rv.NumField(); i++ {
		fieldType := rt.Field(i)
		fieldValue := rv.Field(i)
		parsed, err := structField(fieldType, fieldValue)
		if err != nil {
			return nil, err
		}
		if parsed != nil {
			fields = append(fields, parsed)
		}

		if fieldType.Type.Kind() != reflect.Struct {
			continue
		}
		subStruct, err := toStructVal(fieldValue)
		if err != nil {
			// INVERSION
			continue
		}

		subFields, err := findStructFields(subStruct)
		if err != nil {
			return nil, err
		}
		for _, subField := range subFields {
			subField.fieldName = fieldType.Name + "." + subField.fieldName
			fields = append(fields, subField)
		}
	}

	return fields, nil
}

type field struct {
	fieldName  string
	isBool     bool
	optional   bool
	defaultVal *string
	fieldVal   reflect.Value

	// one of the following
	// - envName and/or flagName
	// - argN
	// - remaining

	envName  string
	flagName string

	remaining bool
	argn      *int
}

func structField(inputField reflect.StructField, val reflect.Value) (*field, error) {
	tag := inputField.Tag
	envName := tag.Get("env")
	flagName := tag.Get("flag")
	if envName == "" && flagName == "" {
		return nil, nil
	}

	parts := strings.SplitN(flagName, ",", 2)
	flagName = parts[0]
	parsed := &field{
		isBool:    inputField.Type.Kind() == reflect.Bool,
		envName:   envName,
		flagName:  flagName,
		fieldName: inputField.Name,
		fieldVal:  val,
	}

	if len(parts) == 2 {
		flagFlag := parts[1]

		if flagFlag == "remaining" {
			if flagName != "" {
				return nil, fmt.Errorf("param name %q cannot be used with ,remaining", flagName)
			}
			if inputField.Type.Kind() != reflect.Slice {
				return nil, fmt.Errorf("remaining args must be a slice")
			}
			if inputField.Type.Elem().Kind() != reflect.String {
				return nil, fmt.Errorf("remaining args must be a slice of strings")
			}
			parsed.remaining = true
		} else if strings.HasPrefix(flagFlag, "arg") {
			if flagName != "" {
				return nil, fmt.Errorf("param name %q cannot be used with ,argN", flagName)
			}
			argn, err := strconv.Atoi(strings.TrimPrefix(flagFlag, "arg"))
			if err != nil {
				return nil, fmt.Errorf("invalid arg number %q", flagFlag)
			}
			parsed.argn = &argn
		}
	}

	defaultStr, ok := tag.Lookup("default")
	if ok {
		parsed.defaultVal = &defaultStr
	}

	if strings.ToLower(tag.Get("required")) == "false" {
		parsed.optional = true
	} else if strings.ToLower(tag.Get("optional")) == "true" {
		parsed.optional = true
	}

	return parsed, nil

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
	FlagName  string
	EnvName   string
	ArgN      *int
	Remaining bool

	Description string
	Default     *string
	Required    bool
}

func GetHelpLines(rt reflect.Type) []HelpLine {
	lines := make([]HelpLine, 0, rt.NumField())
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		tag, err := structField(field, reflect.Value{})
		if err != nil {
			panic(err)
		}
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
			ArgN:        tag.argn,
			Remaining:   tag.remaining,
		})
	}
	return lines
}
