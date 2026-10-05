package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

const usageBody = `{"entitlementId":"e","entitlementSlug":"tokens","licenseId":"l","value":{"type":"number","value":1200,"event_count":1},"limit":{"type":"number","value":5000}}`

// usageAPI is a fake Core API for the usage commands. It records each request
// and answers with what the test configured.
type usageAPI struct {
	mu       sync.Mutex
	requests []recordedUsageRequest
	answer   func(w http.ResponseWriter, r *http.Request, body []byte)
}

type recordedUsageRequest struct {
	method, path string
	query        url.Values
	body         []byte
}

func givenUsageAPI(t *testing.T, answer func(w http.ResponseWriter, r *http.Request, body []byte)) (*httptest.Server, *usageAPI) {
	t.Helper()
	givenIsolatedConfig(t)
	api := &usageAPI{answer: answer}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		api.mu.Lock()
		api.requests = append(api.requests, recordedUsageRequest{r.Method, r.URL.Path, r.URL.Query(), body})
		api.mu.Unlock()
		api.answer(w, r, body)
	}))
	t.Cleanup(server.Close)
	return server, api
}

func (a *usageAPI) only(t *testing.T) recordedUsageRequest {
	t.Helper()
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(a.requests))
	}
	return a.requests[0]
}

func (a *usageAPI) count() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.requests)
}

// runCapturing runs the CLI and returns stdout, stderr and the exit code.
func runCapturing(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	root := NewRootCommand()
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)
	cmd, err := root.ExecuteC()
	return stdout.String(), stderr.String(), exitCode(cmd, err)
}

func TestUsageReportSendsTheTransactionID(t *testing.T) {
	server, api := givenUsageAPI(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
		w.Header().Set("Idempotent-Replayed", "true")
		writeTestJSON(w, http.StatusOK, usageBody)
	})

	stdout, stderr, code := runCapturing(t, "--base-url", server.URL, "--output", "json",
		"instances", "usage", "report", "acme-prod", "tokens", "--value", "1200", "--transaction-id", "llm-call:9f2c:tokens")
	expectExitCode(t, code, exitOK)

	var sent map[string]any
	if err := json.Unmarshal(api.only(t).body, &sent); err != nil {
		t.Fatal(err)
	}
	if sent["transactionId"] != "llm-call:9f2c:tokens" {
		t.Errorf("request body = %s, want the key", api.only(t).body)
	}
	expectStructuredDocument(t, stdout, json.Unmarshal)
	if strings.Contains(stdout, "already counted") {
		t.Error("the replay note belongs on stderr, not in the document scripts read")
	}
	if !strings.Contains(stderr, "transaction llm-call:9f2c:tokens was already counted") {
		t.Errorf("stderr = %q, want the replay note", stderr)
	}
}

func TestUsageReportWithoutAKeySendsNone(t *testing.T) {
	server, api := givenUsageAPI(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
		writeTestJSON(w, http.StatusOK, usageBody)
	})

	_, stderr, code := runCapturing(t, "--base-url", server.URL, "instances", "usage", "report", "acme-prod", "tokens", "--value", "1")
	expectExitCode(t, code, exitOK)
	if strings.Contains(string(api.only(t).body), "transactionId") {
		t.Errorf("request body = %s, want no key", api.only(t).body)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want nothing for a plain report", stderr)
	}
}

func TestUsageReportKeyFailures(t *testing.T) {
	t.Run("a malformed key is a usage error and sends nothing", func(t *testing.T) {
		server, api := givenUsageAPI(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
			writeTestJSON(w, http.StatusOK, usageBody)
		})
		_, _, code := runCapturing(t, "--base-url", server.URL, "instances", "usage", "report", "acme-prod", "tokens", "--value", "1", "--transaction-id", "has space")
		expectExitCode(t, code, exitUsage)
		if api.count() != 0 {
			t.Errorf("requests = %d, want none", api.count())
		}
	})

	t.Run("a reused key is rejected", func(t *testing.T) {
		server, _ := givenUsageAPI(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"title":"Conflict","status":409,"code":"ReportEntitlementUsageMetric.TransactionIdReused","detail":"transactionId evt-1 was already used for another report"}`))
		})
		_, _, code := runCapturing(t, "--base-url", server.URL, "instances", "usage", "report", "acme-prod", "tokens", "--value", "1", "--transaction-id", "evt-1")
		expectExitCode(t, code, exitRejected)
	})
}

func TestUsageHistoryListsTheReports(t *testing.T) {
	server, api := givenUsageAPI(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
		writeTestJSON(w, http.StatusOK, `{"items":[
			{"reportSeq":1,"reportedAt":"2026-10-05T08:00:00Z","behavior":"append","aggregationMethod":"SUM","reportedValue":"600","valueBefore":"0","valueAfter":"600","delta":"600","overageDelta":"0","eventCountAfter":1,"limitValue":"1000","overagePercent":50,"transactionId":"evt-1","instanceId":"11111111-1111-1111-1111-111111111111","entitlementId":"22222222-2222-2222-2222-222222222222","licenseId":"33333333-3333-3333-3333-333333333333"},
			{"reportSeq":2,"reportedAt":"2026-10-05T09:00:00Z","behavior":"set","aggregationMethod":"SUM","reportedValue":"450","valueBefore":"600","valueAfter":"450","delta":"-150","overageDelta":"0","eventCountAfter":2,"instanceId":"11111111-1111-1111-1111-111111111111","entitlementId":"22222222-2222-2222-2222-222222222222","licenseId":"33333333-3333-3333-3333-333333333333"}]}`)
	})

	stdout, _, code := runCapturing(t, "--base-url", server.URL, "instances", "usage", "history", "acme-prod", "tokens", "--from", "2026-10-01T00:00:00Z")
	expectExitCode(t, code, exitOK)

	request := api.only(t)
	if request.path != "/instances/acme-prod/entitlements/tokens/usage/reports" || request.query.Get("from") == "" {
		t.Errorf("request = %s?%s", request.path, request.query.Encode())
	}
	for _, want := range []string{"SEQ", "TRANSACTION ID", "evt-1", "1000", "-150", "set"} {
		expectOutputContains(t, stdout, want)
	}
}

const exportCSV = "organization_id,instance_id,report_seq\no,i,1\no,i,2\n"

func givenExportAPI(t *testing.T) (*httptest.Server, *usageAPI) {
	t.Helper()
	return givenUsageAPI(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="usage-20261001-20261005.csv"`)
		_, _ = w.Write([]byte(exportCSV))
	})
}

func TestUsageExportStreamsAPairToStdout(t *testing.T) {
	server, api := givenExportAPI(t)

	stdout, _, code := runCapturing(t, "--base-url", server.URL, "instances", "usage", "export", "acme-prod", "tokens")
	expectExitCode(t, code, exitOK)
	if stdout != exportCSV {
		t.Errorf("stdout = %q, want the export byte for byte", stdout)
	}
	request := api.only(t)
	if request.path != "/instances/acme-prod/entitlements/tokens/usage/reports/export" || request.query.Get("format") != "csv" {
		t.Errorf("request = %s?%s", request.path, request.query.Encode())
	}
}

func TestUsageExportWritesTheOrganizationToAFile(t *testing.T) {
	server, api := givenExportAPI(t)
	target := filepath.Join(t.TempDir(), "usage.ndjson")

	stdout, stderr, code := runCapturing(t, "--base-url", server.URL, "instances", "usage", "export",
		"--format", "json", "--instance-id", "11111111-1111-1111-1111-111111111111", "--entitlement", "tokens", "--output-file", target)
	expectExitCode(t, code, exitOK)

	written, err := os.ReadFile(target) //nolint:gosec // a path under t.TempDir()
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != exportCSV {
		t.Errorf("file = %q", written)
	}
	if _, err := os.Stat(target + ".partial"); !os.IsNotExist(err) {
		t.Error("the partial file was left behind")
	}
	if stdout != "" || !strings.Contains(stderr, fmt.Sprintf("wrote %d bytes to %s", len(exportCSV), target)) {
		t.Errorf("stdout = %q, stderr = %q", stdout, stderr)
	}

	query := api.only(t).query
	if api.only(t).path != "/usage/reports/export" || query.Get("format") != "json" ||
		query.Get("instanceId") != "11111111-1111-1111-1111-111111111111" || query.Get("entitlementSlug") != "tokens" {
		t.Errorf("request = %s?%s", api.only(t).path, query.Encode())
	}
}

func TestUsageExportInvocationMistakes(t *testing.T) {
	server, api := givenExportAPI(t)

	for name, args := range map[string][]string{
		"one argument":                        {"acme-prod"},
		"organization filters on a pair":      {"acme-prod", "tokens", "--instance", "acme-prod"},
		"an unknown format":                   {"--format", "xml"},
		"an unparsable from":                  {"--from", "yesterday"},
		"a malformed instance ID":             {"--instance-id", "not-a-uuid"},
		"organization filter ID on a pair":    {"acme-prod", "tokens", "--entitlement-id", "22222222-2222-2222-2222-222222222222"},
		"three arguments":                     {"a", "b", "c"},
		"an entitlement filter with one slug": {"acme-prod", "--entitlement", "tokens"},
	} {
		t.Run(name, func(t *testing.T) {
			_, _, code := runCapturing(t, append([]string{"--base-url", server.URL, "instances", "usage", "export"}, args...)...)
			if name == "a malformed instance ID" {
				// Caught by the SDK before any request, as an ordinary error.
				if code == exitOK {
					t.Fatalf("exit code = 0, want a failure")
				}
				return
			}
			expectExitCode(t, code, exitUsage)
		})
	}
	if api.count() != 0 {
		t.Errorf("requests = %d, want none", api.count())
	}
}

func TestUsageExportReportsTheAPIRefusal(t *testing.T) {
	server, _ := givenUsageAPI(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"title":"Unprocessable Entity","status":422,"code":"ExportUsageReports.OutsideRetention","detail":"the usage history is kept from 2026-07-05T12:00:00.000Z"}`))
	})
	target := filepath.Join(t.TempDir(), "usage.csv")

	_, _, code := runCapturing(t, "--base-url", server.URL, "instances", "usage", "export", "acme-prod", "tokens", "--from", "2025-01-01T00:00:00Z", "--output-file", target)
	expectExitCode(t, code, exitRejected)
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Error("a refused export created the output file")
	}
}
