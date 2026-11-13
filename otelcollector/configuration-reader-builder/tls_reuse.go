package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	operatorTargetsTLSNamespace     = "kube-system"
	operatorTargetsServerSecretName = "ama-metrics-operator-targets-server-tls-secret"
	operatorTargetsClientSecretName = "ama-metrics-operator-targets-client-tls-secret"
	operatorTargetsServiceDNSName   = "ama-metrics-operator-targets.kube-system.svc.cluster.local"

	// tlsRotationAge is the maximum age of the operator-targets TLS material in use. It keeps the
	// existing bound: before reuse was introduced the material was regenerated on every start and
	// the config-reader forced a restart after 5 months, so no certificate was used for longer.
	tlsRotationAge = 5 * 30 * 24 * time.Hour
	// tlsReuseValidityMargin is the minimum validity every certificate must still have at the
	// rotation deadline for existing material to be reused.
	tlsReuseValidityMargin = 7 * 24 * time.Hour
)

// activeTLSIssuedAt is the issuance time of the operator-targets TLS material published by this
// process (reused or newly generated). It is written before the health server starts and is
// zero when no material was published.
var activeTLSIssuedAt = time.Time{}

// secretGetter is the subset of the Kubernetes secrets client needed to read the TLS secrets.
type secretGetter interface {
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*corev1.Secret, error)
}

// newOperatorTargetsSecretGetter returns an in-cluster secrets client for kube-system. It is a
// variable so tests can replace it.
var newOperatorTargetsSecretGetter = func() (secretGetter, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("unable to create in-cluster config: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("unable to create Kubernetes client: %w", err)
	}
	return clientset.CoreV1().Secrets(operatorTargetsTLSNamespace), nil
}

// reuseExistingOperatorTargetsTLS returns the issuance time of the TLS material already stored in
// the server and client secrets when that material can be reused at now. Any error means new
// material must be generated.
func reuseExistingOperatorTargetsTLS(now time.Time) (time.Time, error) {
	getter, err := newOperatorTargetsSecretGetter()
	if err != nil {
		return time.Time{}, err
	}
	return loadReusableOperatorTargetsTLS(context.TODO(), getter, now)
}

func loadReusableOperatorTargetsTLS(ctx context.Context, getter secretGetter, now time.Time) (time.Time, error) {
	serverSecret, err := getter.Get(ctx, operatorTargetsServerSecretName, metav1.GetOptions{})
	if err != nil {
		return time.Time{}, fmt.Errorf("unable to read secret %s: %w", operatorTargetsServerSecretName, err)
	}
	clientSecret, err := getter.Get(ctx, operatorTargetsClientSecretName, metav1.GetOptions{})
	if err != nil {
		return time.Time{}, fmt.Errorf("unable to read secret %s: %w", operatorTargetsClientSecretName, err)
	}
	return evaluateReusableOperatorTargetsTLS(serverSecret.Data, clientSecret.Data, now)
}

// evaluateReusableOperatorTargetsTLS validates the stored TLS material and returns its issuance
// time (the oldest NotBefore). Material is reusable only when both secrets carry the same
// self-signed CA, each leaf chains to it with the expected usage and names, each leaf matches its
// private key, the material is younger than tlsRotationAge, and every certificate stays valid for
// tlsReuseValidityMargin past the rotation deadline.
func evaluateReusableOperatorTargetsTLS(serverData map[string][]byte, clientData map[string][]byte, now time.Time) (time.Time, error) {
	serverCAPem := serverData["ca.crt"]
	clientCAPem := clientData["ca.crt"]
	if len(serverCAPem) == 0 || len(clientCAPem) == 0 {
		return time.Time{}, errors.New("CA certificate missing from TLS secrets")
	}
	if !bytes.Equal(serverCAPem, clientCAPem) {
		return time.Time{}, errors.New("server and client TLS secrets contain different CA certificates")
	}
	caCert, err := parseSingleCertificatePEM(serverCAPem)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid CA certificate: %w", err)
	}
	if !caCert.IsCA || !caCert.BasicConstraintsValid {
		return time.Time{}, errors.New("CA certificate is not a CA")
	}
	if err := caCert.CheckSignatureFrom(caCert); err != nil {
		return time.Time{}, fmt.Errorf("CA certificate is not self-signed: %w", err)
	}

	serverCert, err := parseLeafKeyPair(serverData["server.crt"], serverData["server.key"])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid server certificate: %w", err)
	}
	clientCert, err := parseLeafKeyPair(clientData["client.crt"], clientData["client.key"])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid client certificate: %w", err)
	}

	roots := x509.NewCertPool()
	roots.AddCert(caCert)
	for _, dnsName := range []string{operatorTargetsServiceDNSName, "localhost"} {
		if _, err := serverCert.Verify(x509.VerifyOptions{
			Roots:       roots,
			CurrentTime: now,
			DNSName:     dnsName,
			KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		}); err != nil {
			return time.Time{}, fmt.Errorf("server certificate does not verify for %s: %w", dnsName, err)
		}
	}
	if _, err := clientCert.Verify(x509.VerifyOptions{
		Roots:       roots,
		CurrentTime: now,
		KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}); err != nil {
		return time.Time{}, fmt.Errorf("client certificate does not verify: %w", err)
	}

	certs := []*x509.Certificate{caCert, serverCert, clientCert}
	issuedAt := caCert.NotBefore
	for _, cert := range certs[1:] {
		if cert.NotBefore.Before(issuedAt) {
			issuedAt = cert.NotBefore
		}
	}
	if now.Before(issuedAt) {
		return time.Time{}, fmt.Errorf("TLS material issued in the future (%s)", issuedAt.UTC().Format(time.RFC3339))
	}
	if now.Sub(issuedAt) >= tlsRotationAge {
		return time.Time{}, fmt.Errorf("TLS material issued at %s is due for rotation", issuedAt.UTC().Format(time.RFC3339))
	}
	minNotAfter := issuedAt.Add(tlsRotationAge + tlsReuseValidityMargin)
	for _, cert := range certs {
		if cert.NotAfter.Before(minNotAfter) {
			return time.Time{}, fmt.Errorf("certificate %q expires at %s, before rotation deadline plus margin", cert.Subject.CommonName, cert.NotAfter.UTC().Format(time.RFC3339))
		}
	}
	return issuedAt, nil
}

func parseSingleCertificatePEM(data []byte) (*x509.Certificate, error) {
	block, rest := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, errors.New("no PEM certificate block")
	}
	if len(bytes.TrimSpace(rest)) != 0 {
		return nil, errors.New("unexpected data after PEM certificate block")
	}
	return x509.ParseCertificate(block.Bytes)
}

func parseLeafKeyPair(certPem []byte, keyPem []byte) (*x509.Certificate, error) {
	if len(certPem) == 0 || len(keyPem) == 0 {
		return nil, errors.New("certificate or key missing")
	}
	cert, err := parseSingleCertificatePEM(certPem)
	if err != nil {
		return nil, err
	}
	if _, err := tls.X509KeyPair(certPem, keyPem); err != nil {
		return nil, fmt.Errorf("certificate does not match private key: %w", err)
	}
	return cert, nil
}

// tlsRotationReference returns the time from which the forced-rotation restart is measured: the
// earlier of the container start and the issuance of the TLS material in use.
func tlsRotationReference() time.Time {
	if !activeTLSIssuedAt.IsZero() && activeTLSIssuedAt.Before(cfgReaderContainerStartTime) {
		return activeTLSIssuedAt
	}
	return cfgReaderContainerStartTime
}
