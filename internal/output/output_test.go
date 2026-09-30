package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestValidateFormat(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		wantErr bool
	}{
		{name: "an unset format is the caller's default", format: ""},
		{name: "table is supported", format: FormatTable},
		{name: "json is supported", format: FormatJSON},
		{name: "yaml is supported", format: FormatYAML},
		{name: "surrounding whitespace is tolerated", format: "  json  "},
		{name: "an unknown format is rejected", format: "toml", wantErr: true},
		{name: "a format is case sensitive", format: "JSON", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateFormat(test.format)

			if test.wantErr {
				if err == nil {
					t.Fatalf("ValidateFormat(%q) error = nil, want an error", test.format)
				}
				if !strings.Contains(err.Error(), test.format) {
					t.Errorf("ValidateFormat(%q) error = %v, want it to quote the format", test.format, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateFormat(%q) error = %v", test.format, err)
			}
		})
	}
}

func TestWrite(t *testing.T) {
	value := map[string]any{"name": "acme"}
	table := Table{Columns: []string{"NAME"}, Rows: [][]string{{"acme"}}}

	tests := []struct {
		name   string
		format string
		table  Table
		want   string
	}{
		{name: "table renders the columns", format: FormatTable, table: table, want: "NAME"},
		{name: "an unset format renders as table", format: "", table: table, want: "NAME"},
		{name: "json renders the value", format: FormatJSON, table: table, want: `"name": "acme"`},
		{name: "yaml renders the value", format: FormatYAML, table: table, want: "name: acme"},
		{
			// A mutation has already happened by the time it is rendered, so a result with
			// no tabular form must still print rather than fail the command after the fact.
			name:   "table falls back to yaml when there are no columns",
			format: FormatTable,
			table:  Table{},
			want:   "name: acme",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer
			if err := Write(&out, test.format, value, test.table); err != nil {
				t.Fatalf("Write() error = %v", err)
			}

			if !strings.Contains(out.String(), test.want) {
				t.Fatalf("Write() output = %q, want it to contain %q", out.String(), test.want)
			}
		})
	}
}

func TestWriteRejectsAnUnsupportedFormat(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	err := Write(&out, "toml", map[string]any{}, Table{})

	if err == nil || !strings.Contains(err.Error(), "toml") {
		t.Fatalf("Write() error = %v, want it to name the unsupported format", err)
	}
}

func TestWriteJSONAndYAMLRoundTrip(t *testing.T) {
	t.Parallel()

	value := map[string]any{"name": "acme", "count": 2}

	var asJSON bytes.Buffer
	if err := Write(&asJSON, FormatJSON, value, Table{}); err != nil {
		t.Fatalf("Write(json) error = %v", err)
	}
	var fromJSON map[string]any
	if err := json.Unmarshal(asJSON.Bytes(), &fromJSON); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; output = %q", err, asJSON.String())
	}
	if fromJSON["name"] != "acme" {
		t.Errorf("json name = %v, want acme", fromJSON["name"])
	}

	var asYAML bytes.Buffer
	if err := Write(&asYAML, FormatYAML, value, Table{}); err != nil {
		t.Fatalf("Write(yaml) error = %v", err)
	}
	var fromYAML map[string]any
	if err := yaml.Unmarshal(asYAML.Bytes(), &fromYAML); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v; output = %q", err, asYAML.String())
	}
	if fromYAML["name"] != "acme" {
		t.Errorf("yaml name = %v, want acme", fromYAML["name"])
	}
}

func TestWriteTableAlignsColumns(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	err := WriteTable(&out, Table{
		Columns: []string{"NAME", "SLUG"},
		Rows:    [][]string{{"a-very-long-name", "short"}, {"tiny", "another-slug"}},
	})
	if err != nil {
		t.Fatalf("WriteTable() error = %v", err)
	}

	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("WriteTable() wrote %d lines, want a header and two rows: %q", len(lines), out.String())
	}
	slugColumn := strings.Index(lines[0], "SLUG")
	for _, line := range lines[1:] {
		if strings.Index(line, strings.Fields(line)[1]) != slugColumn {
			t.Errorf("row %q does not align its second column with the header at %d", line, slugColumn)
		}
	}
}

func TestWriteTableRequiresColumns(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	err := WriteTable(&out, Table{Rows: [][]string{{"acme"}}})

	if err == nil {
		t.Fatal("WriteTable() error = nil, want an error for a table with no columns")
	}
}

func TestWriteTableRendersAnEmptyResultAsHeadersOnly(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	if err := WriteTable(&out, Table{Columns: []string{"NAME"}}); err != nil {
		t.Fatalf("WriteTable() error = %v", err)
	}

	if got := strings.TrimSpace(out.String()); got != "NAME" {
		t.Fatalf("WriteTable() output = %q, want the header alone", got)
	}
}
