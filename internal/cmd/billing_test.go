package cmd

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const claimBody = `{"leaseId":"lease-1","leasedUntil":"2027-01-01T00:15:00Z","invoices":[
	{"id":"inv-1","customerSlug":"acme","total":1200,"currency":"eur","handoff":{"status":"PENDING","claimCount":1,"leaseId":"lease-1"}},
	{"id":"inv-2","customerSlug":"globex","total":800,"currency":"eur","handoff":{"status":"PENDING","claimCount":1,"leaseId":"lease-1"}}]}`

func TestBillingHandoffClaimPrintsOneInvoicePerLineInNDJSON(t *testing.T) {
	server, api := givenUsageAPI(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
		writeTestJSON(w, http.StatusOK, claimBody)
	})

	stdout, stderr, code := runCapturing(t, "--base-url", server.URL, "--output", "ndjson", "billing", "handoff", "claim", "--limit", "50", "--lease", "30m")
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}

	request := api.only(t)
	if request.method != http.MethodPost || request.path != "/billing/handoff/claim" {
		t.Fatalf("request = %s %s", request.method, request.path)
	}
	var body map[string]any
	if err := json.Unmarshal(request.body, &body); err != nil || body["limit"] != 50.0 || body["leaseSeconds"] != 1800.0 {
		t.Fatalf("body = %s", request.body)
	}

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 2 {
		t.Fatalf("stdout = %q, want two lines", stdout)
	}
	var invoice struct {
		ID      string `json:"id"`
		Handoff struct {
			LeaseID string `json:"leaseId"`
		} `json:"handoff"`
	}
	if err := json.Unmarshal([]byte(lines[1]), &invoice); err != nil || invoice.ID != "inv-2" || invoice.Handoff.LeaseID != "lease-1" {
		t.Fatalf("line = %q", lines[1])
	}
}

func TestBillingHandoffClaimRefusesALeaseOutOfRange(t *testing.T) {
	server, api := givenUsageAPI(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
		writeTestJSON(w, http.StatusOK, claimBody)
	})

	for _, args := range [][]string{{"--lease", "30s"}, {"--lease", "2h"}, {"--limit", "0"}, {"--limit", "101"}} {
		_, _, code := runCapturing(t, append([]string{"--base-url", server.URL, "billing", "handoff", "claim"}, args...)...)
		if code != exitUsage {
			t.Fatalf("%v: exit %d, want %d", args, code, exitUsage)
		}
	}
	if api.count() != 0 {
		t.Fatalf("requests = %d, want none", api.count())
	}
}

func TestBillingHandoffAckSendsTheLease(t *testing.T) {
	server, api := givenUsageAPI(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
		writeTestJSON(w, http.StatusOK, `{"id":"inv-1","status":"PAID","handoffStatus":"ACKNOWLEDGED","total":1200,"currency":"eur"}`)
	})

	stdout, stderr, code := runCapturing(t, "--base-url", server.URL, "billing", "handoff", "ack", "inv-1", "--lease-id", "lease-1", "--external-reference", "ACC-1")
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	request := api.only(t)
	if request.path != "/billing/handoff/inv-1/ack" || string(request.body) != `{"externalReference":"ACC-1","leaseId":"lease-1"}` {
		t.Fatalf("request = %s %s", request.path, request.body)
	}
	expectOutputContains(t, stdout, "ACKNOWLEDGED")
}

// A lease another claim took answers 409: the script is told the API refused.
func TestBillingHandoffAckOfAStolenLeaseExitsRejected(t *testing.T) {
	server, _ := givenUsageAPI(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
		writeTestJSON(w, http.StatusConflict, `{"title":"Conflict","status":409,"code":"AckHandoff.LeaseMismatch","detail":"another claim holds it"}`)
	})

	if _, _, code := runCapturing(t, "--base-url", server.URL, "billing", "handoff", "ack", "inv-1", "--lease-id", "old"); code != exitRejected {
		t.Fatalf("exit %d, want %d", code, exitRejected)
	}
}

func TestBillingInvoicesListSendsTheFilters(t *testing.T) {
	server, api := givenUsageAPI(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
		writeTestJSON(w, http.StatusOK, `{"items":[{"id":"inv-1","status":"PUSH_FAILED","total":1200,"currency":"eur","customerSlug":"acme"}],"hasMore":false}`)
	})

	stdout, stderr, code := runCapturing(t, "--base-url", server.URL, "billing", "invoices", "list",
		"--status", "push_failed", "--status", "PAYMENT_FAILED", "--overdue", "--customer", "acme", "--updated-since", "2027-01-01T00:00:00Z")
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	query := api.only(t).query
	if strings.Join(query["status"], ",") != "PUSH_FAILED,PAYMENT_FAILED" || query.Get("overdue") != "true" ||
		query.Get("customerSlug") != "acme" || query.Get("updatedSince") != "2027-01-01T00:00:00Z" || query.Get("held") != "" {
		t.Fatalf("query = %s", query.Encode())
	}
	expectOutputContains(t, stdout, "1200 EUR")
}

func TestBillingInvoicesVoidNeedsAReasonAndConfirmation(t *testing.T) {
	server, api := givenUsageAPI(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
		writeTestJSON(w, http.StatusOK, `{"id":"inv-1","status":"VOID"}`)
	})

	if _, _, code := runCapturing(t, "--base-url", server.URL, "billing", "invoices", "void", "inv-1", "--yes"); code != exitUsage {
		t.Fatalf("without --reason: exit %d, want %d", code, exitUsage)
	}
	if _, _, code := runCapturing(t, "--base-url", server.URL, "billing", "invoices", "void", "inv-1", "--reason", "duplicate"); code != exitUsage {
		t.Fatalf("without --yes on a non-interactive stdin: exit %d, want %d", code, exitUsage)
	}
	if api.count() != 0 {
		t.Fatalf("requests = %d before confirmation, want none", api.count())
	}

	_, stderr, code := runCapturing(t, "--base-url", server.URL, "billing", "invoices", "void", "inv-1", "--reason", "duplicate", "--yes")
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if request := api.only(t); request.path != "/invoices/inv-1/void" || string(request.body) != `{"reason":"duplicate"}` {
		t.Fatalf("request = %s %s", request.path, request.body)
	}
}

func TestBillingInvoicesMarkPaidOmitsWhatWasNotGiven(t *testing.T) {
	server, api := givenUsageAPI(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
		writeTestJSON(w, http.StatusOK, `{"id":"inv-1","status":"PAID"}`)
	})

	_, stderr, code := runCapturing(t, "--base-url", server.URL, "billing", "invoices", "mark-paid", "inv-1", "--external-reference", "WIRE-9")
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if body := string(api.only(t).body); body != `{"externalReference":"WIRE-9"}` {
		t.Fatalf("body = %s", body)
	}
}

func TestBillingInvoicesExportWritesTheFile(t *testing.T) {
	const csv = "invoiceId,total\ninv-1,1200\n"
	server, api := givenUsageAPI(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
		w.Header().Set("Content-Type", "text/csv")
		_, _ = w.Write([]byte(csv))
	})
	file := filepath.Join(t.TempDir(), "january.csv")

	if _, _, code := runCapturing(t, "--base-url", server.URL, "billing", "invoices", "export", "--from", "2027-01-01T00:00:00Z"); code != exitUsage {
		t.Fatalf("without --to: exit %d, want %d", code, exitUsage)
	}
	_, stderr, code := runCapturing(t, "--base-url", server.URL, "billing", "invoices", "export",
		"--from", "2027-01-01T00:00:00Z", "--to", "2027-02-01T00:00:00Z", "--granularity", "invoice", "--output-file", file)
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	query := api.only(t).query
	if query.Get("format") != "csv" || query.Get("granularity") != "invoice" || query.Get("issuedTo") != "2027-02-01T00:00:00Z" {
		t.Fatalf("query = %s", query.Encode())
	}
	written, err := os.ReadFile(file) //nolint:gosec // the test's own temporary file
	if err != nil || string(written) != csv {
		t.Fatalf("file = %q, %v", written, err)
	}
	if _, err := os.Stat(file + ".partial"); !os.IsNotExist(err) {
		t.Fatalf("the partial file was left behind: %v", err)
	}
}

func TestBillingSubscribeReadsThePayload(t *testing.T) {
	server, api := givenUsageAPI(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
		writeTestJSON(w, http.StatusCreated, `{"subscription":{"status":"ACTIVE"}}`)
	})

	_, stderr, code := runCapturing(t, "--base-url", server.URL, "billing", "subscriptions", "subscribe", "acme-prod",
		"--payload", `{"basePriceId":"price-1","voucherCode":"LAUNCH"}`)
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	request := api.only(t)
	if request.method != http.MethodPost || request.path != "/instances/acme-prod/billing" || string(request.body) != `{"basePriceId":"price-1","voucherCode":"LAUNCH"}` {
		t.Fatalf("request = %s %s %s", request.method, request.path, request.body)
	}
}

// Cancelling at the end of the period needs no body: the API's default mode.
func TestBillingSubscriptionCancelTakesNoPayload(t *testing.T) {
	server, api := givenUsageAPI(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
		writeTestJSON(w, http.StatusOK, `{"status":"ACTIVE"}`)
	})

	_, stderr, code := runCapturing(t, "--base-url", server.URL, "billing", "subscriptions", "cancel", "acme-prod", "--yes")
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if request := api.only(t); request.path != "/instances/acme-prod/billing/cancel" || string(request.body) != `{}` {
		t.Fatalf("request = %s %s", request.path, request.body)
	}
}
