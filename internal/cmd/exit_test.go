package cmd

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestExitCodeForAPIFailures(t *testing.T) {
	tests := []struct {
		name   string
		status int
		want   int
	}{
		{name: "401 means the credentials were refused", status: http.StatusUnauthorized, want: exitAuth},
		{name: "403 means the operation was not allowed", status: http.StatusForbidden, want: exitAuth},
		{name: "404 means the resource does not exist", status: http.StatusNotFound, want: exitNotFound},
		{name: "409 means the API refused the request", status: http.StatusConflict, want: exitRejected},
		{name: "422 means the API refused the request", status: http.StatusUnprocessableEntity, want: exitRejected},
		{name: "500 means the operation may succeed on a retry", status: http.StatusInternalServerError, want: exitUnavailable},
		{name: "503 means the operation may succeed on a retry", status: http.StatusServiceUnavailable, want: exitUnavailable},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := givenAPIRespondingWith(t, test.status)

			got, _ := executeForExitCode(t, "--base-url", server.URL, "components", "get", "api")

			expectExitCode(t, got, test.want)
		})
	}
}

func TestExitCodeForUnreachableAPI(t *testing.T) {
	// A server closed before the request is what a wrong port or a down API looks like.
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	baseURL := server.URL
	server.Close()

	got, _ := executeForExitCode(t, "--base-url", baseURL, "components", "list")

	expectExitCode(t, got, exitUnavailable)
}

func TestExitCodeForInterruptedRun(t *testing.T) {
	givenIsolatedConfig(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	root := NewRootCommand()
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--base-url", "http://127.0.0.1:1", "components", "list"})

	cmd, err := root.ExecuteContextC(ctx)

	if err == nil {
		t.Fatal("ExecuteContextC() error = nil, want a cancellation")
	}
	expectExitCode(t, exitCode(cmd, err), exitInterrupted)
}

func TestExitCodeForUsageMistakes(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "unknown top-level command", args: []string{"bogus"}},
		{name: "unknown subcommand", args: []string{"components", "bogus"}},
		{name: "unknown flag", args: []string{"components", "list", "--include-delete"}},
		{name: "missing required argument", args: []string{"components", "get"}},
		{name: "stray positional argument", args: []string{"components", "list", "stray"}},
		{name: "unsupported output format", args: []string{"--output", "toml", "components", "list"}},
		{name: "no base URL configured", args: []string{"components", "list"}},
		{name: "no input source given", args: []string{"components", "create"}},
		{
			name: "two input sources given at once",
			args: []string{"components", "create", "--payload", "{}", "--name", "api"},
		},
		{
			name: "inline flags missing a required field",
			args: []string{"--base-url", "http://127.0.0.1:1", "components", "create", "--name", "api"},
		},
		{
			name: "malformed RFC3339 timestamp",
			args: []string{"--base-url", "http://127.0.0.1:1", "instances", "audit-trails", "api", "--after", "yesterday"},
		},
		{
			name: "no license entitlement value given",
			args: []string{"--base-url", "http://127.0.0.1:1", "licenses", "entitlements", "associate", "ent", "seats"},
		},
		{
			name: "two license entitlement values given at once",
			args: []string{"--base-url", "http://127.0.0.1:1", "licenses", "entitlements", "associate", "ent", "seats", "--number", "1", "--boolean"},
		},
		{
			name: "unsupported config key",
			args: []string{"config", "set", "bogus", "value"},
		},
		{
			name: "a slug on the update of a resource the API never renames",
			args: []string{"--base-url", "http://127.0.0.1:1", "entitlements", "update", "webhook", "--name", "webhook", "--slug", "webhooks"},
		},
		{
			name: "destructive command with a non-interactive stdin and no --yes",
			args: []string{"--base-url", "http://127.0.0.1:1", "customers", "delete", "acme"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := executeForExitCode(t, test.args...)

			if err == nil {
				t.Fatal("Execute() error = nil, want a usage error")
			}
			if got != exitUsage {
				t.Fatalf("exit code = %d, want %d (error: %v)", got, exitUsage, err)
			}
		})
	}
}

func TestExitCodeForSuccessfulRun(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[],"nextCursor":null,"hasMore":false}`))
	}))
	defer server.Close()

	got, err := executeForExitCode(t, "--base-url", server.URL, "--output", "json", "components", "list")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	expectExitCode(t, got, exitOK)
}

func TestUsageIsPrintedForArgumentErrorsOnly(t *testing.T) {
	givenIsolatedConfig(t)
	server := givenAPIRespondingWith(t, http.StatusNotFound)

	argumentOutput := runForOutput(t, "components", "get")
	runtimeOutput := runForOutput(t, "--base-url", server.URL, "components", "get", "missing")

	if !strings.Contains(argumentOutput, "Usage:") {
		t.Fatalf("output = %q, want usage for an argument error", argumentOutput)
	}
	if strings.Contains(runtimeOutput, "Usage:") {
		t.Fatalf("output = %q, want no usage for a runtime failure", runtimeOutput)
	}
}

func TestEveryCommandValidatesItsArguments(t *testing.T) {
	// The root command is the one exception: leaving Args nil is what makes cobra report
	// an unknown top-level command with its "Did you mean this?" suggestions.
	var missing []string
	walkCommands(NewRootCommand(), func(cmd *cobra.Command) {
		if cmd.HasParent() && cmd.Args == nil {
			missing = append(missing, cmd.CommandPath())
		}
	})

	if len(missing) > 0 {
		t.Fatalf("commands accept arbitrary arguments: %s", strings.Join(missing, ", "))
	}
}

func TestOnlyTheRootCommandDefinesAPersistentPreRun(t *testing.T) {
	// exitCode and the usage silencing both depend on the root's persistent pre-run
	// running for every command, and cobra runs only the innermost one it finds.
	var shadowing []string
	walkCommands(NewRootCommand(), func(cmd *cobra.Command) {
		if cmd.HasParent() && (cmd.PersistentPreRunE != nil || cmd.PersistentPreRun != nil) {
			shadowing = append(shadowing, cmd.CommandPath())
		}
	})

	if len(shadowing) > 0 {
		t.Fatalf("commands shadow the root persistent pre-run: %s", strings.Join(shadowing, ", "))
	}
}

// executeForExitCode runs the root command with args and returns the exit code the CLI
// would report, along with the error it reported it for.
func executeForExitCode(t *testing.T, args ...string) (int, error) {
	t.Helper()
	givenIsolatedConfig(t)

	root := NewRootCommand()
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs(args)

	cmd, err := root.ExecuteC()
	return exitCode(cmd, err), err
}

// runForOutput runs the root command with args and returns everything it wrote.
func runForOutput(t *testing.T, args ...string) string {
	t.Helper()

	var out bytes.Buffer
	root := NewRootCommand()
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)

	if _, err := root.ExecuteC(); err == nil {
		t.Fatalf("Execute(%v) error = nil, want a failure", args)
	}

	return out.String()
}

// givenAPIRespondingWith serves status for every request, so a test can pin the exit code
// the CLI derives from one HTTP status without caring which endpoint produced it.
func givenAPIRespondingWith(t *testing.T, status int) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
	}))
	t.Cleanup(server.Close)

	return server
}

// givenIsolatedConfig points the CLI at an empty configuration so that the machine running
// the tests cannot supply a base URL, a token or an output format of its own. Both HOME and
// XDG_CONFIG_HOME are set because os.UserConfigDir consults a different one per platform.
func givenIsolatedConfig(t *testing.T) {
	t.Helper()

	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("KAITEN_BASE_URL", "")
	t.Setenv("KAITEN_AUTH_TOKEN", "")
	t.Setenv("KAITEN_OUTPUT", "")
}

func expectExitCode(t *testing.T, got, want int) {
	t.Helper()

	if got != want {
		t.Fatalf("exit code = %d, want %d", got, want)
	}
}

// walkCommands calls visit for cmd and every command below it.
func walkCommands(cmd *cobra.Command, visit func(*cobra.Command)) {
	visit(cmd)
	for _, sub := range cmd.Commands() {
		walkCommands(sub, visit)
	}
}
