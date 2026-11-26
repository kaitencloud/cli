package cmd

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
)

const serviceAccountPath = "/service-accounts/deploy-bot"

// TestServiceAccountsDeleteSendsOneDelete pins `service-accounts delete` to a single DELETE
// of the account: the API revokes the tokens it holds in the same operation, so the CLI
// neither lists nor deletes them first.
func TestServiceAccountsDeleteSendsOneDelete(t *testing.T) {
	api := givenRoutes(t, map[string]route{
		"DELETE " + serviceAccountPath: {status: http.StatusNoContent},
	})
	givenIsolatedConfig(t)

	stdout := runSuccessfully(t, "--base-url", api.URL, "service-accounts", "delete", "deploy-bot", "--yes")

	expectCalls(t, api.calls(), "DELETE "+serviceAccountPath)
	if strings.TrimSpace(stdout) != "Service account deleted" {
		t.Fatalf("stdout = %q, want %q", stdout, "Service account deleted")
	}
}

// TestServiceAccountsDeleteAsksFirst keeps the delete behind the confirmation every
// destructive command has. The prompt names the tokens, because they stop authenticating
// with the account; a declined prompt and a stdin nobody can answer send nothing.
func TestServiceAccountsDeleteAsksFirst(t *testing.T) {
	tests := []struct {
		name     string
		terminal bool
		stdin    string
		want     int
		message  string
	}{
		{name: "a confirmed prompt deletes", terminal: true, stdin: "y\n", want: exitOK},
		{name: "a declined prompt cancels", terminal: true, stdin: "n\n", want: exitError, message: "cancelled"},
		{name: "a stdin that cannot answer refuses", terminal: false, stdin: "y\n", want: exitUsage, message: "--yes"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			api := givenRoutes(t, map[string]route{
				"DELETE " + serviceAccountPath: {status: http.StatusNoContent},
			})
			if test.terminal {
				stubTerminal(t)
			}

			code, stderr, err := runServiceAccountsDelete(t, api.URL, test.stdin)

			expectExitCode(t, code, test.want)
			if test.message != "" && (err == nil || !strings.Contains(err.Error(), test.message)) {
				t.Fatalf("Execute() error = %v, want one containing %q", err, test.message)
			}
			if test.terminal && !strings.Contains(stderr, `service account "deploy-bot" and its tokens`) {
				t.Fatalf("prompt = %q, want it to name the account and its tokens", stderr)
			}
			if test.want == exitOK {
				expectCalls(t, api.calls(), "DELETE "+serviceAccountPath)
			} else if calls := api.calls(); len(calls) != 0 {
				t.Fatalf("the API was called %d times, want nothing sent without confirmation", len(calls))
			}
		})
	}
}

// TestServiceAccountsDeleteReportsTheAPIRefusal keeps the API's answers on the documented
// exit codes and its problem code in the message: an account the organization does not
// hold, or one already deleted, exits 4 with ServiceAccount.NotFound; a token without
// write:tokens exits 3.
func TestServiceAccountsDeleteReportsTheAPIRefusal(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		want    int
		message string
	}{
		{
			name:    "the account does not exist",
			status:  http.StatusNotFound,
			body:    `{"status": 404, "title": "Not Found", "code": "ServiceAccount.NotFound"}`,
			want:    exitNotFound,
			message: "ServiceAccount.NotFound",
		},
		{
			name:    "the token lacks the scope",
			status:  http.StatusForbidden,
			body:    `{"status": 403, "title": "Forbidden"}`,
			want:    exitAuth,
			message: "403",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			api := givenRoutes(t, map[string]route{
				"DELETE " + serviceAccountPath: {status: test.status, body: test.body},
			})

			code, err := executeForExitCode(t, "--base-url", api.URL, "service-accounts", "delete", "deploy-bot", "--yes")

			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("Execute() error = %v, want one containing %q", err, test.message)
			}
			expectExitCode(t, code, test.want)
			expectCalls(t, api.calls(), "DELETE "+serviceAccountPath)
		})
	}
}

// runServiceAccountsDelete runs `service-accounts delete deploy-bot` without --yes, with
// stdin as the operator's answer, and returns the exit code, what it wrote to stderr and
// the error it reported.
func runServiceAccountsDelete(t *testing.T, baseURL, stdin string) (int, string, error) {
	t.Helper()
	givenIsolatedConfig(t)

	var stderr bytes.Buffer
	root := NewRootCommand()
	root.SetOut(io.Discard)
	root.SetErr(&stderr)
	root.SetIn(strings.NewReader(stdin))
	root.SetArgs([]string{"--base-url", baseURL, "service-accounts", "delete", "deploy-bot"})

	cmd, err := root.ExecuteC()
	return exitCode(cmd, err), stderr.String(), err
}
