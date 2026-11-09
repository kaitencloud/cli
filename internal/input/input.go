// Package input loads structured command input from files, stdin, or inline strings.
package input

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// LoadFile reads and decodes the JSON or YAML file at path into a value of type T. A path of "-" reads from stdin.
func LoadFile[T any](path string) (T, error) {
	var zero T

	data, err := read(path)
	if err != nil {
		return zero, err
	}

	return decode[T](data, filepath.Ext(path))
}

// LoadString decodes payload, a JSON or YAML string, into a value of type T.
func LoadString[T any](payload string) (T, error) {
	var zero T
	if strings.TrimSpace(payload) == "" {
		return zero, fmt.Errorf("inline payload is required")
	}

	return decode[T]([]byte(payload), "")
}

func read(path string) ([]byte, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("input file is required")
	}

	if path == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("read stdin: %w", err)
		}
		return data, nil
	}

	data, err := os.ReadFile(path) //nolint:gosec // path is a user-supplied CLI argument naming the input file to load
	if err != nil {
		return nil, fmt.Errorf("read input file: %w", err)
	}

	return data, nil
}

func decode[T any](data []byte, formatHint string) (T, error) {
	var zero T

	switch strings.ToLower(formatHint) {
	case ".json":
		var out T
		if err := json.Unmarshal(data, &out); err != nil {
			return zero, fmt.Errorf("decode JSON input: %w", err)
		}
		return out, nil
	case ".yaml", ".yml":
		document, err := parseYAML(data)
		if err != nil {
			return zero, fmt.Errorf("decode YAML input: %w", err)
		}
		out, err := decodeYAML[T](document)
		if err != nil {
			return zero, fmt.Errorf("decode YAML input: %w", err)
		}
		return out, nil
	}

	var out T
	jsonErr := json.Unmarshal(data, &out)
	if jsonErr == nil {
		return out, nil
	}

	// Not JSON, or JSON that does not fit the request. Either is tried as YAML, where a text
	// field takes a bare number, and the second is reported as the JSON it is if that fails.
	jsonValid := json.Valid(data)
	if document, err := parseYAML(data); err == nil {
		out, err := decodeYAML[T](document)
		if err == nil {
			return out, nil
		}
		if !jsonValid {
			return zero, fmt.Errorf("decode YAML input: %w", err)
		}
	}
	if jsonValid {
		return zero, fmt.Errorf("decode JSON input: %w", jsonErr)
	}
	return zero, fmt.Errorf("decode input: expected JSON or YAML")
}
