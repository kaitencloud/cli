package input

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
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

// instanceInput stands for an input type of the API client. Its fields carry no tags: a key
// reaches a field the way JSON matches one, by name and whatever the case.
type instanceInput struct {
	Name             string
	Version          string
	CustomerID       string
	DeploymentZoneID *string
	IsDefault        bool
	SaleUnitFactor   *float64
	StartLicenseDate time.Time
	GroupSlugs       []string
	Metadata         map[string]any
}

// flagInput stands for a type the API client generates: its keys are spelled by json tags.
type flagInput struct {
	EventName      string     `json:"event_name"`
	DefaultVariant typedValue `json:"default_variant"`
}

// typedValue stands for a value discriminated by its type field. It reads itself from JSON
// and from nothing else.
type typedValue struct {
	union json.RawMessage
}

func (v *typedValue) UnmarshalJSON(data []byte) error {
	v.union = append(v.union[:0], data...)
	return nil
}

// TestYAMLDecodesToWhatTheEquivalentJSONDecodesTo pins a YAML document to the request the
// same document written in JSON makes. The types of the API name their fields for JSON
// only, so a YAML decoder reading them matched the keys of one word and dropped every other
// without a word -- externalCustomerId, default_variant -- and read a typed value as nothing.
func TestYAMLDecodesToWhatTheEquivalentJSONDecodesTo(t *testing.T) {
	t.Parallel()

	const instanceYAML = `
name: Sakura Tokyo Prod
customerId: cus_1
deploymentZoneId: zone_1
isDefault: true
saleUnitFactor: 10
startLicenseDate: 2026-01-01T00:00:00Z
groupSlugs: [restaurant-operations, delivery]
metadata:
  region: kanto
  seats: 40
  ratio: 0.5
  pilot: false
  owner: null
  opened: 2026-01-01
  tags: [a, b]
`
	const instanceJSON = `{
  "name": "Sakura Tokyo Prod", "customerId": "cus_1", "deploymentZoneId": "zone_1", "isDefault": true,
  "saleUnitFactor": 10, "startLicenseDate": "2026-01-01T00:00:00Z",
  "groupSlugs": ["restaurant-operations", "delivery"],
  "metadata": {"region": "kanto", "seats": 40, "ratio": 0.5, "pilot": false, "owner": null,
    "opened": "2026-01-01", "tags": ["a", "b"]}
}`
	expectSameDecoding[instanceInput](t, "an input type, its keys of several words included", instanceYAML, instanceJSON)

	expectSameDecoding[flagInput](t, "a generated type and the typed value it holds",
		"event_name: delivery-tracking\ndefault_variant:\n  type: boolean\n  value: false\n",
		`{"event_name": "delivery-tracking", "default_variant": {"type": "boolean", "value": false}}`)

	expectSameDecoding[typedValue](t, "a typed value on its own",
		"type: number\nvalue: 2000\n", `{"type": "number", "value": 2000}`)

	expectSameDecoding[map[string]any](t, "a free-form object",
		"max: 10\n1024: kept\nnested:\n  enabled: true\n", `{"max": 10, "1024": "kept", "nested": {"enabled": true}}`)
}

// TestYAMLKeepsTextAsWritten covers the two places where YAML and JSON part ways on what a
// value is. A text field takes what was written: version 1.0 is the text 1.0, not the
// number 1, which JSON would refuse there. And a date stays the text it is in JSON instead
// of becoming a time and coming back respelled.
func TestYAMLKeepsTextAsWritten(t *testing.T) {
	t.Parallel()

	got, err := LoadString[instanceInput]("name: 2026\nversion: 1.0\ncustomerId: 2026-08-01\nmetadata:\n  since: 2026-08-01\n")
	if err != nil {
		t.Fatalf("LoadString() error = %v", err)
	}

	want := instanceInput{
		Name:       "2026",
		Version:    "1.0",
		CustomerID: "2026-08-01",
		Metadata:   map[string]any{"since": "2026-08-01"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadString() = %+v, want %+v", got, want)
	}
}

// TestYAMLRejectsWhatItCannotPlace keeps a YAML document from going out incomplete: a key
// the request has no field for is an error that names it, and so is a value of the wrong
// kind. Neither is dropped.
func TestYAMLRejectsWhatItCannotPlace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name:    "a key the request has no field for",
			content: "name: api\nexternal_customer_id: crm-1024\n",
			wantErr: `decode YAML input: unknown field "external_customer_id"`,
		},
		{
			name:    "a value of the wrong kind",
			content: "name: api\nisDefault: sometimes\n",
			wantErr: "decode YAML input: cannot unmarshal string",
		},
		{
			name:    "a key that is a list",
			content: "name: api\nmetadata:\n  [a, b]: c\n",
			wantErr: "decode YAML input: yaml: invalid map key",
		},
		{
			name:    "a key tagged as a number",
			content: "name: api\nmetadata:\n  !!int 5: c\n",
			wantErr: "decode YAML input: a key must be text",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// By extension and by content alike: the reason is not lost to the sniffing.
			_, fileErr := LoadFile[instanceInput](givenFile(t, "instance.yaml", test.content))
			_, stringErr := LoadString[instanceInput](test.content)

			for source, err := range map[string]error{"LoadFile": fileErr, "LoadString": stringErr} {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Errorf("%s() error = %v, want it to contain %q", source, err, test.wantErr)
				}
			}
		})
	}
}

// TestJSONIgnoresAKeyTheRequestHasNoFieldFor records that JSON input is left as it was: the
// keys a request has no field for are skipped, which is what lets a resource printed by
// --output json be edited and written back with the fields only the API sets still in it.
func TestJSONIgnoresAKeyTheRequestHasNoFieldFor(t *testing.T) {
	t.Parallel()

	got, err := LoadString[instanceInput](`{"id": "ins_1", "name": "Sakura Tokyo Prod", "createdAt": "2026-10-01T09:00:00Z"}`)
	if err != nil {
		t.Fatalf("LoadString() error = %v", err)
	}
	if want := (instanceInput{Name: "Sakura Tokyo Prod"}); !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadString() = %+v, want %+v", got, want)
	}
}

// TestAnEmptyYAMLDocumentDecodesToNothing keeps an empty file what it was: no field set,
// and no error of its own -- the command reports what the request is missing.
func TestAnEmptyYAMLDocumentDecodesToNothing(t *testing.T) {
	t.Parallel()

	got, err := LoadFile[instanceInput](givenFile(t, "instance.yaml", "# nothing yet\n"))
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}
	if !reflect.DeepEqual(got, instanceInput{}) {
		t.Fatalf("LoadFile() = %+v, want the zero value", got)
	}
}

// expectSameDecoding decodes one document written both ways and fails unless the two
// requests are the same, and the YAML one is not empty.
func expectSameDecoding[T any](t *testing.T, name, yamlDocument, jsonDocument string) {
	t.Helper()

	fromJSON, err := LoadFile[T](givenFile(t, "document.json", jsonDocument))
	if err != nil {
		t.Fatalf("%s: LoadFile(json) error = %v", name, err)
	}
	fromYAML, err := LoadFile[T](givenFile(t, "document.yaml", yamlDocument))
	if err != nil {
		t.Fatalf("%s: LoadFile(yaml) error = %v", name, err)
	}
	sniffed, err := LoadString[T](yamlDocument)
	if err != nil {
		t.Fatalf("%s: LoadString(yaml) error = %v", name, err)
	}

	var zero T
	if reflect.DeepEqual(fromJSON, zero) {
		t.Fatalf("%s: the json document decoded to nothing", name)
	}
	want := normalized(t, fromJSON)
	if !reflect.DeepEqual(normalized(t, fromYAML), want) || !reflect.DeepEqual(normalized(t, sniffed), want) {
		t.Fatalf("%s: yaml decoded to %+v (by extension) and %+v (by content), want what json decodes to, %+v",
			name, fromYAML, sniffed, fromJSON)
	}
}

// normalized returns value with the raw JSON a typedValue holds decoded, so that two
// spellings of one document compare equal.
func normalized(t *testing.T, value any) any {
	t.Helper()

	switch typed := value.(type) {
	case typedValue:
		return decodedJSON(t, typed.union)
	case flagInput:
		return [2]any{typed.EventName, decodedJSON(t, typed.DefaultVariant.union)}
	default:
		return value
	}
}

func decodedJSON(t *testing.T, raw json.RawMessage) any {
	t.Helper()

	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("json.Unmarshal(%q) error = %v", raw, err)
	}
	return decoded
}

func givenFile(t *testing.T, name, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	return path
}
