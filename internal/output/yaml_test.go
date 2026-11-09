package output

import (
	"bytes"
	"encoding/json"
	"math"
	"reflect"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// apiResource stands for a type of the API client. It describes itself in JSON only: json
// tags, an optional field that is left out when absent, and a typed value that knows how to
// write itself as JSON and nothing else.
type apiResource struct {
	ID                 string     `json:"id"`
	ExternalCustomerID *string    `json:"externalCustomerId,omitempty"`
	Domain             *string    `json:"domain,omitempty"`
	CreatedAt          time.Time  `json:"createdAt"`
	Value              typedValue `json:"value"`
}

type typedValue struct {
	union json.RawMessage
}

func (v typedValue) MarshalJSON() ([]byte, error) {
	return v.union, nil
}

// TestWriteYAMLPrintsTheJSONDocument pins what YAML output is made of: the keys, the values
// and the key order of the JSON encoding. A YAML encoder reading the Go type instead names
// the fields externalcustomerid and createdat, prints the absent domain as null and the
// typed value as an empty mapping.
func TestWriteYAMLPrintsTheJSONDocument(t *testing.T) {
	t.Parallel()

	externalID := "crm-1024"
	resource := apiResource{
		ID:                 "cus_1",
		ExternalCustomerID: &externalID,
		CreatedAt:          time.Date(2026, time.October, 1, 9, 0, 0, 0, time.UTC),
		Value:              typedValue{union: json.RawMessage(`{"type":"number","value":1000}`)},
	}

	var out bytes.Buffer
	if err := Write(&out, FormatYAML, resource, Table{}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	const want = `id: cus_1
externalCustomerId: crm-1024
createdAt: "2026-10-01T09:00:00Z"
value:
    type: number
    value: 1000
`
	if out.String() != want {
		t.Fatalf("Write() output =\n%s\nwant\n%s", out.String(), want)
	}
}

// TestWriteYAMLKeepsEveryValueWhatItIsInJSON checks the values one kind at a time: read
// back, the YAML document is the JSON one. A string that reads as another type stays a
// string, and a number is printed as JSON writes it.
func TestWriteYAMLKeepsEveryValueWhatItIsInJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value any
		want  string
	}{
		{name: "a plain string", value: "acme", want: "acme\n"},
		{name: "a string that reads as a boolean", value: "true", want: "\"true\"\n"},
		{name: "a string that reads as a number", value: "1024", want: "\"1024\"\n"},
		{name: "a string that reads as null", value: "null", want: "\"null\"\n"},
		{name: "an empty string", value: "", want: "\"\"\n"},
		{name: "a string of several lines", value: "first\nsecond", want: "|-\n    first\n    second\n"},
		{name: "a boolean", value: false, want: "false\n"},
		{name: "null", value: nil, want: "null\n"},
		{name: "an integer", value: 1000, want: "1000\n"},
		{name: "a decimal", value: 730.5, want: "730.5\n"},
		{name: "an integer too large for a float", value: uint64(math.MaxUint64), want: "18446744073709551615\n"},
		{name: "an empty object", value: map[string]any{}, want: "{}\n"},
		{name: "an empty list", value: []string{}, want: "[]\n"},
		{name: "an absent list", value: []string(nil), want: "null\n"},
		{
			name:  "a list of objects",
			value: []map[string]any{{"slug": "monthly-orders", "limit": -1}},
			want:  "- limit: -1\n  slug: monthly-orders\n",
		},
		{
			name:  "an object holding a list",
			value: map[string]any{"scopes": []string{"read:instances", "write:instances"}},
			want:  "scopes:\n    - read:instances\n    - write:instances\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer
			if err := Write(&out, FormatYAML, test.value, Table{}); err != nil {
				t.Fatalf("Write() error = %v", err)
			}
			if out.String() != test.want {
				t.Fatalf("Write() output = %q, want %q", out.String(), test.want)
			}

			var fromYAML any
			if err := yaml.Unmarshal(out.Bytes(), &fromYAML); err != nil {
				t.Fatalf("yaml.Unmarshal() error = %v; output = %q", err, out.String())
			}
			if got, want := asJSON(t, fromYAML), asJSON(t, test.value); !reflect.DeepEqual(got, want) {
				t.Fatalf("read back, the yaml document is %v, want %v", got, want)
			}
		})
	}
}

// TestWriteYAMLReportsAValueJSONCannotEncode keeps a value with no JSON form an error
// rather than an empty document.
func TestWriteYAMLReportsAValueJSONCannotEncode(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	if err := Write(&out, FormatYAML, map[string]any{"ratio": math.NaN()}, Table{}); err == nil {
		t.Fatalf("Write() error = nil, want one; output = %q", out.String())
	}
}

// asJSON returns value as JSON types it, so that a document read from YAML compares with
// the value it was written from.
func asJSON(t *testing.T, value any) any {
	t.Helper()

	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	return out
}
