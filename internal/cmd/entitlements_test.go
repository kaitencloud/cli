package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sync"
	"testing"
)

// periodicEntitlementBody is monthly-orders as the API reads it back: a NUMBER entitlement
// with a monthly window, a warning threshold, its presentation and a sale unit -- every field
// an update built from scratch would reset, or, for the window, be refused for omitting.
const periodicEntitlementBody = `{
  "id": "ent_1", "name": "Monthly Orders", "slug": "monthly-orders",
  "description": "Orders a restaurant can process per month.",
  "type": "NUMBER", "aggregationMethod": "SUM",
  "resetPeriod": "MONTH", "resetAnchor": "CALENDAR", "warningThresholdPercent": 80,
  "icon": "lucide:shopping-cart", "unitSingular": "order", "unitPlural": "orders",
  "saleUnitSingular": "pack", "saleUnitPlural": "packs", "saleUnitFactor": 10,
  "userFacing": true, "displayOrder": 2,
  "entitlementGroups": [{"id": "grp_1", "name": "Restaurant Operations", "slug": "restaurant-operations"}]
}`

// TestEntitlementsUpdateKeepsWhatItDoesNotChange pins the read-modify-write behind inline
// update flags. The API replaces an entitlement with what it is sent, so an update that
// named only --description used to reset the threshold, the presentation and the sale unit,
// and to be refused outright on a periodic entitlement, whose window it did not send back.
// The command now reads the entitlement first: the body it sends is what it read, with the
// named fields changed, and no groupSlugs -- left out, the groups stay as they are.
func TestEntitlementsUpdateKeepsWhatItDoesNotChange(t *testing.T) {
	api := givenRoutes(t, map[string]route{
		"GET /entitlements/monthly-orders": {status: http.StatusOK, body: periodicEntitlementBody},
		"PUT /entitlements/monthly-orders": {status: http.StatusNoContent},
	})
	givenIsolatedConfig(t)

	runSuccessfully(t, "--base-url", api.URL, "entitlements", "update", "monthly-orders",
		"--description", "Orders a restaurant can take per month.", "--warning-threshold-percent", "90")

	expectCalls(t, api.calls(), "GET /entitlements/monthly-orders", "PUT /entitlements/monthly-orders")
	expectBody(t, api.calls()[1].body, map[string]any{
		"name":                    "Monthly Orders",
		"description":             "Orders a restaurant can take per month.",
		"type":                    "NUMBER",
		"aggregationMethod":       "SUM",
		"resetPeriod":             "MONTH",
		"resetAnchor":             "CALENDAR",
		"warningThresholdPercent": float64(90),
		"icon":                    "lucide:shopping-cart",
		"unitSingular":            "order",
		"unitPlural":              "orders",
		"saleUnitSingular":        "pack",
		"saleUnitPlural":          "packs",
		"saleUnitFactor":          float64(10),
		"userFacing":              true,
		"displayOrder":            float64(2),
	})
}

// TestEntitlementsUpdateRemovesALabelGivenEmpty pins how a label is removed inline: an empty
// value leaves the field out of the request, which the API reads as no value.
func TestEntitlementsUpdateRemovesALabelGivenEmpty(t *testing.T) {
	api := givenRoutes(t, map[string]route{
		"GET /entitlements/monthly-orders": {status: http.StatusOK, body: periodicEntitlementBody},
		"PUT /entitlements/monthly-orders": {status: http.StatusNoContent},
	})
	givenIsolatedConfig(t)

	runSuccessfully(t, "--base-url", api.URL, "entitlements", "update", "monthly-orders", "--icon", "")

	body := api.calls()[1].body
	if _, ok := body["icon"]; ok {
		t.Fatalf("request body = %v, want no icon", body)
	}
	if body["unitPlural"] != "orders" {
		t.Fatalf("request body = %v, want the unit labels kept", body)
	}
}

// TestEntitlementsUpdateSendsAPayloadAsWritten keeps --payload a full replacement: it is the
// body the operator wrote, sent without reading the entitlement first.
func TestEntitlementsUpdateSendsAPayloadAsWritten(t *testing.T) {
	api := givenRoutes(t, map[string]route{
		"PUT /entitlements/menu-items": {status: http.StatusNoContent},
	})
	givenIsolatedConfig(t)

	runSuccessfully(t, "--base-url", api.URL, "entitlements", "update", "menu-items",
		"--payload", `{"name": "Menu Items", "warningThresholdPercent": 80}`)

	expectCalls(t, api.calls(), "PUT /entitlements/menu-items")
	expectBody(t, api.calls()[0].body, map[string]any{
		"name":                    "Menu Items",
		"description":             nil,
		"warningThresholdPercent": float64(80),
	})
}

// TestEntitlementsCreateSendsTheWindowThresholdAndPresentation pins the inline flags of the
// fields the API accepts on create beyond the name, type and aggregation.
func TestEntitlementsCreateSendsTheWindowThresholdAndPresentation(t *testing.T) {
	api := givenRoutes(t, map[string]route{
		"POST /entitlements": {status: http.StatusCreated, body: periodicEntitlementBody},
	})
	givenIsolatedConfig(t)

	runSuccessfully(t, "--base-url", api.URL, "entitlements", "create",
		"--name", "Monthly Orders", "--slug", "monthly-orders", "--type", "NUMBER", "--aggregation-method", "SUM",
		"--reset-period", "MONTH", "--reset-anchor", "CALENDAR", "--warning-threshold-percent", "80",
		"--icon", "lucide:shopping-cart", "--unit-singular", "order", "--unit-plural", "orders",
		"--user-facing", "--display-order", "2")

	expectBody(t, api.calls()[0].body, map[string]any{
		"name":                    "Monthly Orders",
		"description":             nil,
		"slug":                    "monthly-orders",
		"type":                    "NUMBER",
		"aggregationMethod":       "SUM",
		"resetPeriod":             "MONTH",
		"resetAnchor":             "CALENDAR",
		"warningThresholdPercent": float64(80),
		"icon":                    "lucide:shopping-cart",
		"unitSingular":            "order",
		"unitPlural":              "orders",
		"userFacing":              true,
		"displayOrder":            float64(2),
	})
}

// route is one canned answer of the fake API.
type route struct {
	status int
	body   string
}

// routedAPI is a fake API that answers each "METHOD path" with its route and records every
// call, in order, with its query and its decoded body.
type routedAPI struct {
	*httptest.Server

	mu       sync.Mutex
	recorded []recordedCall
}

type recordedCall struct {
	call  string
	query url.Values
	body  map[string]any
}

func (a *routedAPI) calls() []recordedCall {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]recordedCall(nil), a.recorded...)
}

func givenRoutes(t *testing.T, routes map[string]route) *routedAPI {
	t.Helper()

	api := &routedAPI{}
	api.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := r.Method + " " + r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &body); err != nil {
				t.Errorf("%s: undecodable body %q: %v", call, raw, err)
			}
		}
		api.mu.Lock()
		api.recorded = append(api.recorded, recordedCall{call: call, query: r.URL.Query(), body: body})
		api.mu.Unlock()

		answer, ok := routes[call]
		if !ok {
			t.Errorf("unexpected request %s", call)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if answer.body == "" {
			w.WriteHeader(answer.status)
			return
		}
		writeTestJSON(w, answer.status, answer.body)
	}))
	t.Cleanup(api.Close)

	return api
}

func expectCalls(t *testing.T, got []recordedCall, want ...string) {
	t.Helper()

	names := make([]string, len(got))
	for i, call := range got {
		names[i] = call.call
	}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("calls = %v, want %v", names, want)
	}
}

func expectBody(t *testing.T, got, want map[string]any) {
	t.Helper()

	if !reflect.DeepEqual(got, want) {
		gotJSON, _ := json.Marshal(got)
		wantJSON, _ := json.Marshal(want)
		t.Fatalf("request body = %s, want %s", gotJSON, wantJSON)
	}
}
