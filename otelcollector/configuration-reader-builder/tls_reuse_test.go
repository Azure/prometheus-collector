package main

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"strings"
	"testing"
	"time"

	certCreator "github.com/prometheus-collector/certcreator"
	certGenerator "github.com/prometheus-collector/certgenerator"
	certOperator "github.com/prometheus-collector/certoperator"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type testTLSMaterial struct {
	server map[string][]byte
	client map[string][]byte
	caCert *x509.Certificate
	caKey  *rsa.PrivateKey
	co     certOperator.CertOperator
}

// newTestTLSMaterial issues material through the same functions the config-reader uses.
func newTestTLSMaterial(t *testing.T) testTLSMaterial {
	t.Helper()
	co := certOperator.NewCertOperator(certGenerator.NewCertGenerator(certCreator.NewCertCreator()))
	caCert, caCertPem, caKey, _, err := createCACertificate(co)
	if err != nil {
		t.Fatalf("createCACertificate: %v", err)
	}
	serverCertPem, serverKeyPem, err := createServerCertificate(co, caCert, caKey)
	if err != nil {
		t.Fatalf("createServerCertificate: %v", err)
	}
	clientCertPem, clientKeyPem, err := createClientCertificate(co, caCert, caKey)
	if err != nil {
		t.Fatalf("createClientCertificate: %v", err)
	}
	return testTLSMaterial{
		server: map[string][]byte{
			"ca.crt":     []byte(caCertPem),
			"server.crt": []byte(serverCertPem),
			"server.key": []byte(serverKeyPem),
		},
		client: map[string][]byte{
			"ca.crt":     []byte(caCertPem),
			"client.crt": []byte(clientCertPem),
			"client.key": []byte(clientKeyPem),
		},
		caCert: caCert,
		caKey:  caKey,
		co:     co,
	}
}

func copySecretData(data map[string][]byte) map[string][]byte {
	out := make(map[string][]byte, len(data))
	for key, value := range data {
		out[key] = append([]byte(nil), value...)
	}
	return out
}

func expectReuseRejected(t *testing.T, server map[string][]byte, client map[string][]byte, now time.Time, want string) {
	t.Helper()
	_, err := evaluateReusableOperatorTargetsTLS(server, client, now)
	if err == nil {
		t.Fatalf("expected reuse to be rejected with %q, got nil", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("expected error containing %q, got %v", want, err)
	}
}

func TestEvaluateReusableTLSAcceptsGeneratedMaterial(t *testing.T) {
	material := newTestTLSMaterial(t)
	now := time.Now()
	issuedAt, err := evaluateReusableOperatorTargetsTLS(material.server, material.client, now)
	if err != nil {
		t.Fatalf("expected freshly generated material to be reusable: %v", err)
	}
	if now.Sub(issuedAt) < 0 || now.Sub(issuedAt) > time.Minute {
		t.Fatalf("unexpected issuance time %s for now %s", issuedAt, now)
	}
}

func TestEvaluateReusableTLSRejectsMaterialDueForRotation(t *testing.T) {
	material := newTestTLSMaterial(t)
	expectReuseRejected(t, material.server, material.client, time.Now().Add(tlsRotationAge), "due for rotation")
}

func TestEvaluateReusableTLSRejectsDifferentCAs(t *testing.T) {
	first := newTestTLSMaterial(t)
	second := newTestTLSMaterial(t)
	expectReuseRejected(t, first.server, second.client, time.Now(), "different CA")
}

func TestEvaluateReusableTLSRejectsLeafFromOtherCA(t *testing.T) {
	first := newTestTLSMaterial(t)
	second := newTestTLSMaterial(t)
	client := copySecretData(first.client)
	client["client.crt"] = second.client["client.crt"]
	client["client.key"] = second.client["client.key"]
	expectReuseRejected(t, first.server, client, time.Now(), "client certificate does not verify")
}

func TestEvaluateReusableTLSRejectsMismatchedKey(t *testing.T) {
	material := newTestTLSMaterial(t)
	server := copySecretData(material.server)
	server["server.key"] = material.client["client.key"]
	expectReuseRejected(t, server, material.client, time.Now(), "does not match private key")
}

func TestEvaluateReusableTLSRejectsClientCertificateAsServer(t *testing.T) {
	material := newTestTLSMaterial(t)
	server := copySecretData(material.server)
	server["server.crt"] = material.client["client.crt"]
	server["server.key"] = material.client["client.key"]
	expectReuseRejected(t, server, material.client, time.Now(), "server certificate does not verify")
}

func TestEvaluateReusableTLSRejectsMissingData(t *testing.T) {
	material := newTestTLSMaterial(t)
	client := copySecretData(material.client)
	delete(client, "client.key")
	expectReuseRejected(t, material.server, client, time.Now(), "certificate or key missing")

	server := copySecretData(material.server)
	delete(server, "ca.crt")
	expectReuseRejected(t, server, material.client, time.Now(), "CA certificate missing")
}

func TestEvaluateReusableTLSRejectsInsufficientValidity(t *testing.T) {
	material := newTestTLSMaterial(t)
	now := time.Now()
	csr := &x509.Certificate{
		NotBefore:             now,
		NotAfter:              now.Add(tlsRotationAge),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		Subject:               pkix.Name{CommonName: "ama-metrics"},
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	clientCertPem, clientKeyPem, err := material.co.CreateCertificateKeyPair(csr, material.caCert, material.caKey)
	if err != nil {
		t.Fatalf("CreateCertificateKeyPair: %v", err)
	}
	client := copySecretData(material.client)
	client["client.crt"] = []byte(clientCertPem)
	client["client.key"] = []byte(clientKeyPem)
	expectReuseRejected(t, material.server, client, now, "before rotation deadline plus margin")
}

type stubSecretGetter map[string]*corev1.Secret

func (s stubSecretGetter) Get(_ context.Context, name string, _ metav1.GetOptions) (*corev1.Secret, error) {
	if secret, ok := s[name]; ok {
		return secret, nil
	}
	return nil, errors.New("not found")
}

func TestLoadReusableTLSReadsBothSecrets(t *testing.T) {
	material := newTestTLSMaterial(t)
	getter := stubSecretGetter{
		operatorTargetsServerSecretName: {Data: material.server},
		operatorTargetsClientSecretName: {Data: material.client},
	}
	if _, err := loadReusableOperatorTargetsTLS(context.Background(), getter, time.Now()); err != nil {
		t.Fatalf("expected stored material to be reusable: %v", err)
	}

	delete(getter, operatorTargetsClientSecretName)
	if _, err := loadReusableOperatorTargetsTLS(context.Background(), getter, time.Now()); err == nil {
		t.Fatal("expected missing client secret to prevent reuse")
	}
}

func TestTLSRotationReferenceUsesOlderOfStartAndIssuance(t *testing.T) {
	savedStart, savedIssued := cfgReaderContainerStartTime, activeTLSIssuedAt
	defer func() {
		cfgReaderContainerStartTime, activeTLSIssuedAt = savedStart, savedIssued
	}()

	start := time.Now()
	cfgReaderContainerStartTime = start

	activeTLSIssuedAt = time.Time{}
	if got := tlsRotationReference(); !got.Equal(start) {
		t.Fatalf("no published material: got %s, want container start %s", got, start)
	}

	activeTLSIssuedAt = start.Add(-time.Hour)
	if got := tlsRotationReference(); !got.Equal(activeTLSIssuedAt) {
		t.Fatalf("reused material: got %s, want issuance %s", got, activeTLSIssuedAt)
	}

	activeTLSIssuedAt = start.Add(time.Second)
	if got := tlsRotationReference(); !got.Equal(start) {
		t.Fatalf("newly generated material: got %s, want container start %s", got, start)
	}
}
