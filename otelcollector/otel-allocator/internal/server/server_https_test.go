// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package server

// These tests exercise the runtime behavior that the MSRC 122861 hardening relies on:
//   - the HTTPS server (port 8443 in the chart) enforces mutual TLS, and
//   - the plaintext HTTP server (bound to 127.0.0.1:8080 via --listen-addr when HTTPS is
//     enabled) still serves the in-pod config-reader sidecar but never exposes secrets.
// They stand up the real dual-listener server the deployment configures, rather than only
// exercising the handlers in-memory.

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	commoncfg "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	promconfig "github.com/prometheus/prometheus/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"

	allocatorconfig "github.com/open-telemetry/opentelemetry-operator/cmd/otel-allocator/internal/config"
)

const testTASecret = "P@$$w0rd1!?"

// mtlsTestPKI holds a CA-signed server cert (written to disk for the server config) plus a
// CA-signed client cert/key pair and the CA pool used by the test HTTP client.
type mtlsTestPKI struct {
	caPath, certPath, keyPath string
	clientCert                tls.Certificate
	caPool                    *x509.CertPool
}

// newMTLSTestPKI creates a small PKI: a CA that signs both a server certificate (with a
// 127.0.0.1 SAN) and a client certificate. This mirrors the collector<->target-allocator
// mTLS trust the chart wires up via the shared CA secret.
func newMTLSTestPKI(t *testing.T) mtlsTestPKI {
	t.Helper()
	dir := t.TempDir()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "ama-metrics-test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	require.NoError(t, err)
	caCert, err := x509.ParseCertificate(caDER)
	require.NoError(t, err)
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})

	serverCertPEM, serverKeyPEM := issueCert(t, caCert, caKey, &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:     []string{"localhost"},
	})

	clientCertPEM, clientKeyPEM := issueCert(t, caCert, caKey, &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject:      pkix.Name{CommonName: "ama-metrics-collector"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	})

	clientCert, err := tls.X509KeyPair(clientCertPEM, clientKeyPEM)
	require.NoError(t, err)

	caPool := x509.NewCertPool()
	require.True(t, caPool.AppendCertsFromPEM(caPEM))

	pki := mtlsTestPKI{
		caPath:     filepath.Join(dir, "ca.crt"),
		certPath:   filepath.Join(dir, "tls.crt"),
		keyPath:    filepath.Join(dir, "tls.key"),
		clientCert: clientCert,
		caPool:     caPool,
	}
	require.NoError(t, os.WriteFile(pki.caPath, caPEM, 0o600))
	require.NoError(t, os.WriteFile(pki.certPath, serverCertPEM, 0o600))
	require.NoError(t, os.WriteFile(pki.keyPath, serverKeyPEM, 0o600))
	return pki
}

// issueCert signs tmpl with the given CA and returns the cert and key in PEM form.
func issueCert(t *testing.T, caCert *x509.Certificate, caKey *ecdsa.PrivateKey, tmpl *x509.Certificate) (certPEM, keyPEM []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, caKey)
	require.NoError(t, err)
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return certPEM, keyPEM
}

// scrapeConfigWithSecret builds a single scrape config carrying a basic-auth secret so the
// tests can assert whether the secret is revealed (mTLS) or redacted (plaintext).
func scrapeConfigWithSecret(password string) map[string]*promconfig.ScrapeConfig {
	return map[string]*promconfig.ScrapeConfig{
		"serviceMonitor/testapp/testapp/0": {
			JobName:         "serviceMonitor/testapp/testapp/0",
			HonorTimestamps: true,
			ScrapeInterval:  model.Duration(30 * time.Second),
			ScrapeTimeout:   model.Duration(30 * time.Second),
			MetricsPath:     "/metrics",
			Scheme:          "http",
			HTTPClientConfig: commoncfg.HTTPClientConfig{
				FollowRedirects: true,
				BasicAuth: &commoncfg.BasicAuth{
					Username: "test",
					Password: commoncfg.Secret(password),
				},
			},
		},
	}
}

// newDualListenerServer builds the same server topology the deployment runs when
// OperatorTargetsHttpsEnabled=true: a plaintext HTTP server on loopback plus an additional
// mTLS HTTPS server. Both are served on ephemeral 127.0.0.1 ports and cleaned up by the test.
func newDualListenerServer(t *testing.T, pki mtlsTestPKI, scrapeConfigs map[string]*promconfig.ScrapeConfig) (plainAddr, httpsAddr string) {
	t.Helper()

	httpsCfg := allocatorconfig.HTTPSServerConfig{
		Enabled:         true,
		CAFilePath:      pki.caPath,
		TLSCertFilePath: pki.certPath,
		TLSKeyFilePath:  pki.keyPath,
	}
	tlsConfig, _, err := httpsCfg.NewTLSConfig(logger)
	require.NoError(t, err)

	// Plaintext listen addr bound to loopback == --listen-addr=127.0.0.1:8080 in the chart.
	s, err := NewServer(logger, nil, "127.0.0.1:0", WithTLSConfig(tlsConfig, "127.0.0.1:0"))
	require.NoError(t, err)
	require.NoError(t, s.UpdateScrapeConfigResponse(scrapeConfigs))

	plainLn, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	httpsLn, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	go func() { _ = s.server.Serve(plainLn) }()
	go func() { _ = s.httpsServer.ServeTLS(httpsLn, "", "") }()

	t.Cleanup(func() {
		_ = s.server.Close()
		_ = s.httpsServer.Close()
	})

	return plainLn.Addr().String(), httpsLn.Addr().String()
}

// TestServer_HTTPS_MutualTLSEnforced verifies the HTTPS listener requires a client
// certificate and only then serves real secret values.
func TestServer_HTTPS_MutualTLSEnforced(t *testing.T) {
	pki := newMTLSTestPKI(t)
	_, httpsAddr := newDualListenerServer(t, pki, scrapeConfigWithSecret(testTASecret))
	url := "https://" + httpsAddr + "/scrape_configs"

	t.Run("valid client certificate is accepted and receives real secrets", func(t *testing.T) {
		client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{
			RootCAs:      pki.caPool,
			Certificates: []tls.Certificate{pki.clientCert},
		}}}
		resp, err := client.Get(url)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		got := decodeScrapeConfigsBody(t, body)
		require.NotEmpty(t, got)
		for _, c := range got {
			require.NotNil(t, c.HTTPClientConfig.BasicAuth)
			assert.Equal(t, commoncfg.Secret(testTASecret), c.HTTPClientConfig.BasicAuth.Password,
				"mTLS client must receive the real secret value")
		}
	})

	t.Run("client without a certificate is rejected", func(t *testing.T) {
		client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{
			RootCAs: pki.caPool, // deliberately presents no client certificate
		}}}
		resp, err := client.Get(url)
		if resp != nil {
			resp.Body.Close()
		}
		require.Error(t, err, "server must reject a client that presents no certificate")
	})

	t.Run("plaintext HTTP to the HTTPS port is refused and never leaks secrets", func(t *testing.T) {
		// Go's TLS server answers a plaintext request with 400 Bad Request and never runs
		// the handler, so no scrape config (and no secret) is served over the mTLS port.
		resp, err := http.Get("http://" + httpsAddr + "/scrape_configs")
		if err != nil {
			return // a transport-level failure is an acceptable outcome too
		}
		defer resp.Body.Close()
		body, readErr := io.ReadAll(resp.Body)
		require.NoError(t, readErr)
		assert.NotEqual(t, http.StatusOK, resp.StatusCode,
			"the mTLS port must not serve scrape configs over plaintext HTTP")
		assert.NotContains(t, string(body), testTASecret,
			"plaintext HTTP to the mTLS port must never leak secrets")
	})
}

// TestServer_PlaintextListener_LoopbackAndRedactsSecrets verifies that with HTTPS enabled the
// plaintext listener (the sidecar path) stays on loopback and never serves secret values.
func TestServer_PlaintextListener_LoopbackAndRedactsSecrets(t *testing.T) {
	pki := newMTLSTestPKI(t)
	plainAddr, _ := newDualListenerServer(t, pki, scrapeConfigWithSecret(testTASecret))

	assert.True(t, strings.HasPrefix(plainAddr, "127.0.0.1:"),
		"plaintext listener should be loopback-bound (got %s)", plainAddr)

	resp, err := http.Get("http://" + plainAddr + "/scrape_configs")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	// The sidecar can reach it over plain HTTP, but secrets must remain redacted there.
	got := decodeScrapeConfigsBody(t, body)
	require.NotEmpty(t, got)
	for _, c := range got {
		require.NotNil(t, c.HTTPClientConfig.BasicAuth)
		assert.Equal(t, commoncfg.Secret("<secret>"), c.HTTPClientConfig.BasicAuth.Password,
			"plaintext listener must redact secrets")
		assert.NotEqual(t, commoncfg.Secret(testTASecret), c.HTTPClientConfig.BasicAuth.Password)
	}
}

// decodeScrapeConfigsBody parses a /scrape_configs JSON response body into scrape configs.
// JSON is valid YAML, so yaml.Unmarshal both decodes it and undoes the HTML escaping gin
// applies to characters like '<' and '>', letting tests compare secret values directly.
func decodeScrapeConfigsBody(t *testing.T, body []byte) map[string]*promconfig.ScrapeConfig {
	t.Helper()
	out := map[string]*promconfig.ScrapeConfig{}
	require.NoError(t, yaml.Unmarshal(body, out))
	return out
}
