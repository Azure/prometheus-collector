package main

import (
	"net/http"
	"net/http/httptest"
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
