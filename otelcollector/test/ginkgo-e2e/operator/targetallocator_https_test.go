package operator

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"prometheus-collector/otelcollector/test/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	taNamespace     = "kube-system"
	taLabelKey      = "rsName"
	taLabelValue    = "ama-metrics-operator-targets"
	taContainerName = "targetallocator"
	taSidecarName   = "config-reader"
	taListenAddrArg = "--listen-addr=127.0.0.1:8080"
)

func targetAllocatorHTTPS(pod corev1.Pod, required bool) (bool, error) {
	enabled, loopback := false, false
	for _, c := range pod.Spec.Containers {
		for _, port := range c.Ports {
			enabled = enabled || (c.Name == taContainerName && port.ContainerPort == 8443)
		}
		for _, mount := range c.VolumeMounts {
			enabled = enabled || mount.MountPath == "/etc/operator-targets/client/certs"
		}
		for _, arg := range c.Args {
			loopback = loopback || (c.Name == taContainerName && arg == taListenAddrArg)
		}
	}
	if required && !enabled {
		return false, fmt.Errorf("HTTPS is required but pod %s has neither the HTTPS port nor client certificate mount", pod.Name)
	}
	if enabled && !loopback {
		return true, fmt.Errorf("HTTPS pod %s is missing %s", pod.Name, taListenAddrArg)
	}
	return enabled, nil
}

func TestTargetAllocatorHTTPS(t *testing.T) {
	pod := corev1.Pod{Spec: corev1.PodSpec{Containers: []corev1.Container{{
		Name:  taContainerName,
		Ports: []corev1.ContainerPort{{ContainerPort: 8443}},
		Args:  []string{taListenAddrArg},
	}}}}
	if enabled, err := targetAllocatorHTTPS(pod, true); !enabled || err != nil {
		t.Fatalf("hardened HTTPS pod: enabled=%t err=%v", enabled, err)
	}
	pod.Spec.Containers[0].Args = nil
	if _, err := targetAllocatorHTTPS(pod, true); err == nil {
		t.Fatal("removing the hardening argument must fail, not skip")
	}
	pod.Spec.Containers[0].Ports = nil
	if _, err := targetAllocatorHTTPS(pod, true); err == nil {
		t.Fatal("required HTTPS must not silently skip")
	}
	if enabled, err := targetAllocatorHTTPS(pod, false); enabled || err != nil {
		t.Fatalf("explicit HTTP compatibility lane: enabled=%t err=%v", enabled, err)
	}
}

var _ = Describe("Target Allocator port hardening (MSRC 122861)", Label(utils.OperatorLabel), func() {

	hardenedTAPod := func() (podName, podIP string) {
		pods, err := utils.GetPodsWithLabel(K8sClient, taNamespace, taLabelKey, taLabelValue)
		Expect(err).NotTo(HaveOccurred())
		Expect(pods).NotTo(BeEmpty(), "expected at least one ama-metrics-operator-targets pod")
		pod := pods[0]

		enabled, err := targetAllocatorHTTPS(pod, os.Getenv("EXPECT_OPERATOR_HTTPS") == "true")
		Expect(err).NotTo(HaveOccurred())
		if !enabled {
			Skip("HTTP compatibility lane: no HTTPS port or client certificate mount")
		}
		Expect(pod.Status.PodIP).NotTo(BeEmpty(), "pod IP should be assigned")
		return pod.Name, pod.Status.PodIP
	}

	curlStatus := func(podName, url string) (string, error) {
		cmd := []string{"curl", "-s", "-o", "/dev/null", "-w", "%{http_code}", "--noproxy", "*", "--max-time", "5", url}
		stdout, _, err := utils.ExecCmd(K8sClient, Cfg, podName, taSidecarName, taNamespace, cmd)
		return strings.TrimSpace(stdout), err
	}

	It("serves the target allocator port 8080 only on localhost", func() {
		podName, podIP := hardenedTAPod()

		// The in-pod sidecar reaches the plaintext listener over loopback.
		status, err := curlStatus(podName, "http://127.0.0.1:8080/scrape_configs")
		Expect(err).NotTo(HaveOccurred(), "port 8080 must be reachable on localhost for the in-pod sidecar")
		Expect(status).To(Equal("200"))

		// Any other pod would connect via the pod IP, which must be refused (MSRC 122861).
		status, err = curlStatus(podName, fmt.Sprintf("http://%s:8080/scrape_configs", podIP))
		Expect(err).To(HaveOccurred(), "port 8080 must NOT be reachable on the pod IP (got HTTP %s)", status)
	})

	It("keeps the liveness probe endpoint reachable on the pod IP in HTTPS mode", func() {
		podName, podIP := hardenedTAPod()

		// The kubelet liveness probe does an httpGet to <pod-ip>:8081/health-ta, so binding the
		// plaintext scrape port to loopback must not have moved the health server off the pod IP.
		status, err := curlStatus(podName, fmt.Sprintf("http://%s:8081/health-ta", podIP))
		Expect(err).NotTo(HaveOccurred(), "the liveness endpoint on :8081 must stay reachable on the pod IP")
		Expect(status).To(Equal("200"))
	})

	It("requires a trusted client certificate for HTTPS scrape configs", func() {
		podName, _ := hardenedTAPod()
		base := []string{"curl", "--silent", "--show-error", "--noproxy", "*", "--max-time", "10",
			"--output", "/dev/null", "--write-out", "%{http_code}",
			"--cacert", "/etc/operator-targets/client/certs/ca.crt"}
		endpoint := "https://ama-metrics-operator-targets.kube-system.svc.cluster.local:443/scrape_configs"
		trusted := append(append([]string{}, base...), "--cert", "/etc/operator-targets/client/certs/client.crt",
			"--key", "/etc/operator-targets/client/certs/client.key", endpoint)
		status, _, err := utils.ExecCmd(K8sClient, Cfg, podName, taSidecarName, taNamespace, trusted)
		Expect(err).NotTo(HaveOccurred(), "trusted mTLS client must reach the allocator")
		Expect(strings.TrimSpace(status)).To(Equal("200"))
		status, stderr, err := utils.ExecCmd(K8sClient, Cfg, podName, taSidecarName, taNamespace, append(base, endpoint))
		Expect(err).To(HaveOccurred(), "client without a certificate must fail the TLS handshake (HTTP %s)", status)
		Expect(strings.TrimSpace(status)).To(Equal("000"))
		Expect(stderr).To(Or(ContainSubstring("SSL"), ContainSubstring("TLS"), ContainSubstring("certificate")))

		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		Expect(err).NotTo(HaveOccurred())
		template := &x509.Certificate{
			SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "ama-e2e-untrusted-client"},
			NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour),
			KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		}
		certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
		Expect(err).NotTo(HaveOccurred())
		keyDER, err := x509.MarshalPKCS8PrivateKey(key)
		Expect(err).NotTo(HaveOccurred())
		cert := base64.StdEncoding.EncodeToString(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}))
		privateKey := base64.StdEncoding.EncodeToString(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))
		script := `set -eu; d=$(mktemp -d); trap 'rm -f "$d/client.crt" "$d/client.key"; rmdir "$d"' EXIT; ` +
			fmt.Sprintf(`printf %%s %s | base64 -d > "$d/client.crt"; printf %%s %s | base64 -d > "$d/client.key"; `, cert, privateKey) +
			`curl --silent --show-error --noproxy '*' --max-time 10 --output /dev/null --write-out '%{http_code}' ` +
			`--cacert /etc/operator-targets/client/certs/ca.crt --cert "$d/client.crt" --key "$d/client.key" ` + endpoint
		status, stderr, err = utils.ExecCmd(K8sClient, Cfg, podName, taSidecarName, taNamespace, []string{"sh", "-c", script})
		Expect(err).To(HaveOccurred(), "self-signed untrusted client must fail the TLS handshake")
		Expect(strings.TrimSpace(status)).To(Equal("000"))
		Expect(stderr).To(Or(ContainSubstring("SSL"), ContainSubstring("TLS"), ContainSubstring("certificate")))
	})
})
