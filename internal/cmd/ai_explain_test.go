package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// inferenceRow carries every member the explainable-charges contract adds.
const inferenceRow = `{"transactionId":"req-42","status":"COMMITTED","reportedAt":"2026-11-11T10:00:00Z","model":"gpt-4o-2024-11-20","modelSlug":"gpt-4o",
	"requestType":"COMPLETION","usage":{"inputTokens":1000,"outputTokens":800},"authorized":true,
	"credits":{"reserved":"3.4","computed":"3.4","charged":"3.4","refunded":"1"},
	"pricing":{"ruleId":"3a9b8c7d-0000-4000-8000-000000000001","specificity":"EXACT_TYPED","rates":{"perInputToken":"0.001"},
		"minimumCreditsPerRequest":"0","maximumCreditsPerRequest":null,"chargeOnError":"NEVER"},
	"licenseId":"6f1c2a9e-0000-4000-8000-000000000002","licenseSlug":"pro","warnings":["MODEL_DEPRECATED"],"reportSeq":17,
	"vendorCost":{"reported":"0.0125","computed":"0.0122","currency":"USD"},
	"refunds":[{"id":"8d0e","credits":"1","reason":"duplicate call","reportSeq":18,"createdAt":"2026-11-11T11:00:00Z","createdBy":{"id":"u1","name":"Ada Lovelace"}}]}`

func TestAIReportVendorCostIsSetAsWrittenOnACommit(t *testing.T) {
	server, api := givenUsageAPI(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
		writeTestJSON(w, http.StatusOK, `{"ai":{"phase":"commit","transactionId":"call-1","rule":{"id":"r1","specificity":"ANY_ALL"}}}`)
	})
	payload := `{"value":{"type":"ai_usage","phase":"commit","transactionId":"call-1","usage":{"inputTokens":10}}}`
	stdout, stderr, code := runCapturing(t, "--base-url", server.URL, "--output", "json", "ai", "report", "acme", "credits", "--payload", payload, "--vendor-cost", "0.010")
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if body := string(api.only(t).body); !strings.Contains(body, `"vendorCost":"0.010"`) {
		t.Fatalf("body = %s, want vendorCost exactly as written", body)
	}
	expectOutputContains(t, stdout, `"specificity": "ANY_ALL"`)
}

func TestAIReportRefusesAVendorCostTheAPIWould(t *testing.T) {
	server, api := givenUsageAPI(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) { writeTestJSON(w, http.StatusOK, `{}`) })
	commit := `{"value":{"type":"ai_usage","phase":"commit","transactionId":"call-1"}}`
	cases := [][]string{
		{"--payload", `{"value":{"type":"ai_usage","phase":"reserve","transactionId":"call-1"}}`, "--vendor-cost", "0.01"},
		{"--payload", `{"value":{"type":"ai_usage","phase":"commit","transactionId":"call-1","requestStatus":"NOT_SENT"}}`, "--vendor-cost", "0.01"},
		{"--payload", commit, "--vendor-cost", "1e-3"},
		{"--payload", commit, "--vendor-cost", "0.0000000000001"},
		{"--payload", `{"value":{"type":"ai_usage","phase":"commit","transactionId":"call-1","vendorCost":0.01}}`},
		{"--payload", `{"value":{"type":"ai_usage","phase":"commit","transactionId":"call-1","vendorCost":"0.02"}}`, "--vendor-cost", "0.01"},
	}
	for _, args := range cases {
		_, stderr, code := runCapturing(t, append([]string{"--base-url", server.URL, "ai", "report", "acme", "credits"}, args...)...)
		if code != exitUsage {
			t.Fatalf("%v: exit %d: %s", args, code, stderr)
		}
	}
	if api.count() != 0 {
		t.Fatalf("requests = %d", api.count())
	}
}

func TestAIReportReusedTransactionSaysWhatARetryMustResend(t *testing.T) {
	server, _ := givenUsageAPI(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
		writeTestJSON(w, http.StatusConflict, `{"title":"Conflict","status":409,"code":"ReportEntitlementUsageMetric.TransactionIdReused","detail":"transactionId already used with another phase or body"}`)
	})
	_, stderr, code := runCapturingError(t, "--base-url", server.URL, "ai", "report", "acme", "credits", "--payload",
		`{"value":{"type":"ai_usage","phase":"commit","transactionId":"call-1"}}`, "--vendor-cost", "0.01")
	if code != exitRejected || !strings.Contains(stderr, "value.vendorCost byte for byte") {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
}

func TestAIInferencesListSendsTheNewFilters(t *testing.T) {
	server, api := givenUsageAPI(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
		writeTestJSON(w, http.StatusOK, `{"items":[],"hasMore":false}`)
	})
	_, stderr, code := runCapturing(t, "--base-url", server.URL, "--output", "json", "ai", "inferences", "list", "acme", "credits",
		"--authorized", "false", "--model-slug", "gpt-4o", "--model", "gpt-4o-2024-11-20", "--request-type", "EMBEDDING")
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	r := api.only(t)
	if r.path != "/instances/acme/entitlements/credits/usage/inferences" || r.query.Get("authorized") != "false" || r.query.Get("modelSlug") != "gpt-4o" ||
		r.query.Get("model") != "gpt-4o-2024-11-20" || r.query.Get("requestType") != "EMBEDDING" {
		t.Fatalf("request = %+v", r)
	}
}

func TestAIInferencesListWithoutAPoolListsTheOrganization(t *testing.T) {
	server, api := givenUsageAPI(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
		writeTestJSON(w, http.StatusOK, `{"items":[{"instanceSlug":"acme","entitlementSlug":"credits","transactionId":"req-42","status":"RELEASED","warnings":["NO_RESERVATION"],"refunds":[]}],"hasMore":false}`)
	})
	stdout, stderr, code := runCapturing(t, "--base-url", server.URL, "--output", "ndjson", "ai", "inferences", "list", "--instance", "acme", "--entitlement", "credits", "--status", "RELEASED")
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	r := api.only(t)
	if r.path != "/usage/inferences" || r.query.Get("instanceSlug") != "acme" || r.query.Get("entitlementSlug") != "credits" || r.query.Get("status") != "RELEASED" {
		t.Fatalf("request = %+v", r)
	}
	var record map[string]any
	if err := json.Unmarshal([]byte(stdout), &record); err != nil || record["instanceSlug"] != "acme" {
		t.Fatalf("stdout = %s", stdout)
	}
}

func TestAIInferencesListRefusesInvalidCombinations(t *testing.T) {
	server, api := givenUsageAPI(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) { writeTestJSON(w, http.StatusOK, `{}`) })
	cases := [][]string{
		{"acme"},
		{"acme", "credits", "--instance", "acme"},
		{"acme", "credits", "--authorized", "yes"},
		{"--request-type", "CHAT"},
	}
	for _, args := range cases {
		_, stderr, code := runCapturing(t, append([]string{"--base-url", server.URL, "ai", "inferences", "list"}, args...)...)
		if code != exitUsage {
			t.Fatalf("%v: exit %d: %s", args, code, stderr)
		}
	}
	if api.count() != 0 {
		t.Fatalf("requests = %d", api.count())
	}
}

// Table output shows what explains a charge; JSON keeps the record whole.
func TestAIInferencesTableShowsTheExplainingMembers(t *testing.T) {
	server, _ := givenUsageAPI(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
		writeTestJSON(w, http.StatusOK, `{"items":[`+inferenceRow+`],"hasMore":true,"nextCursor":"next-1"}`)
	})
	stdout, stderr, code := runCapturing(t, "--base-url", server.URL, "--output", "table", "ai", "inferences", "list", "acme", "credits")
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	for _, want := range []string{"RULE", "VENDOR COST", "req-42", "gpt-4o-2024-11-20 (gpt-4o)", "EXACT_TYPED 3a9b8c7d-0000-4000-8000-000000000001", "pro", "0.0125 / 0.0122 USD", "MODEL_DEPRECATED", "3.4", "1"} {
		expectOutputContains(t, stdout, want)
	}
	if strings.Contains(stdout, "INSTANCE") {
		t.Errorf("a pool's table carries the organization's columns:\n%s", stdout)
	}
	if !strings.Contains(stderr, "--cursor next-1") {
		t.Errorf("stderr = %q, want the next page's cursor", stderr)
	}

	stdout, _, code = runCapturing(t, "--base-url", server.URL, "--output", "json", "ai", "inferences", "list", "acme", "credits")
	if code != exitOK {
		t.Fatalf("exit %d", code)
	}
	for _, want := range []string{`"pricing"`, `"licenseId"`, `"reportSeq": 17`, `"refunded": "1"`, `"name": "Ada Lovelace"`, `"computed": "0.0122"`} {
		expectOutputContains(t, stdout, want)
	}
}

func TestAIConversionRulesTableShowsAuthorsAndGrants(t *testing.T) {
	server, _ := givenUsageAPI(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
		writeTestJSON(w, http.StatusOK, `[{"id":"rule-1","entitlementSlug":"credits","modelSlug":"assistant-v1","requestType":"COMPLETION","status":"ACTIVE",
			"grantedByLicense":false,"grantedByAddons":true,"createdBy":{"id":"u1","name":"Ada Lovelace"},"updatedBy":{"id":"u2","name":"Grace Hopper"}}]`)
	})
	stdout, stderr, code := runCapturing(t, "--base-url", server.URL, "ai", "conversion-rules", "list", "team-v2")
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	for _, want := range []string{"GRANTED BY ADD-ONS", "CREATED BY", "UPDATED BY", "Ada Lovelace", "Grace Hopper", "true", "false"} {
		expectOutputContains(t, stdout, want)
	}
}

const refundAnswer = `{"id":"8d0e","credits":"3.5","reason":"duplicate call","reportSeq":18,"createdAt":"2026-11-11T11:00:00Z","createdBy":{"id":"u1","name":"Ada Lovelace"}}`

func TestAIRefundNeedsConfirmation(t *testing.T) {
	server, api := givenUsageAPI(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) { writeTestJSON(w, http.StatusOK, refundAnswer) })
	_, stderr, code := runCapturingError(t, "--base-url", server.URL, "ai", "inferences", "refund", "acme", "credits", "req-42")
	if code != exitUsage || api.count() != 0 || !strings.Contains(stderr, "--yes") {
		t.Fatalf("exit %d, requests %d, stderr %q", code, api.count(), stderr)
	}
}

func TestAIRefundSendsItsKeyAndBody(t *testing.T) {
	server, api := givenUsageAPI(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
		w.Header().Set("Idempotent-Replayed", "true")
		writeTestJSON(w, http.StatusOK, refundAnswer)
	})
	stdout, stderr, code := runCapturing(t, "--base-url", server.URL, "ai", "inferences", "refund", "acme", "credits", "req/42",
		"--credits", "3.5", "--reason", "duplicate call", "--idempotency-key", "refund-req-42-1", "--yes")
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	r := api.only(t)
	if r.method != http.MethodPost || r.path != "/instances/acme/entitlements/credits/usage/inferences/req/42/refund" {
		t.Fatalf("request = %+v", r)
	}
	if r.header.Get("Idempotency-Key") != "refund-req-42-1" || string(r.body) != `{"credits":"3.5","reason":"duplicate call"}` {
		t.Fatalf("key %q, body %s", r.header.Get("Idempotency-Key"), r.body)
	}
	for _, want := range []string{"CREDITS", "3.5", "duplicate call", "18", "Ada Lovelace"} {
		expectOutputContains(t, stdout, want)
	}
	if !strings.Contains(stderr, "already recorded under this idempotency key") || strings.Contains(stderr, "idempotency key: ") {
		t.Errorf("stderr = %q, want the replay note and no generated key", stderr)
	}
}

// Without a key one is generated and printed, so an unknown outcome can be retried.
func TestAIRefundGeneratesAndPrintsAKey(t *testing.T) {
	server, api := givenUsageAPI(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) { writeTestJSON(w, http.StatusOK, refundAnswer) })
	stdout, stderr, code := runCapturing(t, "--base-url", server.URL, "--output", "json", "ai", "inferences", "refund", "acme", "credits", "req-42", "--yes")
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	r := api.only(t)
	key := r.header.Get("Idempotency-Key")
	if !idempotencyKeyForm.MatchString(key) || !strings.HasPrefix(key, "cli-refund-") || !strings.Contains(stderr, "--idempotency-key "+key) {
		t.Fatalf("key %q, stderr %q", key, stderr)
	}
	if string(r.body) != `{}` {
		t.Fatalf("body = %s, want {} to refund everything left", r.body)
	}
	expectOutputContains(t, stdout, `"reportSeq": 18`)
}

func TestAIRefundValidatesItsFlags(t *testing.T) {
	server, api := givenUsageAPI(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) { writeTestJSON(w, http.StatusOK, refundAnswer) })
	cases := [][]string{
		{"--credits", "0"},
		{"--credits", "-1"},
		{"--credits", "1.1234567"},
		{"--reason", ""},
		{"--reason", strings.Repeat("x", 501)},
		{"--idempotency-key", "not a key"},
	}
	for _, args := range cases {
		_, stderr, code := runCapturing(t, append([]string{"--base-url", server.URL, "ai", "inferences", "refund", "acme", "credits", "req-42", "--yes"}, args...)...)
		if code != exitUsage {
			t.Fatalf("%v: exit %d: %s", args, code, stderr)
		}
	}
	if api.count() != 0 {
		t.Fatalf("requests = %d", api.count())
	}
}

func TestAIRefundExplainsEachRefusal(t *testing.T) {
	cases := []struct {
		status int
		code   string
		exit   int
		want   string
	}{
		{http.StatusForbidden, "AIUsage.Disabled", exitAuth, "AI usage is disabled"},
		{http.StatusNotFound, "RefundAiInference.ContextNotFound", exitNotFound, "no such instance or AI credit pool"},
		{http.StatusNotFound, "RefundAiInference.InferenceNotFound", exitNotFound, "no inference with this transaction id"},
		{http.StatusConflict, "RefundAiInference.NotCommitted", exitRejected, "only a COMMITTED inference"},
		{http.StatusConflict, "RefundAiInference.NothingToRefund", exitRejected, "already refunded in full"},
		{http.StatusConflict, "RefundAiInference.IdempotencyKeyReused", exitRejected, "under a new key"},
		{http.StatusConflict, "RefundAiInference.PoolNotGranted", exitRejected, "no longer holds this credit pool"},
		{http.StatusUnprocessableEntity, "RefundAiInference.IdempotencyKeyRequired", exitRejected, "needs an idempotency key"},
		{http.StatusUnprocessableEntity, "RefundAiInference.InvalidIdempotencyKey", exitRejected, "--idempotency-key must be"},
		{http.StatusUnprocessableEntity, "RefundAiInference.InvalidAmount", exitRejected, "--credits must be"},
		{http.StatusUnprocessableEntity, "RefundAiInference.InvalidReason", exitRejected, "--reason must be"},
	}
	for _, tc := range cases {
		server, _ := givenUsageAPI(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
			writeTestJSON(w, tc.status, `{"title":"Refused","status":`+jsonInt(tc.status)+`,"code":"`+tc.code+`"}`)
		})
		_, stderr, code := runCapturingError(t, "--base-url", server.URL, "ai", "inferences", "refund", "acme", "credits", "req-42", "--yes")
		if code != tc.exit || !strings.Contains(stderr, tc.want) {
			t.Errorf("%s: exit %d (want %d), stderr %q", tc.code, code, tc.exit, stderr)
		}
	}

	server, _ := givenUsageAPI(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
		writeTestJSON(w, http.StatusConflict, `{"title":"Conflict","status":409,"code":"RefundAiInference.ExceedsRefundable",
			"errors":[{"message":"credits left to refund on this inference","location":"body.credits","value":"1.5"}]}`)
	})
	_, stderr, code := runCapturingError(t, "--base-url", server.URL, "ai", "inferences", "refund", "acme", "credits", "req-42", "--credits", "9", "--yes")
	if code != exitRejected || !strings.Contains(stderr, "1.5 left") {
		t.Errorf("ExceedsRefundable: exit %d, stderr %q", code, stderr)
	}
}

func jsonInt(value int) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

// runCapturingError is runCapturing with the error line Execute prints on stderr, which
// the root command leaves to its caller.
func runCapturingError(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	var stdout, stderr strings.Builder
	root := NewRootCommand()
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)
	cmd, err := root.ExecuteC()
	if err != nil {
		stderr.WriteString(err.Error())
	}
	return stdout.String(), stderr.String(), exitCode(cmd, err)
}
