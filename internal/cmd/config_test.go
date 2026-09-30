package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaitencloud/cli/internal/config"
)

// TestConfigCommandsRecoverFromInvalidPersistedOutput pins the recovery path for a config
// file the CLI can no longer act on: an unsupported output format fails every other command
// at flag validation, so config view and config unset are the only way back out. Both carry
// the skip-output-validation annotation for exactly this reason.
func TestConfigCommandsRecoverFromInvalidPersistedOutput(t *testing.T) {
	path := givenPersistedConfig(t, "output: invalid\n")

	viewOutput := runSuccessfully(t, "config", "view")
	expectOutputContains(t, viewOutput, "output: invalid")

	unsetOutput := runSuccessfully(t, "config", "unset", "--yes", "output")
	expectOutputContains(t, unsetOutput, "Removed output from")

	remaining, err := os.ReadFile(path) //nolint:gosec // path comes from the test's own temp dir
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(remaining), "invalid") {
		t.Fatalf("config file = %q, want the invalid output value removed", remaining)
	}
}

func TestConfigSetPersistsAValueTheNextRunResolves(t *testing.T) {
	givenPersistedConfig(t, "")

	runSuccessfully(t, "config", "set", "base-url", "https://api.example")

	viewOutput := runSuccessfully(t, "config", "view")
	expectOutputContains(t, viewOutput, "https://api.example")
}

func TestConfigViewMasksThePersistedToken(t *testing.T) {
	givenPersistedConfig(t, "auth_token: super-secret-token-value\n")

	viewOutput := runSuccessfully(t, "config", "view")

	if strings.Contains(viewOutput, "super-secret-token-value") {
		t.Fatalf("config view output = %q, want the token masked", viewOutput)
	}
	expectOutputContains(t, viewOutput, "supe")
}

func TestConfigSetRejectsAnUnsupportedOutputFormat(t *testing.T) {
	givenPersistedConfig(t, "")

	_, err := executeForExitCode(t, "config", "set", "output", "toml")

	if err == nil || !strings.Contains(err.Error(), "unsupported output format") {
		t.Fatalf("Execute() error = %v, want an unsupported format error", err)
	}
}

// givenPersistedConfig isolates the config directory and seeds the config file with content,
// returning the path the CLI will read and write.
func givenPersistedConfig(t *testing.T, content string) string {
	t.Helper()
	givenIsolatedConfig(t)

	path, err := config.DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if content != "" {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
	}

	return path
}
