// Package config manages the kaiten CLI's persisted configuration file and resolved runtime settings.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Environment variables that can override configuration file values.
const (
	EnvBaseURL   = "KAITEN_BASE_URL"
	EnvAuthToken = "KAITEN_AUTH_TOKEN"
	EnvOutput    = "KAITEN_OUTPUT"
)

// File is the persisted CLI configuration stored on disk.
type File struct {
	BaseURL   string `yaml:"base_url,omitempty" json:"base_url,omitempty"`
	AuthToken string `yaml:"auth_token,omitempty" json:"auth_token,omitempty"`
	Output    string `yaml:"output,omitempty" json:"output,omitempty"`
}

// Runtime is the effective configuration for a CLI invocation, resolved from flags, environment variables, and the config file.
type Runtime struct {
	BaseURL        string `json:"base_url" yaml:"base_url"`
	AuthToken      string `json:"auth_token,omitempty" yaml:"auth_token,omitempty"`
	Output         string `json:"output,omitempty" yaml:"output,omitempty"`
	ConfigFilePath string `json:"config_file_path" yaml:"config_file_path"`
}

// DefaultPath returns the path to the CLI configuration file in the user's config directory.
func DefaultPath() (string, error) {
	configHome, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}

	return filepath.Join(configHome, "kaiten", "config.yaml"), nil
}

// Load reads and parses the CLI configuration file, returning an empty File if it does not exist.
func Load() (File, string, error) {
	path, err := DefaultPath()
	if err != nil {
		return File{}, "", err
	}

	data, err := os.ReadFile(path) //nolint:gosec // path is derived from os.UserConfigDir, not user input
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return File{}, path, nil
		}
		return File{}, path, fmt.Errorf("read config file: %w", err)
	}

	var cfg File
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return File{}, path, fmt.Errorf("parse config file: %w", err)
	}

	return sanitizeFile(cfg), path, nil
}

// Save writes the CLI configuration file, creating its parent directory if needed.
func Save(cfg File) (string, error) {
	path, err := DefaultPath()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return "", fmt.Errorf("create config directory: %w", err)
	}

	data, err := yaml.Marshal(sanitizeFile(cfg)) //nolint:gosec // auth_token is the user's own CLI credential, intentionally persisted
	if err != nil {
		return "", fmt.Errorf("marshal config file: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("write config file: %w", err)
	}

	return path, nil
}

// Resolve merges flag, environment variable, and config file values into the effective Runtime configuration.
func Resolve(flagBaseURL, flagAuthToken, flagOutput string) (Runtime, error) {
	fileCfg, path, err := Load()
	if err != nil {
		return Runtime{}, err
	}

	baseURL := firstNonEmpty(
		strings.TrimSpace(flagBaseURL),
		strings.TrimSpace(os.Getenv(EnvBaseURL)),
		fileCfg.BaseURL,
	)
	authToken := firstNonEmpty(
		strings.TrimSpace(flagAuthToken),
		strings.TrimSpace(os.Getenv(EnvAuthToken)),
		fileCfg.AuthToken,
	)
	output := firstNonEmpty(
		strings.TrimSpace(flagOutput),
		strings.TrimSpace(os.Getenv(EnvOutput)),
		fileCfg.Output,
	)

	return Runtime{
		BaseURL:        baseURL,
		AuthToken:      authToken,
		Output:         output,
		ConfigFilePath: path,
	}, nil
}

// MaskToken returns a redacted form of token suitable for display, keeping only its first and last four characters.
func MaskToken(token string) string {
	runes := []rune(strings.TrimSpace(token))
	switch len(runes) {
	case 0:
		return ""
	case 1, 2, 3, 4, 5, 6, 7, 8:
		return strings.Repeat("*", len(runes))
	default:
		return string(runes[:4]) + strings.Repeat("*", len(runes)-8) + string(runes[len(runes)-4:])
	}
}

func sanitizeFile(cfg File) File {
	cfg.BaseURL = strings.TrimSpace(cfg.BaseURL)
	cfg.AuthToken = strings.TrimSpace(cfg.AuthToken)
	cfg.Output = strings.TrimSpace(cfg.Output)
	return cfg
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
