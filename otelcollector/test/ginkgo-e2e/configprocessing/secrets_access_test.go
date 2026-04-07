package configprocessing

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	// Test namespace for secrets access e2e tests.
	secretsTestNamespace = "secrets-access-e2e-test"

	// Name constants for test resources.
	basicAuthSecretName    = "e2e-basic-auth"
	serviceMonitorName     = "e2e-basicauth-svcmon"
	testAppName            = "e2e-metrics-app"
	secretsRoleName        = "ama-metrics-secrets-reader"
	secretsRoleBindingName = "ama-metrics-secrets-rolebinding"

	// ama-metrics service account lives in kube-system.
	amaMetricsNamespace = "kube-system"
	amaMetricsSA        = "ama-metrics-serviceaccount"
	settingsConfigMap   = "ama-metrics-settings-configmap"

	// Label used for the ama-metrics replicaset pods.
	amaMetricsLabelKey   = "rsName"
	amaMetricsLabelValue = "ama-metrics"

	// Polling settings for Eventually blocks.
	pollTimeout  = 5 * time.Minute
	pollInterval = 15 * time.Second
)

// createTestMetricsApp creates a pod using the prometheus-collector reference app
// (golang) and a service that exposes the basic-auth /httpsmetrics endpoint on port 2114.
// The reference app's handleRequest validates basic auth with username="admin", password="pwd".
func createTestMetricsApp(namespace string) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testAppName,
			Namespace: namespace,
			Labels: map[string]string{
				"app": testAppName,
			},
		},
		Spec: corev1.PodSpec{
			NodeSelector: map[string]string{
				"kubernetes.io/os":   "linux",
				"kubernetes.io/arch": "amd64",
			},
			Containers: []corev1.Container{
				{
					Name:  "prometheus-reference-app-golang",
					Image: "mcr.microsoft.com/azuremonitor/containerinsights/cidev/prometheus-collector/images:6.26.0-main-03-27-2026-1d8962bb-ref-app-golang",
					Ports: []corev1.ContainerPort{
						{
							Name:          "weather-app",
							ContainerPort: 2112,
							Protocol:      corev1.ProtocolTCP,
						},
						{
							Name:          "untyped-metrics",
							ContainerPort: 2113,
							Protocol:      corev1.ProtocolTCP,
						},
						{
							Name:          "ba-metrics",
							ContainerPort: 2114,
							Protocol:      corev1.ProtocolTCP,
						},
					},
				},
			},
		},
	}
	_, err := K8sClient.CoreV1().Pods(namespace).Create(context.Background(), pod, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())

	// Create a service exposing the basic-auth metrics port for ServiceMonitor discovery
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testAppName,
			Namespace: namespace,
			Labels: map[string]string{
				"app": testAppName,
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": testAppName,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "ba-metrics",
					Port:       2114,
					TargetPort: intstr.FromInt(2114),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
	_, err = K8sClient.CoreV1().Services(namespace).Create(context.Background(), svc, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())
}

var _ = Describe("Secrets access namespaces", Ordered, func() {
	BeforeAll(func() {
		By("Creating the test namespace")
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: secretsTestNamespace,
			},
		}
		_, err := K8sClient.CoreV1().Namespaces().Create(context.Background(), ns, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		By("Deploying the test metrics app with basic auth")
		createTestMetricsApp(secretsTestNamespace)

		By("Waiting for the metrics app pod to be running")
		Eventually(func() (corev1.PodPhase, error) {
			pod, err := K8sClient.CoreV1().Pods(secretsTestNamespace).Get(context.Background(), testAppName, metav1.GetOptions{})
			if err != nil {
				return "", err
			}
			return pod.Status.Phase, nil
		}, pollTimeout, pollInterval).Should(Equal(corev1.PodRunning))

		By("Verifying all containers are ready")
		Eventually(func() (bool, error) {
			pod, err := K8sClient.CoreV1().Pods(secretsTestNamespace).Get(context.Background(), testAppName, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			for _, cs := range pod.Status.ContainerStatuses {
				if !cs.Ready {
					return false, fmt.Errorf("container %s is not ready", cs.Name)
				}
			}
			return true, nil
		}, pollTimeout, pollInterval).Should(BeTrue())
	})

	AfterAll(func() {
		By("Deleting the test namespace and all resources")
		err := K8sClient.CoreV1().Namespaces().Delete(context.Background(), secretsTestNamespace, metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred())
	})

	It("should have the metrics app pod running", func() {
		pod, err := K8sClient.CoreV1().Pods(secretsTestNamespace).Get(context.Background(), testAppName, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(pod.Status.Phase).To(Equal(corev1.PodRunning))
	})
})
