package cmd

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const instanceMetadataPath = "/instances/sakura-tokyo-prod"

// TestInstancesMetadataSendsOnlyTheKeysItNames pins `instances metadata set` and `unset` to
// one PATCH carrying a metadata object of the keys they name and nothing else: no read of
// the instance first, no other instance field, so the API keeps every key the command does
// not name. --string sends text, --json any JSON value, and a removed key goes out as null.
func TestInstancesMetadataSendsOnlyTheKeysItNames(t *testing.T) {
	metadataFile := filepath.Join(t.TempDir(), "metadata.yaml")
	if err := os.WriteFile(metadataFile, []byte("region: kansai\nlegacy-plan: null\nseats: 40\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		args []string
		want map[string]any
	}{
		{
			name: "--string sets text, a number-looking one included",
			args: []string{"set", "sakura-tokyo-prod", "--string", "region=kanto", "--string", "zip=150-0002", "--string", "code=007"},
			want: map[string]any{"region": "kanto", "zip": "150-0002", "code": "007"},
		},
		{
			name: "--string keeps what follows the first = and accepts an empty value",
			args: []string{"set", "sakura-tokyo-prod", "--string", "query=a=b", "--string", "note="},
			want: map[string]any{"query": "a=b", "note": ""},
		},
		{
			name: "--json sets numbers, booleans, lists and objects",
			args: []string{
				"set", "sakura-tokyo-prod",
				"--json", "seats=40", "--json", "sso=true",
				"--json", `tags=["beta", "emea"]`, "--json", `owner={"team": "platform", "oncall": [1, 2]}`,
			},
			want: map[string]any{
				"seats": float64(40), "sso": true,
				"tags": []any{"beta", "emea"}, "owner": map[string]any{"team": "platform", "oncall": []any{float64(1), float64(2)}},
			},
		},
		{
			name: "--json null removes a key, beside --string in the same request",
			args: []string{"set", "sakura-tokyo-prod", "--string", "region=kanto", "--json", "legacy-plan=null"},
			want: map[string]any{"region": "kanto", "legacy-plan": nil},
		},
		{
			name: "--payload is merged as written, null removing",
			args: []string{"set", "sakura-tokyo-prod", "--payload", `{"region": "kansai", "legacy-plan": null}`},
			want: map[string]any{"region": "kansai", "legacy-plan": nil},
		},
		{
			name: "--file reads YAML, null removing",
			args: []string{"set", "sakura-tokyo-prod", "--file", metadataFile},
			want: map[string]any{"region": "kansai", "legacy-plan": nil, "seats": float64(40)},
		},
		{
			name: "unset sends each key as null",
			args: []string{"unset", "sakura-tokyo-prod", "legacy-plan", "trial-notes"},
			want: map[string]any{"legacy-plan": nil, "trial-notes": nil},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			api := givenRoutes(t, map[string]route{
				"PATCH " + instanceMetadataPath: {status: http.StatusNoContent},
			})
			givenIsolatedConfig(t)

			stdout := runSuccessfully(t, append([]string{"--base-url", api.URL, "instances", "metadata"}, test.args...)...)

			calls := api.calls()
			expectCalls(t, calls, "PATCH "+instanceMetadataPath)
			expectBody(t, calls[0].body, map[string]any{"metadata": test.want})
			if strings.TrimSpace(stdout) != "Instance metadata updated" {
				t.Fatalf("stdout = %q, want %q", stdout, "Instance metadata updated")
			}
		})
	}
}

// TestInstancesMetadataKeepsLargeIntegersAsWritten pins --json numbers to the digits the
// operator typed: read as a float64, an identifier past 2^53 would be rounded on the wire.
func TestInstancesMetadataKeepsLargeIntegersAsWritten(t *testing.T) {
	value, err := parseJSONValue("9007199254740993")
	if err != nil {
		t.Fatal(err)
	}
	if got := jsonString(value); got != "9007199254740993" {
		t.Fatalf("--json 9007199254740993 is sent as %s", got)
	}
}

// TestInstancesMetadataRefusesWhatItCannotSendAsWritten stops before the request on an
// invocation whose patch is ambiguous or empty: a key named twice, a pair without a key, a
// --json value that is not one JSON value (bare text most of all), an empty patch, or flags
// mixed with --payload. Every one is a usage error, exit code 2, and nothing is sent.
func TestInstancesMetadataRefusesWhatItCannotSendAsWritten(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		message string
	}{
		{name: "no key", args: []string{"set", "sakura-tokyo-prod"}, message: "provide one input source"},
		{name: "a pair without =", args: []string{"set", "sakura-tokyo-prod", "--string", "region"}, message: "--string takes KEY=VALUE"},
		{name: "an empty key", args: []string{"set", "sakura-tokyo-prod", "--json", "=1"}, message: "--json takes KEY=VALUE"},
		{name: "a key set twice", args: []string{"set", "sakura-tokyo-prod", "--string", "region=kanto", "--json", `region="kansai"`}, message: `"region" is set more than once`},
		{name: "bare text in --json", args: []string{"set", "sakura-tokyo-prod", "--json", "region=kanto"}, message: "text goes in --string"},
		{name: "an empty --json value", args: []string{"set", "sakura-tokyo-prod", "--json", "region="}, message: "no JSON value"},
		{name: "two JSON values", args: []string{"set", "sakura-tokyo-prod", "--json", "seats=1 2"}, message: "more than one JSON value"},
		{name: "an empty payload", args: []string{"set", "sakura-tokyo-prod", "--payload", "{}"}, message: "at least one metadata key"},
		{name: "flags and a payload", args: []string{"set", "sakura-tokyo-prod", "--string", "region=kanto", "--payload", `{"a": 1}`}, message: "exactly one input source"},
		{name: "unset without a key", args: []string{"unset", "sakura-tokyo-prod"}, message: "requires at least 2 arg(s)"},
		{name: "unset an empty key", args: []string{"unset", "sakura-tokyo-prod", " "}, message: "cannot be empty"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			api := givenRoutes(t, map[string]route{})

			code, err := executeForExitCode(t, append([]string{"--base-url", api.URL, "instances", "metadata"}, test.args...)...)

			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("Execute(%v) error = %v, want one containing %q", test.args, err, test.message)
			}
			expectExitCode(t, code, exitUsage)
			if calls := api.calls(); len(calls) != 0 {
				t.Fatalf("the API was called %d times, want the CLI to stop before the request", len(calls))
			}
		})
	}
}

// TestInstancesMetadataReportsTheAPIRefusal keeps the API's answers on the documented exit
// codes: a merged result a declared metadata field refuses, or a Kaiten too old to accept
// metadata on PATCH, is a 422 and exits 5; an unknown instance exits 4.
func TestInstancesMetadataReportsTheAPIRefusal(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   int
	}{
		{
			name:   "a metadata field refuses the merged result",
			status: http.StatusUnprocessableEntity,
			body:   `{"status": 422, "title": "Unprocessable Entity", "code": "MetadataValidation.Failed"}`,
			want:   exitRejected,
		},
		{
			name:   "the instance does not exist",
			status: http.StatusNotFound,
			body:   `{"status": 404, "title": "Not Found"}`,
			want:   exitNotFound,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			api := givenRoutes(t, map[string]route{
				"PATCH " + instanceMetadataPath: {status: test.status, body: test.body},
			})

			code, err := executeForExitCode(t, "--base-url", api.URL, "instances", "metadata", "set", "sakura-tokyo-prod", "--json", "seats=-1")

			if err == nil {
				t.Fatal("Execute() error = nil, want the API's refusal reported")
			}
			expectExitCode(t, code, test.want)
			expectCalls(t, api.calls(), "PATCH "+instanceMetadataPath)
		})
	}
}
