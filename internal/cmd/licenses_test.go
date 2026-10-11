package cmd

import (
	"net/http"
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

func numberGrant(value float64) map[string]any {
	return map[string]any{"type": "number", "value": value}
}
