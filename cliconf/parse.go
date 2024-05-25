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

	rt := rv.Type()

	booleans := findBooleanFlags(rt)
	flagMap, remainingArgs, err := parseFlags(args, booleans)
	if err != nil {
		return err
	}
	dd := &cmdData{
		flagMap:   flagMap,
		remaining: remainingArgs,
	}
	flagErr := dd.runStruct(rv)
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
	flagMap       map[string]string
	remaining     []string
	usedRemaining bool
}

func (cd *cmdData) popValue(tag parsedTag) (*string, error) {
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

func (cd *cmdData) runField(tag parsedTag, fieldVal reflect.Value) error {

	if tag.remaining {
		if cd.usedRemaining {
			return fmt.Errorf("only one field can be tagged with ,remaining")
		}
		if fieldVal.Kind() != reflect.Slice {
			return fmt.Errorf("remaining args must be a slice")
		}
		if fieldVal.Type().Elem().Kind() != reflect.String {
			return fmt.Errorf("remaining args must be a slice of strings")
		}
		fieldVal.Set(reflect.ValueOf(cd.remaining))
		cd.usedRemaining = true
		return nil
	}

	stringPtr, err := cd.popValue(tag)
	if err != nil {
		return err
	}

	if stringPtr == nil {
		// if required, popValue will already throw
		return nil
	}

	stringValue := *stringPtr

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

func (cd *cmdData) runStruct(rv reflect.Value) ParamErrors {
	errs := make(ParamErrors, 0)
	rt := rv.Type()

	for i := 0; i < rv.NumField(); i++ {
		fieldType := rt.Field(i)
		parsed := parseField(fieldType)
		if parsed == nil {
			if fieldType.Type.Kind() != reflect.Struct {
				continue
			}
			subStruct, err := toStructVal(rv.Field(i))
			if err != nil {
				// INVERSION
				continue
			}

			subErrs := cd.runStruct(subStruct)
			if len(subErrs) > 0 {
				errs = append(errs, subErrs...)
			}

			continue
		}
		err := cd.runField(*parsed, rv.Field(i))
		if err != nil {
			errs = append(errs, ParamError{
				Flag:      parsed.flagName,
				Env:       parsed.envName,
				FieldName: fieldType.Name,
				Err:       err,
			})
		}

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
