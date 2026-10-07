package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
)

type environment map[string]string

// readDotEnv supports KEY=value, optional export, quotes, and comments.
// Values are literal: shell commands and variable expansion are never evaluated.
func readDotEnv(path string) (environment, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return environment{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("opening .env: %w", err)
	}
	defer file.Close()
	env := environment{}
	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") || strings.HasPrefix(line, "export\t") {
			line = strings.TrimSpace(line[len("export"):])
		}
		key, value, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || !validEnvKey(key) {
			return nil, fmt.Errorf(".env line %d: expected KEY=value with a valid variable name", lineNumber)
		}
		value = strings.TrimSpace(value)
		if len(value) > 0 && (value[0] == '\'' || value[0] == '"') {
			quote := value[0]
			end := strings.IndexByte(value[1:], quote)
			if end < 0 {
				return nil, fmt.Errorf(".env line %d: unterminated quoted value", lineNumber)
			}
			end++
			tail := strings.TrimSpace(value[end+1:])
			if tail != "" && !strings.HasPrefix(tail, "#") {
				return nil, fmt.Errorf(".env line %d: unexpected text after quoted value", lineNumber)
			}
			value = value[1:end]
		} else {
			for i := 0; i < len(value); i++ {
				if value[i] == '#' && (i == 0 || value[i-1] == ' ' || value[i-1] == '\t') {
					value = strings.TrimSpace(value[:i])
					break
				}
			}
		}
		env[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading .env: %w", err)
	}
	return env, nil
}

func validEnvKey(key string) bool {
	if key == "" {
		return false
	}
	for i, c := range key {
		if c == '_' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || i > 0 && c >= '0' && c <= '9' {
			continue
		}
		return false
	}
	return true
}
