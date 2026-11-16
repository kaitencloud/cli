package cmd

import (
	"net/http"
	"strings"
	"testing"
)

func TestDashboardSendsTheRange(t *testing.T) {
	server, api := givenUsageAPI(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
		writeTestJSON(w, http.StatusOK, `{"asOf":"2027-01-01T00:00:00Z","range":{"range":"7d","days":7}}`)
	})

	stdout, stderr, code := runCapturing(t, "--base-url", server.URL, "dashboard", "--range", "7d", "--output", "json")
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	request := api.only(t)
	if !strings.HasSuffix(request.path, "/dashboard/summary") || request.query.Get("range") != "7d" {
		t.Fatalf("request = %s %s?%s", request.method, request.path, request.query.Encode())
	}
	expectOutputContains(t, stdout, `"asOf": "2027-01-01T00:00:00Z"`)
}

func TestDashboardLeavesTheRangeToKaitenByDefault(t *testing.T) {
	server, api := givenUsageAPI(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
		writeTestJSON(w, http.StatusOK, `{"asOf":"2027-01-01T00:00:00Z"}`)
	})

	_, stderr, code := runCapturing(t, "--base-url", server.URL, "dashboard")
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if query := api.only(t).query; query.Has("range") {
		t.Fatalf("query = %s", query.Encode())
	}
}
