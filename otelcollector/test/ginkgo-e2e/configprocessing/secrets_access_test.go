package configprocessing

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"prometheus-collector/otelcollector/test/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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

/*
 * E2E tests for secrets access scoping.
 *
 * These tests validate that the target allocator can read secrets for basic auth
 * in ServiceMonitors when:
 *   - The namespace is listed in secrets_access_namespaces
 *   - The appropriate Role+RoleBinding exist in that namespace
 *
 * Prerequisites:
 *   - A running AKS cluster with ama-metrics deployed
 *   - The ama-metrics-settings-configmap must already have secrets_access_namespaces
 *     configured (applied by the pipeline before running this test)
 *   - kubectl access to the cluster
 *
 * These tests MUST be run with the flag:
 * -ldflags="-s -X github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring.GroupName=azmonitoring.coreos.com"
 */
var _ = Describe("Secrets Access for Basic Auth in ServiceMonitors",
	Label(utils.ConfigProcessingSecretsAccessNamespaces), Ordered, Serial, func() {

		BeforeAll(func() {
			By("Creating the test namespace")
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: secretsTestNamespace,
				},
			}
			_, err := K8sClient.CoreV1().Namespaces().Create(context.Background(), ns, metav1.CreateOptions{})
			if err != nil && !strings.Contains(err.Error(), "already exists") {
				Fail(fmt.Sprintf("Failed to create test namespace: %v", err))
			}
		})

		AfterAll(func() {
			By("Cleaning up the test namespace")
			err := K8sClient.CoreV1().Namespaces().Delete(context.Background(), secretsTestNamespace, metav1.DeleteOptions{})
			if err != nil {
				fmt.Printf("Warning: failed to delete test namespace %s: %v\n", secretsTestNamespace, err)
			}
		})

		// ---------------------------------------------------------------------
		// Verify the configmap has secrets_access_namespaces configured
		// (applied by the pipeline before this test runs)
		// ---------------------------------------------------------------------
		Context("when the settings configmap is applied by the pipeline", func() {
			It("should have secrets_access_namespaces configured in the settings configmap", func() {
				configMap, err := K8sClient.CoreV1().ConfigMaps(amaMetricsNamespace).Get(
					context.Background(), settingsConfigMap, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred(), "ama-metrics-settings-configmap should exist")

				settings, ok := configMap.Data["prometheus-collector-settings"]
				Expect(ok).To(BeTrue(), "configmap should have prometheus-collector-settings key")

				Expect(settings).To(ContainSubstring("secrets_access_namespaces"),
					"configmap should contain secrets_access_namespaces setting")
				Expect(settings).To(ContainSubstring(secretsTestNamespace),
					"secrets_access_namespaces should include the test namespace %s", secretsTestNamespace)

				fmt.Printf("Current prometheus-collector-settings:\n%s\n", settings)
			})
		})

		// ---------------------------------------------------------------------
		// ServiceMonitor with basicAuth in a configured namespace + RBAC present
		// ---------------------------------------------------------------------
		Context("when secrets_access_namespaces includes the test namespace and RBAC is present", func() {
			BeforeAll(func() {
				By("Creating the basic auth secret matching the reference app credentials")
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      basicAuthSecretName,
						Namespace: secretsTestNamespace,
					},
					Type: corev1.SecretTypeOpaque,
					Data: map[string][]byte{
						"username": []byte("admin"),
						"password": []byte("pwd"),
					},
				}
				_, err := K8sClient.CoreV1().Secrets(secretsTestNamespace).Create(context.Background(), secret, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())

				By("Creating a test metrics app (simple pod exposing /metrics)")
				createTestMetricsApp(secretsTestNamespace)

				By("Waiting for the test metrics app pod to be ready")
				Eventually(func() bool {
					pod, err := K8sClient.CoreV1().Pods(secretsTestNamespace).Get(context.Background(), testAppName, metav1.GetOptions{})
					if err != nil {
						return false
					}
					for _, cond := range pod.Status.Conditions {
						if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
							return true
						}
					}
					return false
				}, pollTimeout, pollInterval).Should(BeTrue(), "test metrics app pod should become ready")

				By("Creating the secrets-reader Role in the test namespace")
				role := &rbacv1.Role{
					ObjectMeta: metav1.ObjectMeta{
						Name:      secretsRoleName,
						Namespace: secretsTestNamespace,
					},
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{""},
							Resources: []string{"secrets"},
							Verbs:     []string{"get", "list", "watch"},
						},
					},
				}
				_, err = K8sClient.RbacV1().Roles(secretsTestNamespace).Create(context.Background(), role, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())

				By("Creating the secrets-reader RoleBinding in the test namespace")
				rb := &rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      secretsRoleBindingName,
						Namespace: secretsTestNamespace,
					},
					Subjects: []rbacv1.Subject{
						{
							Kind:      "ServiceAccount",
							Name:      amaMetricsSA,
							Namespace: amaMetricsNamespace,
						},
					},
					RoleRef: rbacv1.RoleRef{
						Kind:     "Role",
						Name:     secretsRoleName,
						APIGroup: "rbac.authorization.k8s.io",
					},
				}
				_, err = K8sClient.RbacV1().RoleBindings(secretsTestNamespace).Create(context.Background(), rb, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())

				By("Creating a ServiceMonitor with basicAuth referencing the secret")
				sm := &monitoringv1.ServiceMonitor{
					ObjectMeta: metav1.ObjectMeta{
						Name:      serviceMonitorName,
						Namespace: secretsTestNamespace,
					},
					Spec: monitoringv1.ServiceMonitorSpec{
						Selector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": testAppName,
							},
						},
						Endpoints: []monitoringv1.Endpoint{
							{
								Port: "basicauth-metrics",
								Path: "/httpsmetrics",
								HTTPConfigWithProxyAndTLSFiles: monitoringv1.HTTPConfigWithProxyAndTLSFiles{
									HTTPConfigWithTLSFiles: monitoringv1.HTTPConfigWithTLSFiles{
										HTTPConfigWithoutTLS: monitoringv1.HTTPConfigWithoutTLS{
											BasicAuth: &monitoringv1.BasicAuth{
												Username: corev1.SecretKeySelector{
													LocalObjectReference: corev1.LocalObjectReference{
														Name: basicAuthSecretName,
													},
													Key: "username",
												},
												Password: corev1.SecretKeySelector{
													LocalObjectReference: corev1.LocalObjectReference{
														Name: basicAuthSecretName,
													},
													Key: "password",
												},
											},
										},
									},
								},
							},
						},
					},
				}
				_, err = PromClient.MonitoringV1().ServiceMonitors(secretsTestNamespace).Create(context.Background(), sm, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())
			})

			It("should discover the ServiceMonitor target in the target allocator", func() {
				// Verify the ServiceMonitor was created successfully
				sm, err := PromClient.MonitoringV1().ServiceMonitors(secretsTestNamespace).Get(context.Background(), serviceMonitorName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(sm.Spec.Endpoints[0].BasicAuth).NotTo(BeNil(), "ServiceMonitor should have basicAuth configured")

				fmt.Printf("ServiceMonitor %s/%s created with basicAuth referencing secret %s\n",
					sm.Namespace, sm.Name, basicAuthSecretName)
			})

			It("should have the basic auth secret readable by the target allocator", func() {
				// Verify the secret exists in the namespace
				secret, err := K8sClient.CoreV1().Secrets(secretsTestNamespace).Get(context.Background(), basicAuthSecretName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(secret.Data).To(HaveKey("username"))
				Expect(secret.Data).To(HaveKey("password"))
			})

			It("should have the RBAC resources created in the test namespace", func() {
				role, err := K8sClient.RbacV1().Roles(secretsTestNamespace).Get(context.Background(), secretsRoleName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(role.Rules[0].Resources).To(ContainElement("secrets"))

				rb, err := K8sClient.RbacV1().RoleBindings(secretsTestNamespace).Get(context.Background(), secretsRoleBindingName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(rb.Subjects[0].Name).To(Equal(amaMetricsSA))
				Expect(rb.Subjects[0].Namespace).To(Equal(amaMetricsNamespace))
			})

			It("should include the ServiceMonitor scrape job in the Prometheus scrape pool", func() {
				// Query the Prometheus UI from the ama-metrics replicaset pod to check
				// that the ServiceMonitor target appears in the scrape configuration.
				var result utils.APIResponse
				Eventually(func() error {
					return utils.QueryPromUIFromPod(
						K8sClient, Cfg,
						amaMetricsNamespace,
						amaMetricsLabelKey, amaMetricsLabelValue,
						"prometheus-collector",
						"/api/v1/targets",
						true,
						&result,
					)
				}, pollTimeout, pollInterval).Should(Succeed())

				// Parse the targets response to look for our ServiceMonitor job
				var targetsData struct {
					ActiveTargets []struct {
						ScrapePool string            `json:"scrapePool"`
						Labels     map[string]string `json:"labels"`
						Health     string            `json:"health"`
					} `json:"activeTargets"`
				}
				err := json.Unmarshal(result.Data, &targetsData)
				Expect(err).NotTo(HaveOccurred())

				found := false
				for _, target := range targetsData.ActiveTargets {
					if strings.Contains(target.ScrapePool, secretsTestNamespace) &&
						strings.Contains(target.ScrapePool, serviceMonitorName) {
						found = true
						fmt.Printf("Found target in scrape pool: %s (health: %s)\n", target.ScrapePool, target.Health)
						break
					}
				}
				Expect(found).To(BeTrue(),
					"Expected ServiceMonitor %s/%s to appear as an active target", secretsTestNamespace, serviceMonitorName)
			})

			AfterAll(func() {
				By("Cleaning up test resources")
				_ = PromClient.MonitoringV1().ServiceMonitors(secretsTestNamespace).Delete(context.Background(), serviceMonitorName, metav1.DeleteOptions{})
				_ = K8sClient.RbacV1().RoleBindings(secretsTestNamespace).Delete(context.Background(), secretsRoleBindingName, metav1.DeleteOptions{})
				_ = K8sClient.RbacV1().Roles(secretsTestNamespace).Delete(context.Background(), secretsRoleName, metav1.DeleteOptions{})
				_ = K8sClient.CoreV1().Secrets(secretsTestNamespace).Delete(context.Background(), basicAuthSecretName, metav1.DeleteOptions{})
				_ = K8sClient.CoreV1().Services(secretsTestNamespace).Delete(context.Background(), testAppName, metav1.DeleteOptions{})
				_ = K8sClient.CoreV1().Pods(secretsTestNamespace).Delete(context.Background(), testAppName, metav1.DeleteOptions{})
			})
		})

		// -------------------------------------------------------------------------
		// ServiceMonitor with basicAuth in a namespace NOT in secrets_access_namespaces
		// -------------------------------------------------------------------------
		Context("when secrets_access_namespaces does NOT include the test namespace", func() {
			const (
				unconfiguredNamespace  = "secrets-access-e2e-unconfigured"
				unconfiguredSecretName = "e2e-basic-auth-unconfigured"
				unconfiguredSvcMonName = "e2e-basicauth-svcmon-unconfigured"
			)

			BeforeAll(func() {
				By("Creating a namespace NOT listed in secrets_access_namespaces")
				ns := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: unconfiguredNamespace,
					},
				}
				_, err := K8sClient.CoreV1().Namespaces().Create(context.Background(), ns, metav1.CreateOptions{})
				if err != nil && !strings.Contains(err.Error(), "already exists") {
					Fail(fmt.Sprintf("Failed to create namespace: %v", err))
				}

				By("Creating a basic auth secret in the unconfigured namespace")
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      unconfiguredSecretName,
						Namespace: unconfiguredNamespace,
					},
					Type: corev1.SecretTypeOpaque,
					Data: map[string][]byte{
						"username": []byte("admin"),
						"password": []byte("pwd"),
					},
				}
				_, err = K8sClient.CoreV1().Secrets(unconfiguredNamespace).Create(context.Background(), secret, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())

				By("Creating a ServiceMonitor with basicAuth in the unconfigured namespace (no RBAC, no configmap entry)")
				sm := &monitoringv1.ServiceMonitor{
					ObjectMeta: metav1.ObjectMeta{
						Name:      unconfiguredSvcMonName,
						Namespace: unconfiguredNamespace,
					},
					Spec: monitoringv1.ServiceMonitorSpec{
						Selector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": testAppName,
							},
						},
						Endpoints: []monitoringv1.Endpoint{
							{
								Port: "basicauth-metrics",
								Path: "/httpsmetrics",
								HTTPConfigWithProxyAndTLSFiles: monitoringv1.HTTPConfigWithProxyAndTLSFiles{
									HTTPConfigWithTLSFiles: monitoringv1.HTTPConfigWithTLSFiles{
										HTTPConfigWithoutTLS: monitoringv1.HTTPConfigWithoutTLS{
											BasicAuth: &monitoringv1.BasicAuth{
												Username: corev1.SecretKeySelector{
													LocalObjectReference: corev1.LocalObjectReference{
														Name: unconfiguredSecretName,
													},
													Key: "username",
												},
												Password: corev1.SecretKeySelector{
													LocalObjectReference: corev1.LocalObjectReference{
														Name: unconfiguredSecretName,
													},
													Key: "password",
												},
											},
										},
									},
								},
							},
						},
					},
				}
				_, err = PromClient.MonitoringV1().ServiceMonitors(unconfiguredNamespace).Create(context.Background(), sm, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())
			})

			It("should NOT resolve the secret for the ServiceMonitor in the unconfigured namespace", func() {
				// Wait a reasonable time and verify the target does NOT appear with
				// resolved basic auth credentials. The ServiceMonitor is discovered
				// but the secret cannot be read — the scrape target should either be
				// absent or show as unhealthy because the basic auth cannot be resolved.
				//
				// We use Consistently to verify the target never appears healthy.
				Consistently(func() bool {
					var result utils.APIResponse
					err := utils.QueryPromUIFromPod(
						K8sClient, Cfg,
						amaMetricsNamespace,
						amaMetricsLabelKey, amaMetricsLabelValue,
						"prometheus-collector",
						"/api/v1/targets",
						true,
						&result,
					)
					if err != nil {
						return false // can't query yet, acceptable
					}

					var targetsData struct {
						ActiveTargets []struct {
							ScrapePool string `json:"scrapePool"`
							Health     string `json:"health"`
						} `json:"activeTargets"`
					}
					if err := json.Unmarshal(result.Data, &targetsData); err != nil {
						return false
					}

					for _, target := range targetsData.ActiveTargets {
						if strings.Contains(target.ScrapePool, unconfiguredNamespace) &&
							strings.Contains(target.ScrapePool, unconfiguredSvcMonName) &&
							target.Health == "up" {
							return true // target is healthy — this means the secret was resolved (unexpected)
						}
					}
					return false
				}, 2*time.Minute, pollInterval).Should(BeFalse(),
					"ServiceMonitor in unconfigured namespace should NOT have a healthy scrape target")
			})

			AfterAll(func() {
				By("Cleaning up resources for scenario")
				_ = PromClient.MonitoringV1().ServiceMonitors(unconfiguredNamespace).Delete(context.Background(), unconfiguredSvcMonName, metav1.DeleteOptions{})
				_ = K8sClient.CoreV1().Secrets(unconfiguredNamespace).Delete(context.Background(), unconfiguredSecretName, metav1.DeleteOptions{})
				err := K8sClient.CoreV1().Namespaces().Delete(context.Background(), unconfiguredNamespace, metav1.DeleteOptions{})
				if err != nil {
					fmt.Printf("Warning: failed to delete namespace %s: %v\n", unconfiguredNamespace, err)
				}
			})
		})

		// -------------------------------------------------------------------------
		// K8s < 1.36 with cluster-wide secrets access
		// -------------------------------------------------------------------------
		Context("when running on Kubernetes < 1.36", func() {
			It("should verify the ClusterRole includes cluster-wide secrets access", func() {
				serverVersion, err := K8sClient.Discovery().ServerVersion()
				Expect(err).NotTo(HaveOccurred())

				fmt.Printf("Cluster Kubernetes version: %s.%s\n", serverVersion.Major, serverVersion.Minor)

				// Parse version for comparison
				minor := strings.TrimRight(serverVersion.Minor, "+")
				if minor >= "36" {
					Skip("Cluster is running Kubernetes >= 1.36 — cluster-wide secrets test not applicable")
				}

				By("Verifying the ClusterRole includes cluster-wide secrets access on K8s < 1.36")
				clusterRole, err := K8sClient.RbacV1().ClusterRoles().Get(
					context.Background(), "ama-metrics-reader", metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())

				// Check that there's a rule with cluster-wide secrets get/list/watch
				hasClusterWideSecrets := false
				for _, rule := range clusterRole.Rules {
					hasSecrets := false
					hasGetListWatch := false
					isUnrestricted := len(rule.ResourceNames) == 0

					for _, res := range rule.Resources {
						if res == "secrets" {
							hasSecrets = true
							break
						}
					}
					for _, v := range rule.Verbs {
						if v == "list" || v == "watch" {
							hasGetListWatch = true
							break
						}
					}

					if hasSecrets && hasGetListWatch && isUnrestricted {
						hasClusterWideSecrets = true
						break
					}
				}
				Expect(hasClusterWideSecrets).To(BeTrue(),
					"On K8s < 1.36, ClusterRole ama-metrics-reader should include unrestricted secrets get/list/watch")
			})
		})

		// -------------------------------------------------------------------------
		// K8s >= 1.36 with no secrets_access_namespaces — no cluster-wide access
		// -------------------------------------------------------------------------
		Context("when running on Kubernetes >= 1.36", func() {
			It("should verify the ClusterRole does NOT include cluster-wide secrets access", func() {
				serverVersion, err := K8sClient.Discovery().ServerVersion()
				Expect(err).NotTo(HaveOccurred())

				minor := strings.TrimRight(serverVersion.Minor, "+")
				if minor < "36" {
					Skip("Cluster is running Kubernetes < 1.36 — this test is for >= 1.36 only")
				}

				By("Verifying the ClusterRole does NOT include cluster-wide secrets access on K8s >= 1.36")
				clusterRole, err := K8sClient.RbacV1().ClusterRoles().Get(
					context.Background(), "ama-metrics-reader", metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())

				for _, rule := range clusterRole.Rules {
					hasSecrets := false
					isUnrestricted := len(rule.ResourceNames) == 0

					for _, res := range rule.Resources {
						if res == "secrets" {
							hasSecrets = true
							break
						}
					}

					if hasSecrets && isUnrestricted {
						// This rule grants access to secrets without resource name restrictions.
						// On >= 1.36, there should be no such rule with list/watch.
						for _, v := range rule.Verbs {
							Expect(v).NotTo(Equal("list"),
								"ClusterRole should not have unrestricted secrets 'list' on K8s >= 1.36")
							Expect(v).NotTo(Equal("watch"),
								"ClusterRole should not have unrestricted secrets 'watch' on K8s >= 1.36")
						}
					}
				}
			})
		})
	})

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
							Name:          "basicauth-metrics",
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
					Name:       "basicauth-metrics",
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
