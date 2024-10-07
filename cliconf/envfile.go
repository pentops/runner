package cliconf

import (
	"os"
	"strings"
)

func ReadEnvFile(filename string) (map[string]string, error) {
	if filename == "" {
		return nil, nil
	}

	fileData, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(fileData), "\n")
	envMap := make(map[string]string)
	for _, line := range lines {
		if strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		envMap[key] = value
	}

	return envMap, nil
}

func LoadEnvFile(filename string) error {
	env, err := ReadEnvFile(filename)
	if err != nil {
		return err
	}
	for key, value := range env {
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}
	return nil
}
