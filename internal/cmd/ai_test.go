package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

const (
	currentAIModel = `{"id":"model-id","slug":"assistant-v1","name":"Assistant","vendor":"acme","modality":"TEXT","status":"ACTIVE","description":"keep me","externalIds":["provider/model"],"metadata":{"keep":"metadata","exact":9007199254740993},"vendorCosts":{"currency":"USD","perInputToken":"0.000000000002","perOutputToken":"0.000000000003"},"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-02T00:00:00Z"}`
	currentAIRule  = `{"id":"rule-1","licenseSlug":"team-v2","entitlementSlug":"credits","modelSlug":"assistant-v1","requestType":"COMPLETION","rates":{"perInputToken":"0.1","perOutputToken":"0.2"},"minimumCreditsPerRequest":"1.000001","maximumCreditsPerRequest":"10.000001","chargeOnError":"ALWAYS","status":"ACTIVE","grantedByLicense":true,"createdAt":"2026-01-01T00:00:00Z"}`
)

func TestAIModelsReadsOnePageWithFiltersAndBasePrefix(t *testing.T) {
	server, api := givenUsageAPI(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("missing bearer token")
		}
		writeTestJSON(w, http.StatusOK, `{"items":[{"slug":"model-a"}],"hasMore":true,"nextCursor":"next-1"}`)
	})
	stdout, stderr, code := runCapturing(t, "--base-url", server.URL+"/api", "--auth-token", "test-token", "--output", "json", "ai", "models", "list", "--cursor", "first", "--limit", "20", "--vendor", "acme", "--status", "ACTIVE", "--modality", "TEXT")
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	r := api.only(t)
	if r.method != http.MethodGet || r.path != "/api/models" || r.query.Get("cursor") != "first" || r.query.Get("limit") != "20" || r.query.Get("vendor") != "acme" || r.query.Get("modality") != "TEXT" {
		t.Fatalf("request = %+v", r)
	}
	expectOutputContains(t, stdout, `"nextCursor": "next-1"`)
}

func TestAIConversionRuleUpdatePreservesDecimalsAndNull(t *testing.T) {
	server, api := givenUsageAPI(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
		if r.Method == http.MethodGet {
			writeTestJSON(w, http.StatusOK, currentAIRule)
			return
		}
		writeTestJSON(w, http.StatusOK, string(body))
	})
	for _, payload := range []string{
		`{"rates":{"perInputToken":"0.000001"},"minimumCreditsPerRequest":"999999999999.123456","maximumCreditsPerRequest":null,"modelSlug":"assistant-v1"}`,
		"rates:\n  perInputToken: '0.000001'\nminimumCreditsPerRequest: '999999999999.123456'\nmaximumCreditsPerRequest: null\nmodelSlug: assistant-v1",
	} {
		stdout, stderr, code := runCapturing(t, "--base-url", server.URL, "--output", "json", "ai", "conversion-rules", "update", "team-v2", "rule-1", "--payload", payload)
		if code != exitOK {
			t.Fatalf("exit %d: %s", code, stderr)
		}
		for _, want := range []string{`"minimumCreditsPerRequest": "999999999999.123456"`, `"modelSlug": "assistant-v1"`, `"maximumCreditsPerRequest": null`, `"chargeOnError": "ALWAYS"`} {
			expectOutputContains(t, stdout, want)
		}
	}
	if api.count() != 4 {
		t.Fatalf("requests = %d", api.count())
	}
}

func TestAIConversionRulesAreVersionScopedAndArchiveNeedsConfirmation(t *testing.T) {
	server, api := givenUsageAPI(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) { w.WriteHeader(http.StatusNoContent) })
	_, _, code := runCapturing(t, "--base-url", server.URL, "ai", "conversion-rules", "archive", "team-v2", "rule-1")
	if code != exitUsage || api.count() != 0 {
		t.Fatalf("exit = %d, requests = %d", code, api.count())
	}
	_, stderr, code := runCapturing(t, "--base-url", server.URL, "ai", "conversion-rules", "archive", "team-v2", "rule-1", "--yes")
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if r := api.only(t); r.path != "/licenses/team-v2/conversion-rules/rule-1" || r.method != http.MethodDelete {
		t.Fatalf("request = %+v", r)
	}
}

func TestAIReportUsesTransactionIdentityWithoutIdempotencyHeader(t *testing.T) {
	server, api := givenUsageAPI(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
		if r.Header.Get("Idempotency-Key") != "" {
			t.Errorf("AI report must not carry Idempotency-Key")
		}
		writeTestJSON(w, http.StatusOK, `{"value":{"type":"number","value":9007199254740993},"ai":{"phase":"commit","credits":"0.000001","transactionId":"call-1"}}`)
	})
	for _, phase := range []string{"reserve", "commit"} {
		payload := `{"value":{"type":"ai_usage","phase":"` + phase + `","transactionId":"call-1","model":"model-a","requestType":"COMPLETION","usage":{"inputTokens":9007199254740993,"units":"0.000001"}}}`
		stdout, stderr, code := runCapturing(t, "--base-url", server.URL, "--output", "json", "ai", "report", "acme", "credits", "--payload", payload)
		if code != exitOK {
			t.Fatalf("exit %d: %s", code, stderr)
		}
		expectOutputContains(t, stdout, `"value": 9007199254740993`)
		expectOutputContains(t, stdout, `"credits": "0.000001"`)
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	for _, r := range api.requests {
		if r.method != http.MethodPost || r.path != "/instances/acme/entitlements/credits/usage" || !strings.Contains(string(r.body), `"inputTokens":9007199254740993`) || !strings.Contains(string(r.body), `"transactionId":"call-1"`) {
			t.Fatalf("request = %+v", r)
		}
	}
}

func TestAIValidationDoesNotSendInvalidInput(t *testing.T) {
	server, api := givenUsageAPI(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) { writeTestJSON(w, http.StatusOK, `{}`) })
	cases := [][]string{
		{"models", "list", "--limit", "0"},
		{"models", "list", "--limit", "201"},
		{"inferences", "list", "acme", "credits", "--from", "yesterday"},
		{"conversion-rules", "create", "team-v1", "--payload", `{"rates":{"perInputToken":0.000001}}`},
		{"report", "acme", "credits", "--payload", `{"value":{"type":"ai_usage","phase":"reserve"}}`},
		{"report", "acme", "credits", "--payload", `{"behavior":"set","value":{"type":"ai_usage","phase":"commit","transactionId":"call-1"}}`},
	}
	for _, args := range cases {
		_, stderr, code := runCapturing(t, append([]string{"--base-url", server.URL, "ai"}, args...)...)
		if code != exitUsage {
			t.Fatalf("%v: exit %d: %s", args, code, stderr)
		}
	}
	if api.count() != 0 {
		t.Fatalf("requests = %d", api.count())
	}
}

func TestAIReadReportsAndErrorExitCodes(t *testing.T) {
	server, api := givenUsageAPI(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
		writeTestJSON(w, http.StatusOK, `{"items":[{"transactionId":"call-1","credits":"999999999999.123456"}],"hasMore":false}`)
	})
	stdout, stderr, code := runCapturing(t, "--base-url", server.URL, "--output", "ndjson", "ai", "inferences", "list", "acme", "credits", "--transaction-id", "call-1", "--status", "COMMITTED", "--limit", "500")
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	var record map[string]any
	if err := json.Unmarshal([]byte(stdout), &record); err != nil || record["credits"] != "999999999999.123456" {
		t.Fatalf("stdout = %s", stdout)
	}
	if r := api.only(t); r.query.Get("transactionId") != "call-1" || r.path != "/instances/acme/entitlements/credits/usage/inferences" {
		t.Fatalf("request = %+v", r)
	}
	for status, expected := range map[int]int{401: exitAuth, 403: exitAuth, 404: exitNotFound, 409: exitRejected, 422: exitRejected, 503: exitUnavailable} {
		s, _ := givenUsageAPI(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
			writeTestJSON(w, status, `{"title":"Refused","detail":"test"}`)
		})
		if _, _, code := runCapturing(t, "--base-url", s.URL, "ai", "models", "get", "model-a"); code != expected {
			t.Fatalf("status %d: exit %d, want %d", status, code, expected)
		}
	}
}

func TestAIReservationsPreserveNumericCountersAndEmptyNDJSON(t *testing.T) {
	for _, format := range []string{"json", "yaml"} {
		server, api := givenUsageAPI(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
			writeTestJSON(w, http.StatusOK, `{"items":[],"value":{"type":"number","value":9007199254740993},"credits":"999999999999.123456"}`)
		})
		stdout, stderr, code := runCapturing(t, "--base-url", server.URL, "--output", format, "ai", "reservations", "list", "acme", "credits")
		if code != exitOK {
			t.Fatalf("exit %d: %s", code, stderr)
		}
		expectOutputContains(t, stdout, "9007199254740993")
		expectOutputContains(t, stdout, "999999999999.123456")
		if r := api.only(t); r.method != http.MethodGet || r.path != "/instances/acme/entitlements/credits/usage/reservations" {
			t.Fatalf("request = %+v", r)
		}
	}
	for _, body := range []string{`{"items":null,"hasMore":false}`, `{"items":[],"hasMore":false}`, `[]`, `null`} {
		server, _ := givenUsageAPI(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) { writeTestJSON(w, http.StatusOK, body) })
		stdout, stderr, code := runCapturing(t, "--base-url", server.URL, "--output", "ndjson", "ai", "models", "list")
		if code != exitOK || stdout != "" {
			t.Fatalf("body %s: exit %d, stdout %q, stderr %s", body, code, stdout, stderr)
		}
	}
}

func TestAIReportsAreSentOnceOnFailure(t *testing.T) {
	server, api := givenUsageAPI(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
		writeTestJSON(w, http.StatusServiceUnavailable, `{"title":"Unavailable"}`)
	})
	_, _, code := runCapturing(t, "--base-url", server.URL, "ai", "report", "acme", "credits", "--payload", `{"value":{"type":"ai_usage","phase":"commit","transactionId":"call-1"}}`)
	if code != exitUnavailable || api.count() != 1 {
		t.Fatalf("exit %d, requests %d", code, api.count())
	}
}

func TestAIConversionRuleListFilters(t *testing.T) {
	server, api := givenUsageAPI(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) { writeTestJSON(w, http.StatusOK, `[]`) })
	_, stderr, code := runCapturing(t, "--base-url", server.URL, "ai", "conversion-rules", "list", "team-v2", "--status", "ARCHIVED", "--model", "assistant-v1", "--entitlement", "credits")
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if r := api.only(t); r.path != "/licenses/team-v2/conversion-rules" || r.query.Get("status") != "ARCHIVED" || r.query.Get("entitlementSlug") != "credits" || r.query.Get("modelSlug") != "assistant-v1" {
		t.Fatalf("request = %+v", r)
	}
}

func TestAIModelManagementPreservesCostsAndImmutableSlug(t *testing.T) {
	server, api := givenUsageAPI(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
		if r.Method == http.MethodGet {
			writeTestJSON(w, http.StatusOK, currentAIModel)
			return
		}
		writeTestJSON(w, http.StatusOK, string(body))
	})
	for _, args := range [][]string{
		{"create", "--payload", `{"slug":"assistant-v1","name":"Assistant","vendor":"acme","modality":"TEXT","vendorCosts":{"currency":"USD","perInputToken":"0.000000000001"}}`},
		{"update", "assistant-v1", "--payload", `{"slug":"assistant-v1","vendorCosts":{"currency":"USD","perInputToken":"0.000000000001"}}`},
	} {
		stdout, stderr, code := runCapturing(t, append([]string{"--base-url", server.URL, "--output", "json", "ai", "models"}, args...)...)
		if code != exitOK {
			t.Fatalf("exit %d: %s", code, stderr)
		}
		expectOutputContains(t, stdout, `"perInputToken": "0.000000000001"`)
	}
	api.mu.Lock()
	requests := append([]recordedUsageRequest(nil), api.requests...)
	api.mu.Unlock()
	if len(requests) != 3 || requests[0].method != http.MethodPost || requests[0].path != "/models" || requests[1].method != http.MethodGet || requests[2].method != http.MethodPut || requests[2].path != "/models/assistant-v1" {
		t.Fatalf("requests = %+v", requests)
	}
	for _, payload := range []string{`{"slug":"renamed-model"}`, `{"vendorCosts":{"perInputToken":0.000000000001}}`} {
		if _, _, code := runCapturing(t, "--base-url", server.URL, "ai", "models", "update", "assistant-v1", "--payload", payload); code != exitUsage {
			t.Fatalf("payload %s: exit %d", payload, code)
		}
	}
	if api.count() != 3 {
		t.Fatalf("invalid update sent: requests = %d", api.count())
	}
}

func TestAIModelDeleteRequiresConfirmation(t *testing.T) {
	server, api := givenUsageAPI(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) { w.WriteHeader(http.StatusNoContent) })
	if _, _, code := runCapturing(t, "--base-url", server.URL, "ai", "models", "delete", "assistant-v1"); code != exitUsage || api.count() != 0 {
		t.Fatalf("exit = %d, requests = %d", code, api.count())
	}
	stdout, stderr, code := runCapturing(t, "--base-url", server.URL, "ai", "models", "delete", "assistant-v1", "--yes")
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	expectOutputContains(t, stdout, "Catalogue model deleted")
	if r := api.only(t); r.method != http.MethodDelete || r.path != "/models/assistant-v1" {
		t.Fatalf("request = %+v", r)
	}
}

func TestAIModelUpdateMergesStatusAndIndividualPriceDimensions(t *testing.T) {
	for _, patch := range []string{`{"status":"DEPRECATED"}`, `{"vendorCosts":{"perInputToken":"0.000000000001"},"metadata":{"added":"new"}}`} {
		server, api := givenUsageAPI(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
			if r.Method == http.MethodGet {
				writeTestJSON(w, http.StatusOK, currentAIModel)
				return
			}
			var model aiDocument
			if err := json.Unmarshal(body, &model); err != nil {
				t.Errorf("decode body: %v", err)
			}
			for _, field := range []string{"slug", "name", "vendor", "modality", "status", "description", "externalIds", "metadata", "vendorCosts"} {
				if _, present := model[field]; !present {
					t.Errorf("PUT omitted %s", field)
				}
			}
			for _, field := range []string{"id", "createdAt", "updatedAt"} {
				if _, present := model[field]; present {
					t.Errorf("PUT included read-only %s", field)
				}
			}
			if model["slug"] != "assistant-v1" || model["name"] != "Assistant" || model["modality"] != "TEXT" || model["description"] != "keep me" {
				t.Errorf("lost current fields: %s", body)
			}
			costs := model["vendorCosts"].(map[string]any)
			metadata := model["metadata"].(map[string]any)
			if costs["currency"] != "USD" || costs["perOutputToken"] != "0.000000000003" || metadata["keep"] != "metadata" || metadata["exact"] != json.Number("9007199254740993") {
				t.Errorf("lost nested values: %s", body)
			}
			if strings.Contains(patch, "DEPRECATED") {
				if model["status"] != "DEPRECATED" || costs["perInputToken"] != "0.000000000002" {
					t.Errorf("status patch = %s", body)
				}
			} else if costs["perInputToken"] != "0.000000000001" || metadata["added"] != "new" || model["status"] != "ACTIVE" {
				t.Errorf("dimension patch = %s", body)
			}
			writeTestJSON(w, http.StatusOK, string(body))
		})
		_, stderr, code := runCapturing(t, "--base-url", server.URL, "ai", "models", "update", "assistant-v1", "--payload", patch)
		if code != exitOK || api.count() != 2 {
			t.Fatalf("exit %d, requests %d: %s", code, api.count(), stderr)
		}
	}
}

func TestAIConversionRuleUpdatePreservesOmittedPolicyAndReplacesRates(t *testing.T) {
	server, api := givenUsageAPI(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
		if r.Method == http.MethodGet {
			writeTestJSON(w, http.StatusOK, currentAIRule)
			return
		}
		var rule aiDocument
		if err := json.Unmarshal(body, &rule); err != nil {
			t.Errorf("decode body: %v", err)
		}
		for key, want := range map[string]any{"entitlementSlug": "credits", "modelSlug": "assistant-v1", "requestType": "COMPLETION", "minimumCreditsPerRequest": "1.000001", "maximumCreditsPerRequest": "10.000001", "chargeOnError": "ALWAYS"} {
			if rule[key] != want {
				t.Errorf("%s = %v, want %v", key, rule[key], want)
			}
		}
		rates := rule["rates"].(map[string]any)
		if len(rates) != 1 || rates["perInputToken"] != "0.000001" {
			t.Errorf("rates must be whole-map replacement: %s", body)
		}
		for _, field := range []string{"id", "licenseSlug", "status", "createdAt", "grantedByLicense"} {
			if _, present := rule[field]; present {
				t.Errorf("PUT included read-only %s", field)
			}
		}
		writeTestJSON(w, http.StatusOK, string(body))
	})
	_, stderr, code := runCapturing(t, "--base-url", server.URL, "ai", "conversion-rules", "update", "team-v2", "rule-1", "--payload", `{"rates":{"perInputToken":"0.000001"}}`)
	if code != exitOK || api.count() != 2 {
		t.Fatalf("exit %d, requests %d: %s", code, api.count(), stderr)
	}
}

func TestAIUpdatesRejectImmutableIdentitiesWithoutPUT(t *testing.T) {
	cases := []struct{ resource, target, patch, current string }{
		{"models", "assistant-v1", `{"modality":"IMAGE"}`, currentAIModel},
		{"conversion-rules", "team-v2", `{"rates":{},"entitlementSlug":"another"}`, currentAIRule},
		{"conversion-rules", "team-v2", `{"rates":{},"entitlementSlug":null}`, currentAIRule},
		{"conversion-rules", "team-v2", `{"rates":{},"modelSlug":"another"}`, currentAIRule},
		{"conversion-rules", "team-v2", `{"rates":{},"modelSlug":null}`, currentAIRule},
		{"conversion-rules", "team-v2", `{"rates":{},"requestType":"AUDIO"}`, currentAIRule},
		{"conversion-rules", "team-v2", `{"rates":{},"requestType":null}`, currentAIRule},
	}
	for _, tc := range cases {
		server, api := givenUsageAPI(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
			if r.Method != http.MethodGet {
				t.Errorf("immutable update sent %s", r.Method)
			}
			writeTestJSON(w, http.StatusOK, tc.current)
		})
		args := []string{"--base-url", server.URL, "ai", tc.resource, "update", tc.target}
		if tc.resource == "conversion-rules" {
			args = append(args, "rule-1")
		}
		_, _, code := runCapturing(t, append(args, "--payload", tc.patch)...)
		if code != exitUsage || api.count() != 1 {
			t.Fatalf("%s: exit %d, requests %d", tc.patch, code, api.count())
		}
	}
}

func TestAIUpdateStopsOnReadFailureAndRejectsReadOnlyPatch(t *testing.T) {
	server, api := givenUsageAPI(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
		if r.Method != http.MethodGet {
			t.Errorf("failed read followed by %s", r.Method)
		}
		writeTestJSON(w, http.StatusNotFound, `{"title":"Not found"}`)
	})
	if _, _, code := runCapturing(t, "--base-url", server.URL, "ai", "models", "update", "assistant-v1", "--payload", `{"status":"DEPRECATED"}`); code != exitNotFound || api.count() != 1 {
		t.Fatalf("exit %d, requests %d", code, api.count())
	}
	if _, _, code := runCapturing(t, "--base-url", server.URL, "ai", "models", "update", "assistant-v1", "--payload", `{"id":"rewrite"}`); code != exitUsage || api.count() != 1 {
		t.Fatalf("exit %d, requests %d", code, api.count())
	}
}
