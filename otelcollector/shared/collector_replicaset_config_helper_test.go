package shared

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	yaml "gopkg.in/yaml.v2"
)

// tlsTestEnv holds paths and server URL for TLS-based tests.
type tlsTestEnv struct {
	caCertPath     string
	clientCertPath string
	clientKeyPath  string
	serverURL      string
	server         *httptest.Server
}

// setupTLSTestEnv generates a CA, server cert, and client cert, writes them to
// temp files, and starts an HTTPS test server using the server cert.
func setupTLSTestEnv(t *testing.T) *tlsTestEnv {
	t.Helper()
	dir := t.TempDir()

	// Generate CA
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate CA key: %v", err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test CA"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	caCertDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create CA cert: %v", err)
	}
	caCert, _ := x509.ParseCertificate(caCertDER)
	caCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCertDER})

	caCertPath := filepath.Join(dir, "ca.crt")
	if err := os.WriteFile(caCertPath, caCertPEM, 0644); err != nil {
		t.Fatalf("write CA cert: %v", err)
	}

	// Generate server cert signed by CA
	serverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate server key: %v", err)
	}
	serverTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	serverCertDER, err := x509.CreateCertificate(rand.Reader, serverTmpl, caCert, &serverKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create server cert: %v", err)
	}

	// Generate client cert signed by CA
	clientKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate client key: %v", err)
	}
	clientTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject:      pkix.Name{CommonName: "test-client"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	clientCertDER, err := x509.CreateCertificate(rand.Reader, clientTmpl, caCert, &clientKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create client cert: %v", err)
	}

	// Write client cert
	clientCertPath := filepath.Join(dir, "client.crt")
	clientCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientCertDER})
	if err := os.WriteFile(clientCertPath, clientCertPEM, 0644); err != nil {
		t.Fatalf("write client cert: %v", err)
	}

	// Write client key
	clientKeyPath := filepath.Join(dir, "client.key")
	clientKeyDER, err := x509.MarshalECPrivateKey(clientKey)
	if err != nil {
		t.Fatalf("marshal client key: %v", err)
	}
	clientKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: clientKeyDER})
	if err := os.WriteFile(clientKeyPath, clientKeyPEM, 0644); err != nil {
		t.Fatalf("write client key: %v", err)
	}

	// Start HTTPS server with server cert signed by our CA
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	server.TLS = &tls.Config{
		Certificates: []tls.Certificate{{
			Certificate: [][]byte{serverCertDER},
			PrivateKey:  serverKey,
		}},
	}
	server.StartTLS()
	t.Cleanup(server.Close)

	return &tlsTestEnv{
		caCertPath:     caCertPath,
		clientCertPath: clientCertPath,
		clientKeyPath:  clientKeyPath,
		serverURL:      server.URL + "/scrape_configs",
		server:         server,
	}
}

// createTestCollectorConfig writes a minimal OtelConfig YAML to a temp file.
func createTestCollectorConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "collector-config.yml")
	config := OtelConfig{}
	config.Receivers.Prometheus.TargetAllocator = map[string]interface{}{
		"endpoint": "https://ama-metrics-operator-targets.kube-system.svc.cluster.local",
		"tls": map[string]interface{}{
			"ca_file": "/etc/operator-targets/client/certs/ca.crt",
		},
	}
	data, err := yaml.Marshal(config)
	if err != nil {
		t.Fatalf("marshal test config: %v", err)
	}
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("write test config: %v", err)
	}
	return configPath
}

func TestCollectorTAHttpsCheck_CACertMissing(t *testing.T) {
	configPath := createTestCollectorConfig(t)
	os.Unsetenv("COLLECTOR_CONFIG_WITH_HTTPS")
	os.Unsetenv("COLLECTOR_CONFIG_HTTPS_REMOVED")

	cfg := httpsCheckConfig{
		caCertPath:      "/nonexistent/path/ca.crt",
		clientCertPath:  "/nonexistent/path/client.crt",
		clientKeyPath:   "/nonexistent/path/client.key",
		taEndpoint:      "https://127.0.0.1:1/scrape_configs",
		maxRetries:      2,
		certRetryDelay:  time.Millisecond,
		httpsRetryDelay: time.Millisecond,
	}

	err := collectorTAHttpsCheckWithConfig(cfg, configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have fallen back to HTTP — HTTPS env var should NOT be set
	if os.Getenv("COLLECTOR_CONFIG_WITH_HTTPS") == "true" {
		t.Error("expected COLLECTOR_CONFIG_WITH_HTTPS to not be 'true' when CA cert is missing")
	}

	// RemoveHTTPSSettingsInCollectorConfig should have set this
	if os.Getenv("COLLECTOR_CONFIG_HTTPS_REMOVED") != "true" {
		t.Error("expected COLLECTOR_CONFIG_HTTPS_REMOVED to be 'true' after fallback")
	}
}

func TestCollectorTAHttpsCheck_InvalidCACert(t *testing.T) {
	dir := t.TempDir()
	configPath := createTestCollectorConfig(t)
	os.Unsetenv("COLLECTOR_CONFIG_WITH_HTTPS")
	os.Unsetenv("COLLECTOR_CONFIG_HTTPS_REMOVED")

	// Write garbage data as CA cert
	caCertPath := filepath.Join(dir, "ca.crt")
	if err := os.WriteFile(caCertPath, []byte("not a valid cert"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := httpsCheckConfig{
		caCertPath:      caCertPath,
		clientCertPath:  "/nonexistent/client.crt",
		clientKeyPath:   "/nonexistent/client.key",
		taEndpoint:      "https://127.0.0.1:1/scrape_configs",
		maxRetries:      2,
		certRetryDelay:  time.Millisecond,
		httpsRetryDelay: time.Millisecond,
	}

	err := collectorTAHttpsCheckWithConfig(cfg, configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if os.Getenv("COLLECTOR_CONFIG_WITH_HTTPS") == "true" {
		t.Error("expected COLLECTOR_CONFIG_WITH_HTTPS to not be 'true' with invalid CA cert")
	}
	if os.Getenv("COLLECTOR_CONFIG_HTTPS_REMOVED") != "true" {
		t.Error("expected COLLECTOR_CONFIG_HTTPS_REMOVED to be 'true' after fallback")
	}
}

func TestCollectorTAHttpsCheck_ClientCertMissing(t *testing.T) {
	env := setupTLSTestEnv(t)
	configPath := createTestCollectorConfig(t)
	os.Unsetenv("COLLECTOR_CONFIG_WITH_HTTPS")
	os.Unsetenv("COLLECTOR_CONFIG_HTTPS_REMOVED")

	cfg := httpsCheckConfig{
		caCertPath:      env.caCertPath,
		clientCertPath:  "/nonexistent/client.crt",
		clientKeyPath:   "/nonexistent/client.key",
		taEndpoint:      env.serverURL,
		maxRetries:      2,
		certRetryDelay:  time.Millisecond,
		httpsRetryDelay: time.Millisecond,
	}

	err := collectorTAHttpsCheckWithConfig(cfg, configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if os.Getenv("COLLECTOR_CONFIG_WITH_HTTPS") == "true" {
		t.Error("expected COLLECTOR_CONFIG_WITH_HTTPS to not be 'true' when client certs are missing")
	}
	if os.Getenv("COLLECTOR_CONFIG_HTTPS_REMOVED") != "true" {
		t.Error("expected COLLECTOR_CONFIG_HTTPS_REMOVED to be 'true' after fallback")
	}
}

func TestCollectorTAHttpsCheck_HTTPSSuccess(t *testing.T) {
	env := setupTLSTestEnv(t)
	configPath := createTestCollectorConfig(t)
	os.Unsetenv("COLLECTOR_CONFIG_WITH_HTTPS")
	os.Unsetenv("COLLECTOR_CONFIG_HTTPS_REMOVED")

	cfg := httpsCheckConfig{
		caCertPath:      env.caCertPath,
		clientCertPath:  env.clientCertPath,
		clientKeyPath:   env.clientKeyPath,
		taEndpoint:      env.serverURL,
		maxRetries:      3,
		certRetryDelay:  time.Millisecond,
		httpsRetryDelay: time.Millisecond,
	}

	err := collectorTAHttpsCheckWithConfig(cfg, configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if os.Getenv("COLLECTOR_CONFIG_WITH_HTTPS") != "true" {
		t.Error("expected COLLECTOR_CONFIG_WITH_HTTPS to be 'true' on successful HTTPS check")
	}
}

func TestCollectorTAHttpsCheck_HTTPSEndpointUnreachable(t *testing.T) {
	env := setupTLSTestEnv(t)
	configPath := createTestCollectorConfig(t)
	os.Unsetenv("COLLECTOR_CONFIG_WITH_HTTPS")
	os.Unsetenv("COLLECTOR_CONFIG_HTTPS_REMOVED")

	// Close the server so the endpoint is unreachable
	env.server.Close()

	cfg := httpsCheckConfig{
		caCertPath:      env.caCertPath,
		clientCertPath:  env.clientCertPath,
		clientKeyPath:   env.clientKeyPath,
		taEndpoint:      env.serverURL,
		maxRetries:      2,
		certRetryDelay:  time.Millisecond,
		httpsRetryDelay: time.Millisecond,
	}

	err := collectorTAHttpsCheckWithConfig(cfg, configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if os.Getenv("COLLECTOR_CONFIG_WITH_HTTPS") == "true" {
		t.Error("expected COLLECTOR_CONFIG_WITH_HTTPS to not be 'true' when endpoint is unreachable")
	}
	if os.Getenv("COLLECTOR_CONFIG_HTTPS_REMOVED") != "true" {
		t.Error("expected COLLECTOR_CONFIG_HTTPS_REMOVED to be 'true' after fallback")
	}
}

func TestCollectorTAHttpsCheck_HTTPSEndpointReturns500(t *testing.T) {
	dir := t.TempDir()
	configPath := createTestCollectorConfig(t)
	os.Unsetenv("COLLECTOR_CONFIG_WITH_HTTPS")
	os.Unsetenv("COLLECTOR_CONFIG_HTTPS_REMOVED")

	// Generate CA
	caKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test CA"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	caCertDER, _ := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	caCert, _ := x509.ParseCertificate(caCertDER)
	caCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCertDER})

	caCertPath := filepath.Join(dir, "ca.crt")
	os.WriteFile(caCertPath, caCertPEM, 0644)

	// Server cert
	serverKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	serverTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	serverCertDER, _ := x509.CreateCertificate(rand.Reader, serverTmpl, caCert, &serverKey.PublicKey, caKey)

	// Client cert
	clientKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	clientTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject:      pkix.Name{CommonName: "test-client"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	clientCertDER, _ := x509.CreateCertificate(rand.Reader, clientTmpl, caCert, &clientKey.PublicKey, caKey)
	clientCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientCertDER})
	clientCertPath := filepath.Join(dir, "client.crt")
	os.WriteFile(clientCertPath, clientCertPEM, 0644)

	clientKeyDER, _ := x509.MarshalECPrivateKey(clientKey)
	clientKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: clientKeyDER})
	clientKeyPath := filepath.Join(dir, "client.key")
	os.WriteFile(clientKeyPath, clientKeyPEM, 0644)

	// Server that always returns 500
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	server.TLS = &tls.Config{
		Certificates: []tls.Certificate{{
			Certificate: [][]byte{serverCertDER},
			PrivateKey:  serverKey,
		}},
	}
	server.StartTLS()
	t.Cleanup(server.Close)

	cfg := httpsCheckConfig{
		caCertPath:      caCertPath,
		clientCertPath:  clientCertPath,
		clientKeyPath:   clientKeyPath,
		taEndpoint:      server.URL + "/scrape_configs",
		maxRetries:      2,
		certRetryDelay:  time.Millisecond,
		httpsRetryDelay: time.Millisecond,
	}

	err := collectorTAHttpsCheckWithConfig(cfg, configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if os.Getenv("COLLECTOR_CONFIG_WITH_HTTPS") == "true" {
		t.Error("expected COLLECTOR_CONFIG_WITH_HTTPS to not be 'true' when endpoint returns 500")
	}
	if os.Getenv("COLLECTOR_CONFIG_HTTPS_REMOVED") != "true" {
		t.Error("expected COLLECTOR_CONFIG_HTTPS_REMOVED to be 'true' after fallback")
	}
}

func TestRemoveHTTPSSettingsInCollectorConfig(t *testing.T) {
	os.Unsetenv("COLLECTOR_CONFIG_HTTPS_REMOVED")

	configPath := createTestCollectorConfig(t)

	err := RemoveHTTPSSettingsInCollectorConfig(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Read back and verify TLS settings were removed
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var config OtelConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	ta := config.Receivers.Prometheus.TargetAllocator
	if ta["tls"] != nil {
		t.Error("expected tls settings to be removed from target_allocator config")
	}
	if ta["endpoint"] != "http://ama-metrics-operator-targets.kube-system.svc.cluster.local" {
		t.Errorf("expected HTTP endpoint, got %v", ta["endpoint"])
	}

	if os.Getenv("COLLECTOR_CONFIG_HTTPS_REMOVED") != "true" {
		t.Error("expected COLLECTOR_CONFIG_HTTPS_REMOVED to be 'true'")
	}
}
