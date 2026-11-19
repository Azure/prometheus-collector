package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTAHealthCheckUnreachable verifies that taHealthCheck reports 503 when the
// targetallocator endpoint is unreachable. This guards against net/http's
// implicit-200 behavior, which previously let the liveness probe (and e2e
// checks) pass even when the targetallocator was down.
func TestTAHealthCheckUnreachable(t *testing.T) {
	// Bind a server to obtain a free port, then close it so the address refuses
	// connections deterministically.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()

	status, _ := taHealthCheck(url + "/metrics")
	if status != http.StatusServiceUnavailable {
		t.Fatalf("expected %d when targetallocator is unreachable, got %d", http.StatusServiceUnavailable, status)
	}
}

// TestTAHealthCheckHealthy verifies that taHealthCheck reports 200 when the
// targetallocator endpoint responds with 200.
func TestTAHealthCheckHealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	status, _ := taHealthCheck(srv.URL + "/metrics")
	if status != http.StatusOK {
		t.Fatalf("expected %d when targetallocator is healthy, got %d", http.StatusOK, status)
	}
}

// TestTAHealthCheckNon200 verifies that a non-200 response from the
// targetallocator is treated as unhealthy.
func TestTAHealthCheckNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	status, _ := taHealthCheck(srv.URL + "/metrics")
	if status != http.StatusServiceUnavailable {
		t.Fatalf("expected %d when targetallocator returns 500, got %d", http.StatusServiceUnavailable, status)
	}
}

func runUpdateTAConfig(t *testing.T, denyEnv string, httpsEnv string, httpsEnabled bool) string {
	t.Helper()
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "merged.yaml")
	input := "receivers:\n  prometheus:\n    config:\n      scrape_configs: []\n"
	if err := os.WriteFile(inputPath, []byte(input), 0644); err != nil {
		t.Fatal(err)
	}

	origPath := taConfigFilePath
	taConfigFilePath = filepath.Join(dir, "targetallocator.yaml")
	t.Cleanup(func() { taConfigFilePath = origPath })

	t.Setenv("AZMON_DENY_FS_ACCESS_THROUGH_SMS", denyEnv)
	t.Setenv("AZMON_OPERATOR_HTTPS_ENABLED", httpsEnv)
	t.Setenv("KUBE_VERSION", "")

	updateTAConfigFile(inputPath, httpsEnabled)

	out, err := os.ReadFile(taConfigFilePath)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// TestUpdateTAConfigFileDenyFSAccess verifies deny_fs_access_through_sms is only
// written to targetallocator.yaml when the configmap setting exported it.
func TestUpdateTAConfigFileDenyFSAccess(t *testing.T) {
	tests := []struct {
		name         string
		denyEnv      string
		httpsEnv     string
		httpsEnabled bool
		want         bool
	}{
		{"disabled by default, no https", "", "false", false, false},
		{"disabled by default, https", "", "true", true, false},
		{"explicit false, no https", "false", "false", false, false},
		{"enabled, no https", "true", "false", false, true},
		{"enabled, https", "true", "true", true, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := runUpdateTAConfig(t, tc.denyEnv, tc.httpsEnv, tc.httpsEnabled)
			if got := strings.Contains(out, "deny_fs_access_through_sms: true"); got != tc.want {
				t.Fatalf("deny_fs_access_through_sms present = %t, want %t\n%s", got, tc.want, out)
			}
		})
	}
}