package healthmetrics

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
	"github.com/prometheus-collector/test/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = ginkgo.Describe("CCP Health Metrics", ginkgo.Label("ccp"), func() {
	const (
		ccpNamespace    = "kube-system"
		ccpPodLabelKey  = "rsName"
		ccpPodLabelVal  = "ama-metrics-ccp"
		healthMetricsPort = 2234
	)

	ginkgo.Context("when CCP mode is enabled", func() {
		var ccpPod *corev1.Pod

		ginkgo.BeforeEach(func() {
			// Get the CCP pod
			pods, err := kubeClient.Clientset.CoreV1().Pods(ccpNamespace).List(context.Background(), metav1.ListOptions{
				LabelSelector: fmt.Sprintf("%s=%s", ccpPodLabelKey, ccpPodLabelVal),
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(pods.Items).NotTo(gomega.BeEmpty(), "CCP pod not found")
			
			ccpPod = &pods.Items[0]
			gomega.Expect(ccpPod.Status.Phase).To(gomega.Equal(corev1.PodRunning), "CCP pod is not running")
		})

		ginkgo.It("should expose health metrics endpoint on port 2234", func() {
			// Port forward to the CCP pod
			stopChan, readyChan := make(chan struct{}, 1), make(chan struct{}, 1)
			defer close(stopChan)

			localPort := 18234 // Use a different local port to avoid conflicts
			
			go func() {
				err := utils.PortForward(kubeClient.RestConfig, ccpPod.Name, ccpNamespace, localPort, healthMetricsPort, stopChan, readyChan)
				if err != nil {
					ginkgo.GinkgoLogr.Error(err, "Port forward failed")
				}
			}()

			// Wait for port forward to be ready
			select {
			case <-readyChan:
				ginkgo.GinkgoLogr.Info("Port forward ready")
			case <-time.After(10 * time.Second):
				ginkgo.Fail("Port forward did not become ready in time")
			}

			// Give it a moment to stabilize
			time.Sleep(1 * time.Second)

			// Try to access the health metrics endpoint
			metricsURL := fmt.Sprintf("http://localhost:%d/metrics", localPort)
			resp, err := http.Get(metricsURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to GET health metrics endpoint")
			defer resp.Body.Close()

			gomega.Expect(resp.StatusCode).To(gomega.Equal(http.StatusOK), "Health metrics endpoint returned non-200 status")
			
			body, err := io.ReadAll(resp.Body)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			bodyStr := string(body)

			// Verify the response contains Prometheus metrics format markers
			gomega.Expect(bodyStr).To(gomega.ContainSubstring("# HELP"), "Response does not contain Prometheus HELP comments")
			gomega.Expect(bodyStr).To(gomega.ContainSubstring("# TYPE"), "Response does not contain Prometheus TYPE comments")
		})

		ginkgo.It("should expose all 5 required health metrics", func() {
			// Port forward to the CCP pod
			stopChan, readyChan := make(chan struct{}, 1), make(chan struct{}, 1)
			defer close(stopChan)

			localPort := 18234
			
			go func() {
				err := utils.PortForward(kubeClient.RestConfig, ccpPod.Name, ccpNamespace, localPort, healthMetricsPort, stopChan, readyChan)
				if err != nil {
					ginkgo.GinkgoLogr.Error(err, "Port forward failed")
				}
			}()

			select {
			case <-readyChan:
			case <-time.After(10 * time.Second):
				ginkgo.Fail("Port forward did not become ready in time")
			}
			time.Sleep(1 * time.Second)

			// Get metrics
			metricsURL := fmt.Sprintf("http://localhost:%d/metrics", localPort)
			resp, err := http.Get(metricsURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			bodyStr := string(body)

			// Verify all required metrics are present
			requiredMetrics := []string{
				"timeseries_received_per_minute",
				"timeseries_sent_per_minute",
				"bytes_sent_per_minute",
				"invalid_custom_prometheus_config",
				"exporting_metrics_failed",
			}

			for _, metric := range requiredMetrics {
				gomega.Expect(bodyStr).To(gomega.ContainSubstring(metric), 
					fmt.Sprintf("Required metric %s not found in response", metric))
			}
		})

		ginkgo.It("should have correct labels on health metrics", func() {
			// Port forward to the CCP pod
			stopChan, readyChan := make(chan struct{}, 1), make(chan struct{}, 1)
			defer close(stopChan)

			localPort := 18234
			
			go func() {
				err := utils.PortForward(kubeClient.RestConfig, ccpPod.Name, ccpNamespace, localPort, healthMetricsPort, stopChan, readyChan)
				if err != nil {
					ginkgo.GinkgoLogr.Error(err, "Port forward failed")
				}
			}()

			select {
			case <-readyChan:
			case <-time.After(10 * time.Second):
				ginkgo.Fail("Port forward did not become ready in time")
			}
			time.Sleep(1 * time.Second)

			// Get metrics
			metricsURL := fmt.Sprintf("http://localhost:%d/metrics", localPort)
			resp, err := http.Get(metricsURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			bodyStr := string(body)

			// Verify labels are present on metrics
			// Look for the standard labels: computer, release, controller_type
			gomega.Expect(bodyStr).To(gomega.MatchRegexp(`computer="`), "Metrics missing 'computer' label")
			gomega.Expect(bodyStr).To(gomega.MatchRegexp(`release="`), "Metrics missing 'release' label")
			gomega.Expect(bodyStr).To(gomega.MatchRegexp(`controller_type="`), "Metrics missing 'controller_type' label")
		})

		ginkgo.It("should not have fluent-bit running in CCP mode", func() {
			// Execute ps command in the CCP pod to list running processes
			ctx := context.Background()
			
			// Get the prometheus-collector container
			var containerName string
			for _, container := range ccpPod.Spec.Containers {
				if strings.Contains(container.Name, "prometheus-collector") {
					containerName = container.Name
					break
				}
			}
			gomega.Expect(containerName).NotTo(gomega.BeEmpty(), "prometheus-collector container not found in CCP pod")

			// Execute ps command
			stdout, stderr, err := utils.ExecInPod(kubeClient, ccpPod.Name, ccpNamespace, containerName, []string{"ps", "aux"})
			gomega.Expect(err).NotTo(gomega.HaveOccurred(), fmt.Sprintf("Failed to exec in pod: %s", stderr))

			// Verify fluent-bit is NOT running
			gomega.Expect(stdout).NotTo(gomega.ContainSubstring("fluent-bit"), 
				"fluent-bit process should not be running in CCP mode")
			
			ginkgo.GinkgoLogr.Info("Verified fluent-bit is not running in CCP mode")
		})
	})
})
