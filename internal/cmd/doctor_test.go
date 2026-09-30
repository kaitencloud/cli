package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDoctorReportsConnectivityThroughTheSDKClient(t *testing.T) {
	givenIsolatedConfig(t)
	recorder := &requestRecorder{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder.record(r, nil)
		recorder.recordHeader(r.Header.Get("Authorization"))
		writeTestJSON(w, http.StatusOK, `{"items":[],"nextCursor":null,"hasMore":false}`)
	}))
	defer server.Close()

	stdout := runSuccessfully(t,
		"--base-url", server.URL,
		"--auth-token", "secret-token",
		"--output", "json",
		"doctor",
	)

	_, _, path, _ := recorder.snapshot()
	if path != "/instances" {
		t.Errorf("path = %q, want %q", path, "/instances")
	}
	if got := recorder.header(); got != "Bearer secret-token" {
		t.Errorf("Authorization = %q, want %q", got, "Bearer secret-token")
	}

	var payload struct {
		Status         string `json:"status"`
		InstancesCount int    `json:"instances_count"`
		AuthToken      string `json:"auth_token"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; output = %q", err, stdout)
	}
	if payload.Status != "ok" {
		t.Errorf("Status = %q, want %q", payload.Status, "ok")
	}
	if payload.InstancesCount != 0 {
		t.Errorf("InstancesCount = %d, want 0", payload.InstancesCount)
	}
	// The token is the whole reason doctor exists, and the whole reason it must not print
	// it: an operator pasting doctor output into an issue must not leak their credential.
	if strings.Contains(stdout, "secret-token") {
		t.Errorf("stdout = %q, want the auth token masked", stdout)
	}
	if !strings.HasPrefix(payload.AuthToken, "secr") || !strings.Contains(payload.AuthToken, "*") {
		t.Errorf("AuthToken = %q, want a masked token", payload.AuthToken)
	}
}

func TestDoctorReportsAnUnreachableAPI(t *testing.T) {
	givenIsolatedConfig(t)

	_, err := executeForExitCode(t, "--base-url", "http://127.0.0.1:1", "doctor")

	if err == nil || !strings.Contains(err.Error(), "connectivity check failed") {
		t.Fatalf("Execute() error = %v, want a connectivity failure", err)
	}
}
