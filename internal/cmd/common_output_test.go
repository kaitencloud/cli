package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kaitencloud/cli/internal/config"
	"github.com/kaitencloud/cli/internal/output"
	"gopkg.in/yaml.v3"
)

// TestCommandsRenderInEveryOutputFormat runs one command of every rendering shape - a list
// with columns, a detail view without them, a create returning a body, and mutations
// reporting only a message - under every supported output format, selected both by flag and
// by environment variable. Each run asserts the command succeeds, keeps the data the server
// returned, and, for json and yaml, writes a structured document rather than a human-readable
// line. It lives with common.go because writeStructured and writeMessage are what it pins.
func TestCommandsRenderInEveryOutputFormat(t *testing.T) {
	server := givenCustomerAPI(t)

	commands := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "list renders the collection the API returned",
			args: []string{"customers", "list"},
			want: "Acme",
		},
		{
			name: "get renders one customer",
			args: []string{"customers", "get", "acme"},
			want: "Acme",
		},
		{
			name: "create echoes the customer it created",
			args: []string{"customers", "create", "--name", "Acme"},
			want: "Acme",
		},
		{
			name: "token create surfaces the plaintext token exactly once",
			args: []string{"service-accounts", "tokens", "create", "ci", "--name", "deploy"},
			want: "plaintext-secret",
		},
		{
			name: "update reports a body-less mutation",
			args: []string{"customers", "update", "acme", "--name", "Acme"},
			want: "updated",
		},
		{
			name: "delete reports a body-less mutation",
			args: []string{"customers", "delete", "acme", "--yes"},
			want: "deleted",
		},
		{
			name: "version reports build information",
			args: []string{"version"},
			want: "Kaiten CLI",
		},
		{
			name: "config view reports the resolved configuration",
			args: []string{"config", "view"},
			want: "path",
		},
	}

	formats := []struct {
		name string
		flag string
		env  string
	}{
		{name: "no format selected"},
		{name: "table by flag", flag: output.FormatTable},
		{name: "json by flag", flag: output.FormatJSON},
		{name: "yaml by flag", flag: output.FormatYAML},
		{name: "table by environment", env: output.FormatTable},
		{name: "json by environment", env: output.FormatJSON},
		{name: "yaml by environment", env: output.FormatYAML},
	}

	for _, format := range formats {
		for _, command := range commands {
			t.Run(format.name+"/"+command.name, func(t *testing.T) {
				givenIsolatedConfig(t)
				t.Setenv(config.EnvOutput, format.env)

				args := []string{"--base-url", server.URL}
				if format.flag != "" {
					args = append(args, "--output", format.flag)
				}
				args = append(args, command.args...)

				stdout := runSuccessfully(t, args...)

				expectOutputContains(t, stdout, command.want)

				selected := format.flag + format.env
				switch selected {
				case output.FormatJSON:
					expectStructuredDocument(t, stdout, json.Unmarshal)
				case output.FormatYAML:
					expectStructuredDocument(t, stdout, yaml.Unmarshal)
				}
			})
		}
	}
}

func TestWriteMessageFallsBackToABareLineForNonStructuredFormats(t *testing.T) {
	server := givenCustomerAPI(t)
	givenIsolatedConfig(t)

	stdout := runSuccessfully(t, "--base-url", server.URL, "--output", "table", "customers", "update", "acme", "--name", "Acme")

	if got := strings.TrimSpace(stdout); got != "Customer updated" {
		t.Fatalf("stdout = %q, want the bare message line", got)
	}
}

// givenCustomerAPI serves the customer and token endpoints the rendering tests exercise.
func givenCustomerAPI(t *testing.T) *httptest.Server {
	t.Helper()

	const customerBody = `{"id":"cus_1","name":"Acme","slug":"acme","externalCustomerId":"crm-123","createdBy":{},"updatedBy":{}}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "GET /customers":
			writeTestJSON(w, http.StatusOK, `{"items":[`+customerBody+`],"nextCursor":null,"hasMore":false}`)
		case "GET /customers/acme":
			writeTestJSON(w, http.StatusOK, customerBody)
		case "POST /customers":
			writeTestJSON(w, http.StatusCreated, customerBody)
		case "POST /service-accounts/ci/tokens":
			writeTestJSON(w, http.StatusCreated, `{"id":"tok_1","name":"deploy","slug":"deploy","token":"plaintext-secret","createdBy":{}}`)
		case "PUT /customers/acme", "DELETE /customers/acme":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	t.Cleanup(server.Close)

	return server
}

// runSuccessfully drives a fresh command tree through the root, exactly as a user would,
// and returns everything it wrote to stdout.
func runSuccessfully(t *testing.T, args ...string) string {
	t.Helper()

	var stdout, stderr bytes.Buffer
	root := NewRootCommand()
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute(%v) error = %v; stderr = %q", args, err, stderr.String())
	}

	return stdout.String()
}

func expectOutputContains(t *testing.T, out, want string) {
	t.Helper()

	if !strings.Contains(out, want) {
		t.Fatalf("stdout = %q, want it to contain %q", out, want)
	}
}

// expectStructuredDocument fails unless out decodes to a mapping or a sequence, which rules
// out both undecodable output and a human-readable line that happens to parse as a scalar.
func expectStructuredDocument(t *testing.T, out string, unmarshal func([]byte, any) error) {
	t.Helper()

	var decoded any
	if err := unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("decode stdout error = %v, stdout = %q", err, out)
	}

	switch decoded.(type) {
	case map[string]any, []any:
	default:
		t.Fatalf("stdout = %q, want a structured document, got %T", out, decoded)
	}
}

func writeTestJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}
