package cmd

import (
	"net/http"
	"strings"
	"testing"
)

// TestPreviousComponentIDIsOfferedOnCreateOnly pins --previous-component-id to the command
// where it does something. The link to the version a component follows is set once, at
// creation: the API's update refuses the field, and the request built there never carried
// it, so `components update --previous-component-id` printed the component and exited 0
// with the link untouched. Create still sends it; update no longer takes the flag.
func TestPreviousComponentIDIsOfferedOnCreateOnly(t *testing.T) {
	const component = `{"id": "cmp_2", "name": "kitchen-display", "slug": "kitchen-display-1-3-0", "version": "1.3.0"}`
	api := givenRoutes(t, map[string]route{
		"POST /components":                      {status: http.StatusCreated, body: component},
		"PUT /components/kitchen-display-1-3-0": {status: http.StatusOK, body: component},
	})
	givenIsolatedConfig(t)

	runSuccessfully(t, "--base-url", api.URL, "components", "create",
		"--name", "kitchen-display", "--version", "1.3.0", "--previous-component-id", "cmp_1")
	runSuccessfully(t, "--base-url", api.URL, "components", "update", "kitchen-display-1-3-0",
		"--name", "kitchen-display", "--version", "1.3.1")

	calls := api.calls()
	expectCalls(t, calls, "POST /components", "PUT /components/kitchen-display-1-3-0")
	expectBody(t, calls[0].body, map[string]any{
		"name": "kitchen-display", "version": "1.3.0", "previousComponentId": "cmp_1",
	})
	expectBody(t, calls[1].body, map[string]any{"name": "kitchen-display", "version": "1.3.1"})

	code, err := executeForExitCode(t, "--base-url", api.URL, "components", "update", "kitchen-display-1-3-0",
		"--name", "kitchen-display", "--version", "1.3.1", "--previous-component-id", "cmp_1")

	if err == nil || !strings.Contains(err.Error(), "unknown flag: --previous-component-id") {
		t.Fatalf("Execute() error = %v, want --previous-component-id refused as an unknown flag", err)
	}
	expectExitCode(t, code, exitUsage)
	if got := len(api.calls()); got != len(calls) {
		t.Fatalf("the API was called %d more times, want the CLI to stop before the request", got-len(calls))
	}
}
