package cliconf

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
)

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
	out += fmt.Sprintf("%d CLI errors:\n", len(pe))
	for _, err := range pe {
		out += fmt.Sprintf("Error parsing %s: %s\n", err.FieldName, err.Err)
	}
	return out
}

func ParseCombined(rvRaw reflect.Value, args []string) error {
	rv, err := toStructVal(rvRaw)
	if err != nil {
		return err
	}

	fields, err := findStructFields(rv)
	if err != nil {
		return err
	}

	argMap := map[int]*field{}
	var remaining *field
	booleans := map[string]struct{}{}
	flagEnvFields := make([]*field, 0, len(fields))

	for _, field := range fields {
		if field.isBool {
			booleans[field.flagName] = struct{}{}
		}

		if field.argn != nil {
			argMap[*field.argn] = field
		} else if field.remaining {
			if remaining != nil {
				return fmt.Errorf("only one field can be tagged with ,remaining")
			}
			remaining = field
		} else if field.flagName != "" || field.envName != "" {
			flagEnvFields = append(flagEnvFields, field)
		} else {
			return fmt.Errorf("field %s has no flag, env, argn, or remaining tag", field.fieldName)
		}
	}

	flagMap, remainingArgs, err := parseFlags(args, booleans)
	if err != nil {
		return err
	}

	dd := &cmdData{
		flagMap: flagMap,
	}

	flagErr := make(ParamErrors, 0)
	thenRemainingArgs := make([]string, 0, len(remainingArgs))
	for idx, arg := range remainingArgs {
		argField, ok := argMap[idx]
		if ok {
			err = setFieldValue(argField, arg)
			if err != nil {
				flagErr = append(flagErr, ParamError{
					Flag:      argField.flagName,
					Env:       argField.envName,
					FieldName: argField.fieldName,
					Err:       err,
				})
			}
		} else {
			thenRemainingArgs = append(thenRemainingArgs, arg)
		}
	}

	if len(thenRemainingArgs) > 0 {
		if remaining != nil {
			remaining.fieldVal.Set(reflect.ValueOf(remainingArgs))
		} else if len(remainingArgs) > 0 {
			flagErr = append(flagErr, ParamError{
				FieldName: "remaining",
				Err:       errors.New("too many remaining args"),
			})
		}
	}

	for _, field := range flagEnvFields {

		stringPtr, err := dd.popValue(field)
		if err != nil {
			return err
		}

		if stringPtr == nil {
			// if required, popValue will already throw
			continue
		}

		stringValue := *stringPtr
		err = setFieldValue(field, stringValue)
		if err != nil {
			flagErr = append(flagErr, ParamError{
				Flag:      field.flagName,
				Env:       field.envName,
				FieldName: field.fieldName,
				Err:       err,
			})
		}
	}

	for k := range dd.flagMap {
		flagErr = append(flagErr, ParamError{
			Err:  errors.New("unknown flag"),
			Flag: k,
		})
	}
	if len(flagErr) > 0 {
		return flagErr
	}
	return nil
}

type cmdData struct {
	flagMap map[string]string
}

func (cd *cmdData) popValue(tag *field) (*string, error) {
	if tag.flagName != "" {
		val, ok := cd.flagMap[tag.flagName]
		if ok {
			delete(cd.flagMap, tag.flagName)
			return &val, nil
		}
	}

	if tag.envName != "" {
		val := os.Getenv(tag.envName)
		if val != "" {
			return &val, nil
		}
	}

	if tag.isBool {
		// leave it false
		return nil, nil
	}

	if tag.defaultVal != nil {
		// if default is empty, that still works, e.g. empty string
		return tag.defaultVal, nil
	}

	if tag.optional {
		return nil, nil
	}

	return nil, errors.New("required")
}

func setFieldValue(field *field, stringValue string) error {

	fieldVal := field.fieldVal

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
			return fmt.Errorf("struct fields should be set using JSON strings")
		}

		if err := json.Unmarshal([]byte(stringValue), fieldInterface); err != nil {
			return err
		}

		return nil
	}

	if err := SetFromString(fieldInterface, stringValue); err != nil {
		return err
	}

	return nil
}

type FlagError string

func (fe FlagError) Error() string {
	return string(fe)
}
