package commander

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type Command[C any] struct {
	Callback func(context.Context, C) error
	CommandOption
}

type CommandOption struct {
	description     string
	outcomeCallback func(context.Context, error)
}

func WithDescription(description string) func(*CommandOption) {
	return func(co *CommandOption) {
		co.description = description
	}
}

func WithOutcomeCallback(outcomeCallback func(context.Context, error)) func(*CommandOption) {
	return func(co *CommandOption) {
		co.outcomeCallback = outcomeCallback
	}
}

func NewCommand[C any](callback func(context.Context, C) error, options ...func(*CommandOption)) *Command[C] {
	option := CommandOption{}
	for _, opt := range options {
		opt(&option)
	}

	return &Command[C]{
		Callback:      callback,
		CommandOption: option,
	}
}

func (cc *Command[C]) helpLines(prefix string) []string {
	config := new(C)
	rt := reflect.ValueOf(config).Elem().Type()
	lines := make([][]string, 0, rt.NumField())
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		description := field.Tag.Get("description")
		flagName := field.Tag.Get("flag")
		envName := field.Tag.Get("env")
		defaultValue := field.Tag.Get("default")

		if flagName == "" && envName == "" {
			continue
		}

		if defaultValue != "" {
			description += fmt.Sprintf(" (default: %s)", defaultValue)
		}

		name := ""
		if flagName != "" && envName != "" {
			name = fmt.Sprintf("--%s / $%s", flagName, envName)
		} else if flagName != "" {
			name = fmt.Sprintf("--%s", flagName)
		} else if envName != "" {
			name = fmt.Sprintf("$%s", envName)
		}

		lines = append(lines, []string{name, description})
	}
	return evenJoin(prefix, lines)
}

func (cc *Command[C]) Help() string {
	lines := cc.helpLines("  ")
	return cc.description + "\n" + strings.Join(lines, "\n")
}

type HelpError struct {
	Usage string
	Lines []string
}

func (he HelpError) Error() string {
	return strings.Join(he.Lines, "\n")
}

func (cc *Command[C]) Run(ctx context.Context, args []string) error {
	config := new(C)
	configValue := reflect.ValueOf(config).Elem()

	parseError := parse(configValue, args)
	if parseError != nil {
		if paramErrors := new(ParamErrors); errors.As(parseError, paramErrors) {
			lines := make([]string, 0, len(*paramErrors))
			for _, err := range *paramErrors {
				var name string
				if err.Flag != "" && err.Env != "" {
					name = fmt.Sprintf("--%s / $%s", err.Flag, err.Env)
				} else if err.Flag != "" {
					name = fmt.Sprintf("--%s", err.Flag)
				} else if err.Env != "" {
					name = fmt.Sprintf("$%s", err.Env)
				} else if err.FieldName != "" {
					name = err.FieldName
				} else {
					name = "<unknown>"
				}
				lines = append(lines, fmt.Sprintf("  %s : %s", name, err.Err))
			}

			lines = append(lines, "Flags and Env Vars:")
			lines = append(lines, cc.helpLines("  ")...)

			return HelpError{
				Usage: "[options]",
				Lines: lines,
			}
		}
		return parseError
	}

	mainErr := cc.Callback(ctx, *config)
	if cc.outcomeCallback != nil {
		cc.outcomeCallback(ctx, mainErr)
	}
	return mainErr
}

func parseFlags(src []string, booleans map[string]struct{}) (map[string]string, []string, ParamErrors) {
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
				continue
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
			return nil, nil, ParamErrors{{
				Flag: arg,
				Err:  fmt.Errorf("flag has no value"),
			}}
		}

		val := src[0]
		src = src[1:]
		flagMap[arg] = val
	}
	return flagMap, []string{}, nil
}

type ParamError struct {
	Flag      string
	Env       string
	FieldName string
	Err       error
}

func (pe ParamError) Error() string {
	return fmt.Sprintf("Error parsing %s: %s", pe.FieldName, pe.Err)
}

type ParamErrors []ParamError

func (pe ParamErrors) Error() string {
	var out string
	for _, err := range pe {
		out += fmt.Sprintf("Error parsing %s: %s\n", err.FieldName, err.Err)
	}
	return out
}

func parse(rv reflect.Value, args []string) error {

	errs := make(ParamErrors, 0)

	booleans := make(map[string]struct{})
	for i := 0; i < rv.NumField(); i++ {
		field := rv.Type().Field(i)
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

	rt := rv.Type()
	for i := 0; i < rv.NumField(); i++ {
		tag := rt.Field(i).Tag
		envName := tag.Get("env")
		flagName := tag.Get("flag")
		if envName == "" && flagName == "" {
			continue
		}

		if flagName == ",remaining" {
			if rv.Field(i).Kind() != reflect.Slice {
				return fmt.Errorf("remaining args must be a slice")
			}
			if rv.Field(i).Type().Elem().Kind() != reflect.String {
				return fmt.Errorf("remaining args must be a slice of strings")
			}
			rv.Field(i).Set(reflect.ValueOf(remainingArgs))
			continue
		}

		var stringValue string
		if envName != "" {
			stringValue = os.Getenv(envName)
		}

		if flagName != "" {
			// flags can override env vars
			flagVal, ok := flagMap[flagName]
			if ok {
				stringValue = flagVal
				delete(flagMap, flagName)
			}
		}

		if stringValue == "" {
			if rt.Field(i).Type.Kind() == reflect.Bool {
				// leave it false
				continue
			}

			if defaultValue, ok := tag.Lookup("default"); ok {
				stringValue = defaultValue
			} else if req, ok := tag.Lookup("required"); ok && req == "false" {
				continue
			} else {
				errs = append(errs, ParamError{
					Flag:      flagName,
					Env:       envName,
					FieldName: rt.Field(i).Name,
					Err:       errors.New("required"),
				})
				continue
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
			if !strings.HasPrefix(stringValue, "{") {
				return fmt.Errorf("In field %s: struct fields should be set using JSON strings", envName)
			}

			if err := json.Unmarshal([]byte(stringValue), fieldInterface); err != nil {
				return err
			}
			continue
		}

		if err := SetFromString(fieldInterface, stringValue); err != nil {
			return fmt.Errorf("In field %s: %s", envName, err)
		}
	}

	for k := range flagMap {
		errs = append(errs, ParamError{
			Err:  errors.New("unknown flag"),
			Flag: k,
		})
	}

	if len(errs) > 0 {
		return errs
	}
	return nil

}

type FlagError string

func (fe FlagError) Error() string {
	return string(fe)
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
