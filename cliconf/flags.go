package cliconf

import (
	"fmt"
	"strings"
)

const (
	boolTrue  = "true"
	boolFalse = "false"
)

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
			if lower == boolTrue || lower == boolFalse {
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
