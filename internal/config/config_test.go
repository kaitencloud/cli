package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultPathIsUnderTheUserConfigDir(t *testing.T) {
	givenIsolatedConfigHome(t)

	path, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() error = %v", err)
	}

	if filepath.Base(path) != "config.yaml" || filepath.Base(filepath.Dir(path)) != "kaiten" {
		t.Fatalf("DefaultPath() = %q, want it to end in kaiten/config.yaml", path)
	}
}

func TestLoadReturnsAnEmptyFileWhenNoneExists(t *testing.T) {
	givenIsolatedConfigHome(t)

	cfg, path, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg != (File{}) {
		t.Errorf("Load() = %+v, want the zero File", cfg)
	}
	if path == "" {
		t.Error("Load() path = \"\", want the path the CLI would write to")
	}
}

func TestLoadTrimsSurroundingWhitespace(t *testing.T) {
	givenPersistedFile(t, "base_url: \"  https://api.example  \"\nauth_token: \"  token  \"\noutput: \"  json  \"\n")

	cfg, _, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := File{BaseURL: "https://api.example", AuthToken: "token", Output: "json"}
	if cfg != want {
		t.Fatalf("Load() = %+v, want %+v", cfg, want)
	}
}

func TestLoadReportsAMalformedConfigFile(t *testing.T) {
	givenPersistedFile(t, "base_url: [unclosed\n")

	_, _, err := Load()

	if err == nil || !strings.Contains(err.Error(), "parse config file") {
		t.Fatalf("Load() error = %v, want a parse failure", err)
	}
}

func TestSaveRoundTripsThroughLoad(t *testing.T) {
	givenIsolatedConfigHome(t)
	want := File{BaseURL: "https://api.example", AuthToken: "token", Output: "json"}

	path, err := Save(want)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, loadedPath, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got != want {
		t.Errorf("Load() = %+v, want %+v", got, want)
	}
	if loadedPath != path {
		t.Errorf("Load() path = %q, want %q", loadedPath, path)
	}
}

// TestSaveKeepsTheConfigFilePrivate pins the file mode: the config file holds an API token,
// so a mode any other account on the machine can read would leak it.
func TestSaveKeepsTheConfigFilePrivate(t *testing.T) {
	givenIsolatedConfigHome(t)

	path, err := Save(File{AuthToken: "token"})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("config file mode = %04o, want 0600", got)
	}
}

func TestResolvePrecedence(t *testing.T) {
	const (
		fileConfig = "base_url: https://file.example\nauth_token: file-token\noutput: yaml\n"
		flagURL    = "https://flag.example"
		envURL     = "https://env.example"
	)

	tests := []struct {
		name          string
		file          string
		env           bool
		flagBaseURL   string
		flagAuthToken string
		flagOutput    string
		wantBaseURL   string
		wantAuthToken string
		wantOutput    string
	}{
		{
			name:          "a flag beats the environment and the file",
			file:          fileConfig,
			env:           true,
			flagBaseURL:   flagURL,
			flagAuthToken: "flag-token",
			flagOutput:    "table",
			wantBaseURL:   flagURL,
			wantAuthToken: "flag-token",
			wantOutput:    "table",
		},
		{
			name:          "the environment beats the file when no flag is given",
			file:          fileConfig,
			env:           true,
			wantBaseURL:   envURL,
			wantAuthToken: "env-token",
			wantOutput:    "json",
		},
		{
			name:          "the file is used when nothing else supplies a value",
			file:          fileConfig,
			wantBaseURL:   "https://file.example",
			wantAuthToken: "file-token",
			wantOutput:    "yaml",
		},
		{
			name: "nothing configured resolves to empty values",
		},
		{
			// A flag that is only whitespace is not a value, and must not shadow the file.
			name:        "a whitespace-only flag falls through to the file",
			file:        fileConfig,
			flagBaseURL: "   ",
			wantBaseURL: "https://file.example", wantAuthToken: "file-token", wantOutput: "yaml",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.file != "" {
				givenPersistedFile(t, test.file)
			} else {
				givenIsolatedConfigHome(t)
			}
			if test.env {
				t.Setenv(EnvBaseURL, envURL)
				t.Setenv(EnvAuthToken, "env-token")
				t.Setenv(EnvOutput, "json")
			}

			cfg, err := Resolve(test.flagBaseURL, test.flagAuthToken, test.flagOutput)
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}

			expectField(t, "BaseURL", cfg.BaseURL, test.wantBaseURL)
			expectField(t, "AuthToken", cfg.AuthToken, test.wantAuthToken)
			expectField(t, "Output", cfg.Output, test.wantOutput)
			if cfg.ConfigFilePath == "" {
				t.Error("ConfigFilePath = \"\", want the resolved config file path")
			}
		})
	}
}

func TestMaskToken(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  string
	}{
		{name: "an empty token masks to nothing", token: "", want: ""},
		{name: "a token shorter than the window is fully masked", token: "abcde", want: "*****"},
		{name: "an eight character token is fully masked", token: "abcdefgh", want: "********"},
		{name: "a longer token keeps its first and last four", token: "abcdefgh12345678", want: "abcd********5678"},
		{name: "masking counts runes, not bytes", token: "abcdéfg雪ijkl", want: "abcd****ijkl"},
		{name: "surrounding whitespace is trimmed before masking", token: "  abcdefgh12345678  ", want: "abcd********5678"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := MaskToken(test.token); got != test.want {
				t.Fatalf("MaskToken(%q) = %q, want %q", test.token, got, test.want)
			}
		})
	}
}

func expectField(t *testing.T, name, got, want string) {
	t.Helper()

	if got != want {
		t.Errorf("%s = %q, want %q", name, got, want)
	}
}

// givenIsolatedConfigHome points os.UserConfigDir at an empty temp directory. Both variables
// are set because it consults XDG_CONFIG_HOME on Linux and HOME on macOS.
func givenIsolatedConfigHome(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	t.Setenv(EnvBaseURL, "")
	t.Setenv(EnvAuthToken, "")
	t.Setenv(EnvOutput, "")
}

// givenPersistedFile isolates the config directory and seeds the config file with content.
func givenPersistedFile(t *testing.T, content string) string {
	t.Helper()
	givenIsolatedConfigHome(t)

	path, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	return path
}
