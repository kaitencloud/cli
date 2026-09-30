package cmd

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// deleteServer serves DELETE /customers/{slug} and records whether it was ever reached.
func deleteServer(t *testing.T, called *bool) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want %q", r.Method, http.MethodDelete)
		}
		if r.URL.Path != "/customers/acme" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/customers/acme")
		}
		*called = true
		w.WriteHeader(http.StatusNoContent)
	}))
}

func TestDestructiveCommandRequiresConfirmationWhenStdinIsNotInteractive(t *testing.T) {
	called := false
	server := deleteServer(t, &called)
	defer server.Close()

	root := NewRootCommand()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetIn(strings.NewReader("y\n"))
	root.SetArgs([]string{"--base-url", server.URL, "customers", "delete", "acme"})

	err := root.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want a refusal")
	}
	if !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("Execute() error = %q, want it to mention --yes", err)
	}
	if called {
		t.Fatal("delete reached the API without confirmation")
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want no prompt on a non-interactive stdin", stderr.String())
	}
}

func TestDestructiveCommandSkipsPromptWithYes(t *testing.T) {
	called := false
	server := deleteServer(t, &called)
	defer server.Close()

	root := NewRootCommand()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"--base-url", server.URL, "customers", "delete", "--yes", "acme"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !called {
		t.Fatal("delete did not reach the API")
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want no prompt when --yes is given", stderr.String())
	}
}

func TestDestructiveCommandProceedsOnConfirmedPrompt(t *testing.T) {
	called := false
	server := deleteServer(t, &called)
	defer server.Close()

	stubTerminal(t)

	root := NewRootCommand()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetIn(strings.NewReader("y\n"))
	root.SetArgs([]string{"--base-url", server.URL, "customers", "delete", "acme"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !called {
		t.Fatal("delete did not reach the API after confirmation")
	}
	if !strings.Contains(stderr.String(), `customer "acme"`) {
		t.Fatalf("prompt = %q, want it to echo the target", stderr.String())
	}
}

func TestDestructiveCommandStopsOnDeclinedPrompt(t *testing.T) {
	called := false
	server := deleteServer(t, &called)
	defer server.Close()

	stubTerminal(t)

	root := NewRootCommand()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetIn(strings.NewReader("n\n"))
	root.SetArgs([]string{"--base-url", server.URL, "customers", "delete", "acme"})

	err := root.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want a cancellation")
	}
	if !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("Execute() error = %q, want a cancellation", err)
	}
	if called {
		t.Fatal("delete reached the API after the prompt was declined")
	}
}

func TestConfirmDestructiveRefusesEmptyTerminal(t *testing.T) {
	stubTerminal(t)

	cmd := NewRootCommand()
	addConfirmFlag(cmd)
	cmd.SetErr(io.Discard)
	cmd.SetIn(strings.NewReader(""))

	err := confirmDestructive(cmd, "delete", `customer "acme"`)
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("confirmDestructive() error = %v, want it to mention --yes", err)
	}
}

func TestEveryDestructiveCommandCarriesConfirmFlag(t *testing.T) {
	want := []string{
		"components delete",
		"customers delete",
		"deployment-zones delete",
		"entitlement-groups delete",
		"entitlement-groups remove-entitlement",
		"entitlements delete",
		"feature-flags delete",
		"instances delete",
		"licenses delete",
		"licenses entitlements delete",
		"releases delete",
		"service-accounts tokens delete",
		"config unset",
	}

	root := NewRootCommand()
	for _, path := range want {
		cmd, _, err := root.Find(strings.Split(path, " "))
		if err != nil {
			t.Fatalf("Find(%q) error = %v", path, err)
		}
		if cmd.Flags().Lookup(confirmFlagName) == nil {
			t.Fatalf("%q has no --%s flag", path, confirmFlagName)
		}
	}
}

// stubTerminal makes the confirmation helper treat the command's stdin as interactive.
func stubTerminal(t *testing.T) {
	t.Helper()

	original := isTerminal
	isTerminal = func(io.Reader) bool { return true }
	t.Cleanup(func() { isTerminal = original })
}
