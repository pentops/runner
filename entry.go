package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type Command[F any, E any] struct {
	Callback func(context.Context, F, E) error
}

func NewCommand[F any, E any](callback func(context.Context, F, E) error) *Command[F, E] {
	return &Command[F, E]{Callback: callback}
}

func (cc *Command[F, E]) Run(ctx context.Context, args []string) error {
	flagConfig := new(F)
	envConfig := new(E)

	flagValue := reflect.ValueOf(flagConfig).Elem()
	booleans := make(map[string]struct{})
	for i := 0; i < flagValue.NumField(); i++ {
		field := flagValue.Type().Field(i)
		if field.Type.Kind() != reflect.Bool {
			continue
		}
		flagName := field.Tag.Get("flag")
		if flagName == "" {
			continue
		}
		booleans[flagName] = struct{}{}
	}

	flagMap, remainingArgs, err := parseFlags(args, booleans)
	if err != nil {
		return err
	}

	if err := parse("flag", flagValue, func(s string) string {
		if val, ok := flagMap[s]; ok {
			return val
		}
		return ""
	}, remainingArgs); err != nil {
		return err
	}

	envValue := reflect.ValueOf(envConfig).Elem()
	if err := parse("env", envValue, os.Getenv, nil); err != nil {
		return err
	}

	return cc.Callback(ctx, *flagConfig, *envConfig)
}

func parseFlags(src []string, booleans map[string]struct{}) (map[string]string, []string, error) {
	flagMap := make(map[string]string)

	for len(src) > 0 {
		arg := src[0]
		if !strings.HasPrefix(arg, "-") {
			// once the first non -- or - arg is found, the rest are treated as
			// plain args
			return flagMap, src, nil
		}
		arg = strings.TrimPrefix(arg, "-")
		arg = strings.TrimPrefix(arg, "-")
		src = src[1:]

		if _, ok := booleans[arg]; ok {
			if len(src) == 0 || strings.HasPrefix(src[0], "-") {
				flagMap[arg] = "true"
			}
			lower := strings.ToLower(src[0])
			// Consume a flag for true or false only.
			// Being too flexible here can lead to unexpected behavior, e.g. if
			// we accept 't' and 'yes' etc, then a user might accidentally pass
			// a remaining flag that starts with 't' and it will be interpreted as true.
			// In the flag package, the first remaining will be skipped if the
			// last specified flag is a boolean, regardless of the specified
			// value
			if lower == "true" || lower == "false" {
				flagMap[arg] = lower
				src = src[1:]
			}

			continue
		}

		eqSplit := strings.SplitN(arg, "=", 2)
		if len(eqSplit) == 2 {
			flagMap[eqSplit[0]] = eqSplit[1]
			continue
		}

		if len(src) == 0 {
			return nil, nil, fmt.Errorf("flag %v has no value", arg)
		}

		val := src[0]
		src = src[1:]
		flagMap[arg] = val
	}
	return flagMap, []string{}, nil
}

func parse(tagName string, rv reflect.Value, resolver func(string) string, remainingArgs []string) error {
	rt := rv.Type()
	for i := 0; i < rv.NumField(); i++ {
		tag := rt.Field(i).Tag
		envName := tag.Get(tagName)
		if envName == "" {
			continue
		}
		if envName == ",remaining" {
			if rv.Field(i).Kind() != reflect.Slice {
				return fmt.Errorf("remaining args must be a slice")
			}
			if rv.Field(i).Type().Elem().Kind() != reflect.String {
				return fmt.Errorf("remaining args must be a slice of strings")
			}
			rv.Field(i).Set(reflect.ValueOf(remainingArgs))
			continue
		}

		envVal := resolver(envName)
		if envVal == "" {
			if defaultValue, ok := tag.Lookup("default"); ok {
				envVal = defaultValue
			} else if req, ok := tag.Lookup("required"); ok && req == "false" {
				continue
			} else {
				return fmt.Errorf("Required ENV var not set: %v", tag)
			}
		}

		fieldVal := rv.Field(i)

		fieldInterface := fieldVal.Addr().Interface()

		actualType := fieldVal.Kind()
		if actualType == reflect.Pointer {
			elemType := fieldVal.Type().Elem()
			newVal := reflect.New(elemType)
			fieldVal.Set(newVal)
			fieldVal = newVal
			actualType = fieldVal.Elem().Kind()
		}

		if actualType == reflect.Struct {
			if !strings.HasPrefix(envVal, "{") {
				return fmt.Errorf("In field %s: struct fields should be set using JSON strings", envName)
			}

			if err := json.Unmarshal([]byte(envVal), fieldInterface); err != nil {
				return err
			}
			continue
		}

		if err := SetFromString(fieldInterface, envVal); err != nil {
			return fmt.Errorf("In field %s: %s", envName, err)
		}

	}
	return nil

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
