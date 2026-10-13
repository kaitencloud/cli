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

// TestInstancesUpdateKeepsTheMetadataItDoesNotName pins what an inline update does with an
// instance's metadata. The API replaces the metadata with what it is sent, and the request
// cannot leave it out, so an update whose flags named none used to send an empty object and
// wipe it. The command now reads the instance first and sends its metadata back, unless
// --metadata-json gives the metadata to store -- an empty object included, which is how it
// is cleared on purpose. --file and --payload are still sent as written.
func TestInstancesUpdateKeepsTheMetadataItDoesNotName(t *testing.T) {
	const withMetadata = `{
		"id": "ins_1", "name": "Sakura Tokyo Prod", "slug": "sakura-tokyo-prod", "description": "Production",
		"customerId": "cus_1", "licenseId": "lic_1",
		"startLicenseDate": "2026-01-01T00:00:00Z", "endLicenseDate": "2027-01-01T00:00:00Z",
		"metadata": {"region": "kanto", "seats": 40}}`
	const withoutMetadata = `{
		"id": "ins_1", "name": "Sakura Tokyo Prod", "slug": "sakura-tokyo-prod", "description": "Production",
		"customerId": "cus_1", "licenseId": "lic_1",
		"startLicenseDate": "2026-01-01T00:00:00Z", "endLicenseDate": "2027-01-01T00:00:00Z"}`
	inline := []string{
		"--name", "Sakura Tokyo Prod", "--description", "Production, Shibuya", "--customer-id", "cus_1", "--license-id", "lic_1",
		"--start-license-date", "2026-01-01T00:00:00Z", "--end-license-date", "2027-01-01T00:00:00Z",
	}

	tests := []struct {
		name      string
		current   string
		args      []string
		wantCalls []string
		want      map[string]any
	}{
		{
			name:      "flags that name no metadata keep the instance's",
			current:   withMetadata,
			args:      inline,
			wantCalls: []string{"GET", "PUT"},
			want:      map[string]any{"region": "kanto", "seats": float64(40)},
		},
		{
			name:      "--metadata-json is sent as given, without reading the instance",
			current:   withMetadata,
			args:      append(append([]string{}, inline...), "--metadata-json", `{"region": "kansai"}`),
			wantCalls: []string{"PUT"},
			want:      map[string]any{"region": "kansai"},
		},
		{
			name:      "an empty --metadata-json clears the metadata",
			current:   withMetadata,
			args:      append(append([]string{}, inline...), "--metadata-json", `{}`),
			wantCalls: []string{"PUT"},
			want:      map[string]any{},
		},
		{
			name:      "an instance without metadata still sends an object",
			current:   withoutMetadata,
			args:      inline,
			wantCalls: []string{"GET", "PUT"},
			want:      map[string]any{},
		},
		{
			name:    "a payload is sent as written",
			current: withMetadata,
			args: []string{"--payload", `{
				"name": "Sakura Tokyo Prod", "description": "Production, Shibuya", "customerId": "cus_1", "licenseId": "lic_1",
				"startLicenseDate": "2026-01-01T00:00:00Z", "endLicenseDate": "2027-01-01T00:00:00Z",
				"metadata": {"region": "kansai"}}`},
			wantCalls: []string{"PUT"},
			want:      map[string]any{"region": "kansai"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			const path = "/instances/sakura-tokyo-prod"
			api := givenRoutes(t, map[string]route{
				"GET " + path: {status: http.StatusOK, body: test.current},
				"PUT " + path: {status: http.StatusNoContent},
			})
			givenIsolatedConfig(t)

			runSuccessfully(t, append([]string{"--base-url", api.URL, "instances", "update", "sakura-tokyo-prod"}, test.args...)...)

			calls := api.calls()
			wantCalls := make([]string, len(test.wantCalls))
			for i, method := range test.wantCalls {
				wantCalls[i] = method + " " + path
			}
			expectCalls(t, calls, wantCalls...)
			expectBody(t, calls[len(calls)-1].body, map[string]any{
				"name": "Sakura Tokyo Prod", "description": "Production, Shibuya", "customerId": "cus_1", "licenseId": "lic_1",
				"startLicenseDate": "2026-01-01T00:00:00Z", "endLicenseDate": "2027-01-01T00:00:00Z",
				"metadata": test.want,
			})
		})
	}
}

// TestInstancesUpdateStopsWhenTheInstanceCannotBeRead keeps the read a precondition: an
// update that cannot learn the metadata to keep sends nothing rather than an empty object.
func TestInstancesUpdateStopsWhenTheInstanceCannotBeRead(t *testing.T) {
	api := givenRoutes(t, map[string]route{
		"GET /instances/gone": {status: http.StatusNotFound, body: `{"status": 404, "title": "Not Found"}`},
	})

	code, err := executeForExitCode(t, "--base-url", api.URL, "instances", "update", "gone",
		"--name", "Gone", "--description", "Removed", "--customer-id", "cus_1", "--license-id", "lic_1",
		"--start-license-date", "2026-01-01T00:00:00Z", "--end-license-date", "2027-01-01T00:00:00Z")

	if err == nil {
		t.Fatal("Execute() error = nil, want the failed read reported")
	}
	expectExitCode(t, code, exitNotFound)
	expectCalls(t, api.calls(), "GET /instances/gone")
}
