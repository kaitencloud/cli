package input

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type component struct {
	Name    string `json:"name" yaml:"name"`
	Version string `json:"version" yaml:"version"`
}

func TestLoadFile(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
		content  string
		want     component
	}{
		{
			name:     "a .json extension selects the json decoder",
			fileName: "component.json",
			content:  `{"name":"api","version":"1.2.3"}`,
			want:     component{Name: "api", Version: "1.2.3"},
		},
		{
			name:     "a .yaml extension selects the yaml decoder",
			fileName: "component.yaml",
			content:  "name: api\nversion: 1.2.3\n",
			want:     component{Name: "api", Version: "1.2.3"},
		},
		{
			name:     "a .yml extension selects the yaml decoder",
			fileName: "component.yml",
			content:  "name: api\nversion: 1.2.3\n",
			want:     component{Name: "api", Version: "1.2.3"},
		},
		{
			// An operator naming a file component.txt still means the document inside it.
			name:     "an unknown extension falls back to sniffing the content",
			fileName: "component.txt",
			content:  "name: api\nversion: 1.2.3\n",
			want:     component{Name: "api", Version: "1.2.3"},
		},
		{
			name:     "a file with no extension falls back to sniffing the content",
			fileName: "component",
			content:  `{"name":"api","version":"1.2.3"}`,
			want:     component{Name: "api", Version: "1.2.3"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			path := givenFile(t, test.fileName, test.content)

			got, err := LoadFile[component](path)
			if err != nil {
				t.Fatalf("LoadFile() error = %v", err)
			}

			if got != test.want {
				t.Fatalf("LoadFile() = %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestLoadFileErrors(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		content  string
		wantErr  string
		useFile  bool
		fileName string
	}{
		{name: "an empty path is rejected before any read", path: "", wantErr: "input file is required"},
		{name: "a whitespace-only path is rejected too", path: "   ", wantErr: "input file is required"},
		{name: "a missing file reports the read failure", path: "definitely-absent.yaml", wantErr: "read input file"},
		{
			name:     "a malformed json document names the format",
			useFile:  true,
			fileName: "component.json",
			content:  "{not json",
			wantErr:  "decode JSON input",
		},
		{
			name:     "a malformed yaml document names the format",
			useFile:  true,
			fileName: "component.yaml",
			content:  "name: [unclosed",
			wantErr:  "decode YAML input",
		},
		{
			name:     "a document that is neither json nor yaml is reported as such",
			useFile:  true,
			fileName: "component.txt",
			content:  "\tname:\t[unclosed",
			wantErr:  "expected JSON or YAML",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			path := test.path
			if test.useFile {
				path = givenFile(t, test.fileName, test.content)
			} else if path == "definitely-absent.yaml" {
				path = filepath.Join(t.TempDir(), path)
			}

			_, err := LoadFile[component](path)

			if err == nil {
				t.Fatalf("LoadFile(%q) error = nil, want %q", path, test.wantErr)
			}
			if !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("LoadFile(%q) error = %v, want it to contain %q", path, err, test.wantErr)
			}
		})
	}
}

func TestLoadString(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    component
		wantErr string
	}{
		{
			name:    "an inline json payload is decoded",
			payload: `{"name":"api","version":"1.2.3"}`,
			want:    component{Name: "api", Version: "1.2.3"},
		},
		{
			name:    "an inline yaml payload is decoded",
			payload: "name: api\nversion: 1.2.3",
			want:    component{Name: "api", Version: "1.2.3"},
		},
		{name: "an empty payload is rejected", payload: "", wantErr: "inline payload is required"},
		{name: "a whitespace-only payload is rejected", payload: "  \n ", wantErr: "inline payload is required"},
		{name: "an undecodable payload is reported", payload: "\tname:\t[unclosed", wantErr: "expected JSON or YAML"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := LoadString[component](test.payload)

			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("LoadString() error = %v, want it to contain %q", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadString() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("LoadString() = %+v, want %+v", got, test.want)
			}
		})
	}
}

func givenFile(t *testing.T, name, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	return path
}
