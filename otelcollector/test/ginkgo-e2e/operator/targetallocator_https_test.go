package operator

import (
	"context"
	"fmt"
	"strings"

	"prometheus-collector/otelcollector/test/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// These e2e tests validate the MSRC 122861 hardening on a live cluster:
//   - the ama-metrics-operator-targets Service no longer exposes the unauthenticated
//     plaintext port 80 when HTTPS is enabled (only the mTLS port 443->8443 remains), and
//   - the target allocator binds its plaintext HTTP listener to 127.0.0.1:8080 (via
//     --listen-addr) and stops declaring containerPort 8080, so the scrape-config/topology/
//     pprof endpoints are reachable only by the in-pod config-reader sidecar.
const (
	taNamespace     = "kube-system"
	taServiceName   = "ama-metrics-operator-targets"
	taLabelKey      = "rsName"
	taLabelValue    = "ama-metrics-operator-targets"
	taContainerName = "targetallocator"
	taSidecarName   = "config-reader"
	taListenAddrArg = "--listen-addr=127.0.0.1:8080"

	// In /proc/net/tcp the local_address is "<little-endian IPv4 hex>:<port hex>". Port 8080
	// is 0x1F90; 127.0.0.1 encodes to 0100007F and 0.0.0.0 to 00000000. A loopback-bound
	// plaintext listener therefore appears as 0100007F:1F90, whereas the pre-fix all-interfaces
	// binding would appear as 00000000:1F90.
	loopbackListen8080   = "0100007F:1F90"
	allInterfacesListen  = "00000000:1F90"
	procNetTCPReadScript = `while IFS= read -r line; do echo "$line"; done < /proc/net/tcp`
)

var _ = Describe("Target Allocator unauthenticated-port hardening (MSRC 122861)", Label(utils.OperatorLabel), func() {

	// getTargetAllocatorContainer returns the targetallocator container spec of the running pod.
	getTargetAllocatorContainer := func() (corev1.Container, string) {
		pods, err := utils.GetPodsWithLabel(K8sClient, taNamespace, taLabelKey, taLabelValue)
		Expect(err).NotTo(HaveOccurred())
		Expect(pods).NotTo(BeEmpty(), "expected at least one ama-metrics-operator-targets pod")
		pod := pods[0]
		for _, c := range pod.Spec.Containers {
			if c.Name == taContainerName {
				return c, pod.Name
			}
		}
		Fail(fmt.Sprintf("container %q not found in pod %s", taContainerName, pod.Name))
		return corev1.Container{}, ""
	}

	// httpsEnabled reports whether the deployment runs in the hardened HTTPS mode. The
	// --listen-addr=127.0.0.1:8080 argument is only rendered when OperatorTargetsHttpsEnabled=true.
	httpsEnabled := func(c corev1.Container) bool {
		for _, a := range c.Args {
			if a == taListenAddrArg {
				return true
			}
		}
		return false
	}

	// requireHardenedTAContainer skips the spec when the cluster runs with HTTPS disabled, since
	// the MSRC 122861 hardening only applies in the default HTTPS-enabled configuration.
	requireHardenedTAContainer := func() (corev1.Container, string) {
		c, podName := getTargetAllocatorContainer()
		if !httpsEnabled(c) {
			Skip("OperatorTargetsHttpsEnabled is false on this cluster; MSRC 122861 hardening applies only in HTTPS mode")
		}
		return c, podName
	}

	It("does not expose the unauthenticated plaintext port 80 on the Service when HTTPS is enabled", func() {
		requireHardenedTAContainer()

		svc, err := K8sClient.CoreV1().Services(taNamespace).Get(context.Background(), taServiceName, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())

		for _, p := range svc.Spec.Ports {
			Expect(p.Port).NotTo(Equal(int32(80)),
				"Service must not expose the unauthenticated plaintext port 80 when HTTPS is enabled (MSRC 122861)")
			Expect(p.Name).NotTo(Equal("targetallocation"),
				"the plaintext 'targetallocation' port must be removed when HTTPS is enabled")
		}

		var httpsPort *corev1.ServicePort
		for i := range svc.Spec.Ports {
			if svc.Spec.Ports[i].Name == "targetallocation-https" {
				httpsPort = &svc.Spec.Ports[i]
			}
		}
		Expect(httpsPort).NotTo(BeNil(), "the mTLS 'targetallocation-https' port must still be exposed")
		Expect(httpsPort.Port).To(Equal(int32(443)))
		Expect(httpsPort.TargetPort.IntValue()).To(Equal(8443))
	})

	It("binds the plaintext target-allocator listener to loopback and stops exposing containerPort 8080", func() {
		container, _ := requireHardenedTAContainer()

		Expect(container.Args).To(ContainElement(taListenAddrArg),
			"target allocator must bind its plaintext listener to loopback via --listen-addr=127.0.0.1:8080")

		for _, p := range container.Ports {
			Expect(p.ContainerPort).NotTo(Equal(int32(8080)),
				"containerPort 8080 must not be declared when HTTPS is enabled (MSRC 122861)")
		}

		var hasHTTPS bool
		for _, p := range container.Ports {
			if p.ContainerPort == 8443 {
				hasHTTPS = true
			}
		}
		Expect(hasHTTPS).To(BeTrue(), "the mTLS containerPort 8443 must remain declared")
	})

	It("keeps the plaintext scrape-config listener reachable only on loopback inside the pod", func() {
		_, podName := requireHardenedTAContainer()

		// Read the pod's listening sockets from the shared network namespace using pure bash
		// builtins (no curl/cat dependency). Containers in a pod share a netns, so the sidecar
		// observes the target allocator's listeners.
		command := []string{"bash", "-c", procNetTCPReadScript}
		stdout, _, err := utils.ExecCmd(K8sClient, Cfg, podName, taSidecarName, taNamespace, command)
		Expect(err).NotTo(HaveOccurred())

		listeners := strings.ToUpper(stdout)
		Expect(listeners).To(ContainSubstring(loopbackListen8080),
			"the plaintext :8080 listener must be bound to loopback (127.0.0.1) so only the in-pod sidecar can reach it")
		Expect(listeners).NotTo(ContainSubstring(allInterfacesListen),
			"the plaintext :8080 listener must NOT be bound to all interfaces (0.0.0.0) (MSRC 122861)")
	})
})
