package operator

import (
	"fmt"
	"strings"

	"prometheus-collector/otelcollector/test/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// These e2e tests validate the MSRC 122861 hardening against the deployed target allocator:
//   - the plaintext scrape-config port 8080 is reachable only on localhost (so only the in-pod
//     config-reader sidecar can read it, not any other pod), and
//   - moving that listener to loopback did not break the target allocator liveness probe, whose
//     health endpoint (port 8081) must stay reachable on the pod IP for the kubelet.
const (
	taNamespace     = "kube-system"
	taLabelKey      = "rsName"
	taLabelValue    = "ama-metrics-operator-targets"
	taContainerName = "targetallocator"
	taSidecarName   = "config-reader"
	taListenAddrArg = "--listen-addr=127.0.0.1:8080"
)

var _ = Describe("Target Allocator port hardening (MSRC 122861)", Label(utils.OperatorLabel), func() {

	// hardenedTAPod returns the target allocator pod's name and IP, skipping the spec when the
	// cluster runs with HTTPS disabled (the loopback --listen-addr is only set in HTTPS mode).
	hardenedTAPod := func() (podName, podIP string) {
		pods, err := utils.GetPodsWithLabel(K8sClient, taNamespace, taLabelKey, taLabelValue)
		Expect(err).NotTo(HaveOccurred())
		Expect(pods).NotTo(BeEmpty(), "expected at least one ama-metrics-operator-targets pod")
		pod := pods[0]

		hardened := false
		for _, c := range pod.Spec.Containers {
			if c.Name == taContainerName {
				for _, a := range c.Args {
					if a == taListenAddrArg {
						hardened = true
					}
				}
			}
		}
		if !hardened {
			Skip("OperatorTargetsHttpsEnabled is false on this cluster; the port-8080 hardening only applies in HTTPS mode")
		}
		Expect(pod.Status.PodIP).NotTo(BeEmpty(), "pod IP should be assigned")
		return pod.Name, pod.Status.PodIP
	}

	// curlStatus execs curl in the config-reader sidecar and returns the HTTP status code. When
	// the connection is refused (nothing listening on that address) curl exits non-zero, which
	// ExecCmd surfaces as an error.
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
})
