package main

import (
	"bytes"
	"testing"
	"time"
)

// TestCreateTLSCertificatesAndSecretReusesValidStoredMaterial is the behavioural regression test
// for 979 run 132116: a config-reader restart (settings hot-reload) must not regenerate and
// republish the operator-targets TLS material when valid material is already stored, because
// republishing the client secret restarts every replicaset collector a second time through its
// /etc/operator-targets/client/certs watch.
//
// The regeneration path writes the secrets through rest.InClusterConfig. The test clears
// KUBERNETES_SERVICE_HOST so that any attempt to regenerate and publish fails with
// rest.ErrNotInCluster instead of reaching a cluster. Without the reuse step in
// createTLSCertificatesAndSecret this test therefore fails: secret publication errors are returned
// and no material is recorded as active.
func TestCreateTLSCertificatesAndSecretReusesValidStoredMaterial(t *testing.T) {
	material := newTestTLSMaterial(t)
	serverBefore := copySecretData(material.server)
	clientBefore := copySecretData(material.client)

	savedGetter, savedIssued := newOperatorTargetsSecretGetter, activeTLSIssuedAt
	defer func() {
		newOperatorTargetsSecretGetter, activeTLSIssuedAt = savedGetter, savedIssued
	}()

	getterCalls := 0
	getter := stubSecretGetter{
		operatorTargetsServerSecretName: {Data: material.server},
		operatorTargetsClientSecretName: {Data: material.client},
	}
	newOperatorTargetsSecretGetter = func() (secretGetter, error) {
		getterCalls++
		return getter, nil
	}
	activeTLSIssuedAt = time.Time{}
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	t.Setenv("KUBERNETES_SERVICE_PORT", "")

	caErr, serErr, cliErr, serverSecretErr, clientSecretErr := createTLSCertificatesAndSecret()
	if caErr != nil || serErr != nil || cliErr != nil || serverSecretErr != nil || clientSecretErr != nil {
		t.Fatalf("valid stored TLS material was not reused; regeneration/publication was attempted: ca=%v server=%v client=%v serverSecret=%v clientSecret=%v",
			caErr, serErr, cliErr, serverSecretErr, clientSecretErr)
	}
	if getterCalls != 1 {
		t.Fatalf("expected the stored TLS secrets to be read once, got %d reads", getterCalls)
	}

	wantIssuedAt, err := evaluateReusableOperatorTargetsTLS(serverBefore, clientBefore, time.Now())
	if err != nil {
		t.Fatalf("stored material unexpectedly not reusable: %v", err)
	}
	if !activeTLSIssuedAt.Equal(wantIssuedAt) {
		t.Fatalf("active TLS issuance = %s, want stored material issuance %s", activeTLSIssuedAt, wantIssuedAt)
	}

	for key, value := range serverBefore {
		if !bytes.Equal(getter[operatorTargetsServerSecretName].Data[key], value) {
			t.Fatalf("server secret key %q changed", key)
		}
	}
	for key, value := range clientBefore {
		if !bytes.Equal(getter[operatorTargetsClientSecretName].Data[key], value) {
			t.Fatalf("client secret key %q changed", key)
		}
	}
}

// TestCreateTLSCertificatesAndSecretRegeneratesWhenStoredMaterialIsDue verifies the rotation
// side is preserved: stored material that is not reusable must not be adopted, and the reader
// falls through to regeneration (which cannot publish outside a cluster).
func TestCreateTLSCertificatesAndSecretRegeneratesWhenStoredMaterialIsDue(t *testing.T) {
	first := newTestTLSMaterial(t)
	second := newTestTLSMaterial(t)

	savedGetter, savedIssued := newOperatorTargetsSecretGetter, activeTLSIssuedAt
	defer func() {
		newOperatorTargetsSecretGetter, activeTLSIssuedAt = savedGetter, savedIssued
	}()

	// Mismatched CAs make the stored material non-reusable.
	getter := stubSecretGetter{
		operatorTargetsServerSecretName: {Data: first.server},
		operatorTargetsClientSecretName: {Data: second.client},
	}
	newOperatorTargetsSecretGetter = func() (secretGetter, error) {
		return getter, nil
	}
	activeTLSIssuedAt = time.Time{}
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	t.Setenv("KUBERNETES_SERVICE_PORT", "")

	caErr, serErr, cliErr, serverSecretErr, clientSecretErr := createTLSCertificatesAndSecret()
	if caErr != nil || serErr != nil || cliErr != nil {
		t.Fatalf("certificate generation failed: ca=%v server=%v client=%v", caErr, serErr, cliErr)
	}
	if serverSecretErr == nil {
		t.Fatal("expected regeneration to attempt publishing the server secret (no cluster available)")
	}
	if clientSecretErr == nil {
		t.Fatal("expected regeneration to attempt publishing the client secret (no cluster available)")
	}
	if !activeTLSIssuedAt.IsZero() {
		t.Fatalf("unpublished material must not be recorded as active, got %s", activeTLSIssuedAt)
	}
}
