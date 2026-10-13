package cmd

import (
	"net/http"
	"strings"
	"testing"
)

const emptyPage = `{"items": [], "nextCursor": null, "hasMore": false}`

// TestInstancesReadsOfferOnlyTheFiltersTheAPIReads pins `instances list` and `instances
// audit-trails` to the query parameters the API declares. Two flags were accepted and
// changed nothing: --include-deleted was sent as include_deleted, which the list takes no
// such parameter for and ignores, and --offset was not sent at all, the audit trail being
// paged by cursor. Neither is a flag any more, and neither parameter is sent.
func TestInstancesReadsOfferOnlyTheFiltersTheAPIReads(t *testing.T) {
	const auditTrails = "GET /instances/sakura-tokyo-prod/audit-trails"
	api := givenRoutes(t, map[string]route{
		"GET /instances": {status: http.StatusOK, body: emptyPage},
		auditTrails:      {status: http.StatusOK, body: emptyPage},
	})
	givenIsolatedConfig(t)

	runSuccessfully(t, "--base-url", api.URL, "instances", "list")
	runSuccessfully(t, "--base-url", api.URL, "instances", "audit-trails", "sakura-tokyo-prod",
		"--event-name", "INSTANCE_UPDATED", "--after", "2026-10-01T00:00:00Z", "--limit", "10")

	calls := api.calls()
	expectCalls(t, calls, "GET /instances", auditTrails)
	for _, call := range calls {
		for _, parameter := range []string{"include_deleted", "offset"} {
			if call.query.Has(parameter) {
				t.Errorf("%s was sent %s=%q, which the API does not read", call.call, parameter, call.query.Get(parameter))
			}
		}
	}
	// The filters the API does read still go out.
	for parameter, want := range map[string]string{
		"event_name": "INSTANCE_UPDATED", "after": "2026-10-01T00:00:00Z", "limit": "10",
	} {
		if got := calls[1].query.Get(parameter); got != want {
			t.Errorf("%s was sent %s=%q, want %q", calls[1].call, parameter, got, want)
		}
	}

	for _, refused := range []struct {
		flag string
		args []string
	}{
		{flag: "--include-deleted", args: []string{"instances", "list", "--include-deleted"}},
		{flag: "--offset", args: []string{"instances", "audit-trails", "sakura-tokyo-prod", "--offset", "5"}},
	} {
		code, err := executeForExitCode(t, append([]string{"--base-url", api.URL}, refused.args...)...)

		if err == nil || !strings.Contains(err.Error(), "unknown flag: "+refused.flag) {
			t.Fatalf("Execute(%v) error = %v, want %s refused as an unknown flag", refused.args, err, refused.flag)
		}
		expectExitCode(t, code, exitUsage)
	}
	if got := len(api.calls()); got != len(calls) {
		t.Fatalf("the API was called %d more times, want the CLI to stop before the request", got-len(calls))
	}
}
