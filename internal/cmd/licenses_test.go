package cmd

import (
	"net/http"
	"strings"
	"testing"
)

// TestLicensesEntitlementsUpdateKeepsTheAllowance pins what an update does with a grant's
// allowance. The API replaces the grant and derives an omitted allowance from the value --
// none, for a finite one -- so an update that changed only the value used to turn a soft
// limit into a hard one. Without --overage-percent the command now reads the grant first
// and keeps its allowance when it still fits the new value; with it, it sends the one given
// and reads nothing.
func TestLicensesEntitlementsUpdateKeepsTheAllowance(t *testing.T) {
	const softGrant = `{"entitlementSlug": "monthly-orders", "value": {"type": "number", "value": 1000}, "limitCapExceededOveragePercent": 20}`
	const unlimitedGrant = `{"entitlementSlug": "locations", "value": {"type": "number", "value": -1}, "limitCapExceededOveragePercent": -1}`
	const booleanGrant = `{"entitlementSlug": "delivery-tracking", "value": {"type": "boolean", "value": true}}`

	tests := []struct {
		name      string
		current   string
		args      []string
		wantCalls []string
		want      map[string]any
	}{
		{
			name:      "a new finite value keeps the soft limit",
			current:   softGrant,
			args:      []string{"--number", "1500"},
			wantCalls: []string{"GET", "PUT"},
			want:      map[string]any{"value": numberGrant(1500), "limitCapExceededOveragePercent": float64(20)},
		},
		{
			name:      "--overage-percent is sent as given, without reading the grant",
			current:   softGrant,
			args:      []string{"--number", "1500", "--overage-percent", "0"},
			wantCalls: []string{"PUT"},
			want:      map[string]any{"value": numberGrant(1500), "limitCapExceededOveragePercent": float64(0)},
		},
		{
			name:      "an unlimited value leaves the allowance to the API",
			current:   softGrant,
			args:      []string{"--number", "-1"},
			wantCalls: []string{"GET", "PUT"},
			want:      map[string]any{"value": numberGrant(-1)},
		},
		{
			name:      "an unlimited grant has no allowance to keep",
			current:   unlimitedGrant,
			args:      []string{"--number", "5"},
			wantCalls: []string{"GET", "PUT"},
			want:      map[string]any{"value": numberGrant(5)},
		},
		{
			name:      "a boolean grant has none either",
			current:   booleanGrant,
			args:      []string{"--boolean=false"},
			wantCalls: []string{"GET", "PUT"},
			want:      map[string]any{"value": map[string]any{"type": "boolean", "value": false}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			const path = "/licenses/premium-v2/entitlements/monthly-orders"
			api := givenRoutes(t, map[string]route{
				"GET " + path: {status: http.StatusOK, body: test.current},
				"PUT " + path: {status: http.StatusNoContent},
			})
			givenIsolatedConfig(t)

			args := append([]string{"--base-url", api.URL, "licenses", "entitlements", "update", "premium-v2", "monthly-orders"}, test.args...)
			runSuccessfully(t, args...)

			calls := api.calls()
			wantCalls := make([]string, len(test.wantCalls))
			for i, method := range test.wantCalls {
				wantCalls[i] = method + " " + path
			}
			expectCalls(t, calls, wantCalls...)
			expectBody(t, calls[len(calls)-1].body, test.want)
		})
	}
}

// TestLicensesEntitlementsAssociateSendsTheAllowance pins --overage-percent on a new grant:
// sent when given, absent otherwise, so the API derives it from the value as before.
func TestLicensesEntitlementsAssociateSendsTheAllowance(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want map[string]any
	}{
		{
			name: "with an allowance",
			args: []string{"--number", "100", "--overage-percent", "10"},
			want: map[string]any{"entitlementSlug": "monthly-orders", "value": numberGrant(100), "limitCapExceededOveragePercent": float64(10)},
		},
		{
			name: "without one",
			args: []string{"--number", "100"},
			want: map[string]any{"entitlementSlug": "monthly-orders", "value": numberGrant(100)},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			api := givenRoutes(t, map[string]route{
				"POST /licenses/standard/entitlements": {status: http.StatusNoContent},
			})
			givenIsolatedConfig(t)

			args := append([]string{"--base-url", api.URL, "licenses", "entitlements", "associate", "standard", "monthly-orders"}, test.args...)
			runSuccessfully(t, args...)

			expectBody(t, api.calls()[0].body, test.want)
		})
	}
}

// TestLicensesCreateLeavesTheVersionToTheAPI pins create to what the API takes. It numbers
// the versions of a license itself and refuses any it is sent, so the request has no
// version field: --version used to be required there and then dropped. It is no longer a
// flag of create, so passing it is refused before anything is sent.
func TestLicensesCreateLeavesTheVersionToTheAPI(t *testing.T) {
	api := givenRoutes(t, map[string]route{
		"POST /licenses": {status: http.StatusCreated, body: `{"name": "Premium", "slug": "premium", "version": "1"}`},
	})
	givenIsolatedConfig(t)
	create := []string{
		"--base-url", api.URL, "licenses", "create",
		"--name", "Premium", "--description", "Every feature.", "--type", "PAID", "--default=false",
	}

	runSuccessfully(t, create...)

	expectCalls(t, api.calls(), "POST /licenses")
	expectBody(t, api.calls()[0].body, map[string]any{
		"name": "Premium", "description": "Every feature.", "type": "PAID", "isDefault": false,
	})

	code, err := executeForExitCode(t, append(create, "--version", "2")...)

	if err == nil || !strings.Contains(err.Error(), "unknown flag: --version") {
		t.Fatalf("Execute() error = %v, want --version refused as an unknown flag", err)
	}
	expectExitCode(t, code, exitUsage)
	expectCalls(t, api.calls(), "POST /licenses")
}

// TestLicensesUpdateSendsTheVersionAsACheck pins what is left of --version on update. The
// API accepts there the version the license already has and refuses any other, so the flag
// is sent as given for the API to check, and is no longer required: without it the version
// goes out empty, which the API reads as the one it holds.
func TestLicensesUpdateSendsTheVersionAsACheck(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "without --version the update names none", want: ""},
		{name: "with --version it is sent for the API to check", args: []string{"--version", "2"}, want: "2"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			api := givenRoutes(t, map[string]route{
				"PUT /licenses/premium-v2": {status: http.StatusNoContent},
			})
			givenIsolatedConfig(t)

			args := append([]string{
				"--base-url", api.URL, "licenses", "update", "premium-v2",
				"--name", "Premium", "--description", "Every feature.", "--type", "PAID", "--default=false",
			}, test.args...)
			runSuccessfully(t, args...)

			expectCalls(t, api.calls(), "PUT /licenses/premium-v2")
			expectBody(t, api.calls()[0].body, map[string]any{
				"name": "Premium", "description": "Every feature.", "type": "PAID", "isDefault": false,
				"version": test.want,
			})
		})
	}
}

// TestLicensesUpdateReportsARefusedVersion checks the other end of the check: the API's
// refusal of a version the license does not have reaches the operator as a refused request.
func TestLicensesUpdateReportsARefusedVersion(t *testing.T) {
	api := givenRoutes(t, map[string]route{
		"PUT /licenses/premium-v2": {status: http.StatusUnprocessableEntity, body: `{
			"status": 422, "title": "Unprocessable Entity", "code": "UpdateLicense.VersionNotSettable",
			"detail": "License \"premium-v2\" is version 2; the version is assigned by the server and cannot be changed"}`},
	})

	code, err := executeForExitCode(t, "--base-url", api.URL, "licenses", "update", "premium-v2",
		"--name", "Premium", "--description", "Every feature.", "--type", "PAID", "--default=false", "--version", "3")

	if err == nil || !strings.Contains(err.Error(), "UpdateLicense.VersionNotSettable") {
		t.Fatalf("Execute() error = %v, want the API's refusal", err)
	}
	expectExitCode(t, code, exitRejected)
}

func numberGrant(value float64) map[string]any {
	return map[string]any{"type": "number", "value": value}
}
