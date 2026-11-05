package configprocessing

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/model/relabel"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilversion "k8s.io/apimachinery/pkg/util/version"
	"sigs.k8s.io/yaml"

	"prometheus-collector/otelcollector/test/utils"
)

type transitionTarget struct {
	ScrapePool       string            `json:"scrapePool"`
	Labels           map[string]string `json:"labels"`
	DiscoveredLabels map[string]string `json:"discoveredLabels"`
	Health           string            `json:"health"`
	LastError        string            `json:"lastError"`
	LastScrape       time.Time         `json:"lastScrape"`
}

func transitionFixture(parts ...string) []byte {
	path := filepath.Join(append([]string{"..", "..", "scenarios", "settings"}, parts...)...)
	data, err := os.ReadFile(path)
	Expect(err).NotTo(HaveOccurred(), "fixture must come from this verified test-source checkout: %s", path)
	return data
}

func replaceSettings(data map[string]string) {
	cm, err := K8sClient.CoreV1().ConfigMaps("kube-system").Get(context.Background(),
		"ama-metrics-settings-configmap", metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())
	cm.Data = maps.Clone(data)
	_, err = K8sClient.CoreV1().ConfigMaps("kube-system").Update(context.Background(), cm, metav1.UpdateOptions{})
	Expect(err).NotTo(HaveOccurred())
}

func settingsPhase(variant, file string) time.Time {
	var cm corev1.ConfigMap
	Expect(yaml.Unmarshal(transitionFixture(variant, file), &cm)).To(Succeed())
	Expect(cm.Name).To(Equal("ama-metrics-settings-configmap"))
	Expect(cm.Namespace).To(Equal("kube-system"))
	started := time.Now()
	replaceSettings(cm.Data)
	AddReportEntry("settings-phase-"+file, started.UTC().Format(time.RFC3339Nano))
	return started
}

func restoreSettingsAfterTransition() {
	cm, err := K8sClient.CoreV1().ConfigMaps("kube-system").Get(context.Background(),
		"ama-metrics-settings-configmap", metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())
	original := maps.Clone(cm.Data)
	DeferCleanup(func() { replaceSettings(original) })
}

func transitionAPI() promv1.API {
	endpoint := os.Getenv("AMW_QUERY_ENDPOINT")
	Expect(endpoint).NotTo(BeEmpty(), "selected workflow requires the existing AMW query endpoint")
	api, err := utils.CreatePrometheusAPIClient(endpoint)
	Expect(err).NotTo(HaveOccurred())
	return api
}

func fallbackCluster(cluster, mac string) (string, error) {
	if strings.TrimSpace(mac) == "true" {
		parts := strings.Split(strings.TrimSpace(cluster), "/")
		cluster = parts[len(parts)-1]
	}
	if strings.TrimSpace(cluster) == "" {
		return "", fmt.Errorf("collector CLUSTER must independently establish the non-alias identity")
	}
	return cluster, nil
}

func transitionCluster() string {
	pods, err := utils.GetPodsWithLabel(K8sClient, "kube-system", "rsName", "ama-metrics")
	Expect(err).NotTo(HaveOccurred())
	var cluster, mac string
	for _, container := range pods[0].Spec.Containers {
		if container.Name != "prometheus-collector" {
			continue
		}
		for _, variable := range container.Env {
			switch variable.Name {
			case "CLUSTER":
				cluster = variable.Value
			case "MAC":
				mac = variable.Value
			}
		}
	}
	result, err := fallbackCluster(cluster, mac)
	Expect(err).NotTo(HaveOccurred())
	return result
}

func transitionTargets() ([]transitionTarget, error) {
	responses, err := utils.QueryPromUIFromPods(K8sClient, Cfg, "kube-system", "rsName", "ama-metrics",
		"prometheus-collector", "/api/v1/targets", true)
	if err != nil {
		return nil, err
	}
	var targets []transitionTarget
	for _, response := range responses {
		var data struct {
			ActiveTargets []transitionTarget `json:"activeTargets"`
		}
		if err := json.Unmarshal(response.Data, &data); err != nil {
			return nil, err
		}
		targets = append(targets, data.ActiveTargets...)
	}
	return targets, nil
}

func validateTransitionTargets(targets []transitionTarget, jobs []string, health string, after time.Time) error {
	for _, job := range jobs {
		found := false
		for _, target := range targets {
			if target.ScrapePool != job {
				continue
			}
			found = true
			if target.Health != health || target.LastScrape.Before(after) {
				return fmt.Errorf("%s must have a fresh %s scrape", job, health)
			}
			if health == "down" && !strings.Contains(target.LastError, "401") {
				return fmt.Errorf("%s must fail authentication, not an unrelated scrape error", job)
			}
		}
		if !found {
			return fmt.Errorf("missing expected monitor target %s", job)
		}
	}
	return nil
}

func monitorTransitionState(jobs []string, username string, absent bool) error {
	responses, err := utils.QueryPromUIFromPods(K8sClient, Cfg, "kube-system", "rsName", "ama-metrics",
		"prometheus-collector", "/api/v1/status/config", true)
	if err != nil {
		return err
	}
	for _, response := range responses {
		var data promv1.ConfigResult
		if err := json.Unmarshal(response.Data, &data); err != nil {
			return err
		}
		cfg, err := config.Load(data.YAML, nil)
		if err != nil {
			return err
		}
		for _, expected := range jobs {
			found := false
			for _, job := range cfg.ScrapeConfigs {
				if job.JobName != expected {
					continue
				}
				found = true
				if !absent && (job.HTTPClientConfig.BasicAuth == nil ||
					job.HTTPClientConfig.BasicAuth.Username != username) {
					return fmt.Errorf("pod %s has not reloaded the expected dummy username for %s", response.PodName, expected)
				}
			}
			if found == absent {
				return fmt.Errorf("pod %s: %s presence=%t, expected absence=%t", response.PodName, expected, found, absent)
			}
		}
	}
	return nil
}

func freshTransitionSamples(api promv1.API, selector string, after time.Time) (model.Vector, error) {
	warnings, result, err := utils.InstantQuery(api, utils.FreshMetricQuery(selector, after))
	if err != nil {
		return nil, err
	}
	if len(warnings) != 0 {
		return nil, fmt.Errorf("AMW returned query warnings: %v", warnings)
	}
	vector, ok := result.(model.Vector)
	if !ok {
		return nil, fmt.Errorf("AMW query did not return an instant vector")
	}
	return vector, nil
}

func expectTransitionMetric(api promv1.API, metric, job, cluster, node string, after time.Time) {
	selector := fmt.Sprintf("%s{job=%q,cluster=%q}", metric, job, cluster)
	if node != "" {
		selector = fmt.Sprintf("%s{job=%q,cluster=%q,instance=%q}", metric, job, cluster, node)
	}
	Eventually(func(g Gomega) {
		samples, err := freshTransitionSamples(api, selector, after)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(utils.ValidateMetricSamples(samples, metric, job, cluster, time.Now(), metric == "up")).To(Succeed())
	}, 8*time.Minute, 15*time.Second).Should(Succeed())
}

func expectWindowsKubeProxy(api promv1.API, cluster string, after time.Time) {
	pods, err := utils.GetPodsWithLabel(K8sClient, "kube-system", "dsName", "ama-metrics-win-node")
	Expect(err).NotTo(HaveOccurred(), "actual Windows collectors are required; do not substitute Linux metrics")
	const metric = "kubeproxy_sync_proxy_rules_duration_seconds_count"
	present := regexp.MustCompile(`(?m)^` + metric + `(?:\{|[ \t])`)
	for _, pod := range pods {
		Expect(pod.Spec.NodeName).NotTo(BeEmpty())
		Expect(pod.Status.HostIP).NotTo(BeEmpty())
		url := "http://" + pod.Status.HostIP + ":10249/metrics"
		command := []string{"powershell", "-NoProfile", "-Command",
			fmt.Sprintf("(Invoke-WebRequest -UseBasicParsing -TimeoutSec 30 '%s').Content", url)}
		out, _, err := utils.ExecCmd(K8sClient, Cfg, pod.Name, "prometheus-collector", "kube-system", command)
		Expect(err).NotTo(HaveOccurred(), "unsupported/unreachable Windows kube-proxy metrics endpoint %s", url)
		Expect(present.MatchString(out)).To(BeTrue(), "unsupported Windows kube-proxy metric %s on node %s", metric, pod.Spec.NodeName)
		expectTransitionMetric(api, "up", "kube-proxy-windows", cluster, pod.Spec.NodeName, after)
		expectTransitionMetric(api, metric, "kube-proxy-windows", cluster, pod.Spec.NodeName, after)
	}
}

type annotationExpectation struct {
	regex      string
	namespaces []string
}

func validateAnnotationTargets(targets []transitionTarget, allowed []string, after time.Time) error {
	found := map[string]bool{}
	for _, target := range targets {
		pod := target.Labels["kubernetes_pod_name"]
		if discovered := target.DiscoveredLabels["__meta_kubernetes_pod_name"]; discovered != "" {
			pod = discovered
		}
		if target.ScrapePool != "kubernetes-pods" || (pod != "annotation-target" && pod != "unannotated-target") {
			continue
		}
		namespace := target.Labels["kubernetes_namespace"]
		if discovered := target.DiscoveredLabels["__meta_kubernetes_namespace"]; discovered != "" {
			namespace = discovered
		}
		if !slices.Contains([]string{"ama-settings-included", "ama-settings-excluded", "ama-settings-included-similar"}, namespace) {
			continue
		}
		if target.Labels["kubernetes_namespace"] != namespace || target.Labels["kubernetes_pod_name"] != pod {
			return fmt.Errorf("annotation fixture %s/%s is missing its exported identity labels", namespace, pod)
		}
		if pod == "unannotated-target" || !slices.Contains(allowed, namespace) {
			return fmt.Errorf("excluded annotation fixture %s/%s remains active", namespace, pod)
		}
		if target.Health != "up" || target.LastScrape.Before(after) {
			return fmt.Errorf("annotation fixture %s/%s has no fresh healthy scrape", namespace, pod)
		}
		found[namespace] = true
	}
	for _, namespace := range allowed {
		if !found[namespace] {
			return fmt.Errorf("missing admitted annotation target in %s", namespace)
		}
	}
	return nil
}

func expectAnnotationNamespaces(api promv1.API, cluster string, expected annotationExpectation, after time.Time) {
	checkTargets := func() error {
		responses, err := utils.QueryPromUIFromPods(K8sClient, Cfg, "kube-system", "rsName", "ama-metrics",
			"prometheus-collector", "/api/v1/status/config", true)
		if err != nil {
			return err
		}
		for _, response := range responses {
			var data promv1.ConfigResult
			if err := json.Unmarshal(response.Data, &data); err != nil {
				return err
			}
			cfg, err := config.Load(data.YAML, nil)
			if err != nil {
				return err
			}
			if err := validateAnnotationConfig(cfg.ScrapeConfigs, expected.regex); err != nil {
				return err
			}
		}
		targets, err := transitionTargets()
		if err != nil {
			return err
		}
		return validateAnnotationTargets(targets, expected.namespaces, after)
	}
	Eventually(checkTargets, 8*time.Minute, 15*time.Second).Should(Succeed())
	// Samples scraped before reload converges are not evidence of continued excluded ingestion.
	after = time.Now()
	AddReportEntry("annotation-observation-start", after.UTC().Format(time.RFC3339Nano))
	check := func() error {
		if err := checkTargets(); err != nil {
			return err
		}
		for _, namespace := range []string{"ama-settings-included", "ama-settings-excluded", "ama-settings-included-similar"} {
			selector := fmt.Sprintf("weather_temperature{job=%q,cluster=%q,kubernetes_namespace=%q,kubernetes_pod_name=%q}",
				"kubernetes-pods", cluster, namespace, "annotation-target")
			samples, err := freshTransitionSamples(api, selector, after)
			if err != nil {
				return err
			}
			if slices.Contains(expected.namespaces, namespace) {
				if err := utils.ValidateMetricSamples(samples, "weather_temperature", "kubernetes-pods", cluster, time.Now(), false); err != nil {
					return err
				}
			} else if len(samples) != 0 {
				return fmt.Errorf("excluded annotation namespace %s still ingests new samples", namespace)
			}
		}
		samples, err := freshTransitionSamples(api,
			fmt.Sprintf("weather_temperature{job=%q,cluster=%q,kubernetes_pod_name=%q}",
				"kubernetes-pods", cluster, "unannotated-target"), after)
		if err != nil {
			return err
		}
		if len(samples) != 0 {
			return fmt.Errorf("unannotated negative control ingests new samples")
		}
		return nil
	}
	Eventually(check, 8*time.Minute, 15*time.Second).Should(Succeed())
	Consistently(check, time.Minute, 15*time.Second).Should(Succeed())
}

func validateAnnotationConfig(jobs []*config.ScrapeConfig, expression string) error {
	for _, job := range jobs {
		if job.JobName != "kubernetes-pods" {
			continue
		}
		if expression == "" {
			return fmt.Errorf("removed annotation job remains configured")
		}
		for _, rule := range job.RelabelConfigs {
			if string(rule.Action) == "keep" && rule.SourceLabels.String() == "__meta_kubernetes_namespace" &&
				rule.Regex.String() == expression {
				return nil
			}
		}
		return fmt.Errorf("annotation namespace selector has not converged")
	}
	if expression != "" {
		return fmt.Errorf("admitted annotation job missing from config")
	}
	return nil
}

var _ = Describe("Customer alias and annotation transitions", Serial, func() {
	for _, variant := range []struct{ schema, label string }{
		{"v1", utils.ConfigProcessingCustomerTransitionsV1},
		{"v2", utils.ConfigProcessingCustomerTransitionsV2},
	} {
		schema := variant.schema
		It("changes and clears alias, then updates and removes annotation namespace selection in "+schema,
			Label(variant.label), func() {
				restoreSettingsAfterTransition()
				api, fallback := transitionAPI(), transitionCluster()
				for _, phase := range []struct{ file, cluster, namespace string }{
					{"initial.yaml", "Customer_Config_POC", "ama-settings-included"},
					{"alias-updated.yaml", "Customer_Config_Updated", "ama-settings-included"},
					{"alias-cleared.yaml", fallback, "ama-settings-included"},
					{"namespace-updated.yaml", fallback, "ama-settings-excluded"},
					{"namespace-removed.yaml", fallback, ""},
				} {
					started := settingsPhase("customer-transitions-"+schema, phase.file)
					Eventually(func() error {
						targets, err := transitionTargets()
						if err != nil {
							return err
						}
						return validateTransitionTargets(targets, []string{
							"podMonitor/ama-settings-included/settings-alias/0",
							"serviceMonitor/ama-settings-included/settings-alias/0",
						}, "up", started)
					}, 8*time.Minute, 15*time.Second).Should(Succeed())
					for _, job := range []string{"ama-settings-included/settings-alias", "settings-monitor-target"} {
						expectTransitionMetric(api, "weather_temperature", job, phase.cluster, "", started)
						expectTransitionMetric(api, "up", job, phase.cluster, "", started)
					}
					expectWindowsKubeProxy(api, phase.cluster, started)
					expected := annotationExpectation{}
					if phase.namespace != "" {
						expected = annotationExpectation{regex: "^" + phase.namespace + "$", namespaces: []string{phase.namespace}}
					}
					expectAnnotationNamespaces(api, phase.cluster, expected, started)
				}
			})
	}
})

var _ = Describe("Annotation namespace regex edges", Serial, func() {
	for _, variant := range []struct{ schema, label string }{
		{"v1", utils.ConfigProcessingAnnotationRegexEdgesV1},
		{"v2", utils.ConfigProcessingAnnotationRegexEdgesV2},
	} {
		schema := variant.schema
		It("handles absent, empty, alternation, wildcard and invalid regex on normal reload in "+schema,
			Label(variant.label), func() {
				restoreSettingsAfterTransition()
				api, cluster := transitionAPI(), transitionCluster()
				collectors, err := utils.GetPodsWithLabel(K8sClient, "kube-system", "rsName", "ama-metrics")
				Expect(err).NotTo(HaveOccurred())
				for _, fixture := range []struct{ namespace, pod string }{
					{"ama-settings-included", "annotation-target"},
					{"ama-settings-excluded", "annotation-target"},
					{"ama-settings-included-similar", "annotation-target"},
					{"ama-settings-included", "unannotated-target"},
				} {
					Eventually(func(g Gomega) {
						pod, err := K8sClient.CoreV1().Pods(fixture.namespace).Get(context.Background(), fixture.pod, metav1.GetOptions{})
						g.Expect(err).NotTo(HaveOccurred())
						g.Expect(pod.Status.PodIP).NotTo(BeEmpty())
						g.Expect(pod.Annotations["prometheus.io/scrape"] == "true").To(Equal(fixture.pod == "annotation-target"))
						url := "http://" + net.JoinHostPort(pod.Status.PodIP, "2112") + "/metrics"
						out, _, err := utils.ExecCmd(K8sClient, Cfg, collectors[0].Name, "prometheus-collector",
							"kube-system", []string{"curl", "--fail", "--silent", "--show-error", "--noproxy", "*", "--max-time", "30", url})
						g.Expect(err).NotTo(HaveOccurred())
						g.Expect(out).To(MatchRegexp(`(?m)^weather_temperature(?:\{|[ \t])`))
					}, 8*time.Minute, 15*time.Second).Should(Succeed())
				}
				for _, phase := range []struct {
					file     string
					expected annotationExpectation
				}{
					{"default.yaml", annotationExpectation{}},
					{"empty.yaml", annotationExpectation{}},
					{"invalid.yaml", annotationExpectation{}},
					{"alternation.yaml", annotationExpectation{
						regex:      "^(ama-settings-included|ama-settings-excluded)$",
						namespaces: []string{"ama-settings-included", "ama-settings-excluded"},
					}},
					{"wildcard.yaml", annotationExpectation{
						regex: ".*", namespaces: []string{"ama-settings-included", "ama-settings-excluded", "ama-settings-included-similar"},
					}},
					{"invalid.yaml", annotationExpectation{}},
				} {
					started := settingsPhase("annotation-regex-edges-"+schema, phase.file)
					if phase.file == "invalid.yaml" {
						Eventually(func(g Gomega) {
							for _, source := range []struct{ label, container string }{
								{"ama-metrics", "prometheus-collector"}, {"ama-metrics-operator-targets", "config-reader"},
							} {
								pods, err := utils.GetPodsWithLabel(K8sClient, "kube-system", "rsName", source.label)
								g.Expect(err).NotTo(HaveOccurred())
								for _, pod := range pods {
									startedAt := time.Time{}
									for _, status := range pod.Status.ContainerStatuses {
										if status.Name == source.container && status.Ready && status.State.Running != nil {
											startedAt = status.State.Running.StartedAt.Time
										}
									}
									g.Expect(startedAt.After(started)).To(BeTrue(),
										"%s/%s must reload through its normal fresh-process lifecycle", pod.Name, source.container)
									logs, err := K8sClient.CoreV1().Pods("kube-system").GetLogs(pod.Name, &corev1.PodLogOptions{
										Container: source.container, SinceTime: &metav1.Time{Time: started},
									}).DoRaw(context.Background())
									g.Expect(err).NotTo(HaveOccurred())
									g.Expect(strings.Contains(string(logs), "Invalid namespace regex for pod annotations: [")).To(BeTrue(),
										"%s/%s must report the invalid annotation regex", pod.Name, source.container)
								}
							}
						}, 8*time.Minute, 15*time.Second).Should(Succeed())
						AddReportEntry("invalid-annotation-regex", "diagnostic verified after normal collector/reader reload")
					}
					expectAnnotationNamespaces(api, cluster, phase.expected, started)
					expectTransitionMetric(api, "up", "kubelet", cluster, "", started)
				}
			})
	}
})

func replaceAuthSecret(data map[string][]byte) {
	secret, err := K8sClient.CoreV1().Secrets("ama-settings-auth").Get(context.Background(),
		"settings-auth-creds", metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())
	secret.Data = data
	_, err = K8sClient.CoreV1().Secrets("ama-settings-auth").Update(context.Background(), secret, metav1.UpdateOptions{})
	Expect(err).NotTo(HaveOccurred())
}

func recoverAuthSecret() time.Time {
	var secret corev1.Secret
	Expect(yaml.Unmarshal(transitionFixture("auth-recovered.yaml"), &secret)).To(Succeed())
	Expect(secret.Name).To(Equal("settings-auth-creds"))
	Expect(secret.Namespace).To(Equal("ama-settings-auth"))
	data := map[string][]byte{}
	for key, value := range secret.StringData {
		data[key] = []byte(value)
	}
	started := time.Now()
	replaceAuthSecret(data)
	return started
}

func collectorRestarts() map[string]int32 {
	result := map[string]int32{}
	for _, selector := range []struct{ namespace, key, value string }{
		{"kube-system", "rsName", "ama-metrics"}, {"kube-system", "rsName", "ama-metrics-operator-targets"},
		{"ama-settings-auth", "app", "ama-settings-auth-target"},
	} {
		pods, err := utils.GetPodsWithLabel(K8sClient, selector.namespace, selector.key, selector.value)
		Expect(err).NotTo(HaveOccurred())
		for _, pod := range pods {
			Expect(pod.UID).NotTo(BeEmpty())
			Expect(pod.Status.ContainerStatuses).To(HaveLen(len(pod.Spec.Containers)))
			for _, status := range pod.Status.ContainerStatuses {
				result[string(pod.UID)+"/"+status.Name] = status.RestartCount
			}
		}
	}
	Expect(result).NotTo(BeEmpty())
	return result
}

var _ = Describe("Credential reload and allowlist transitions, not valid-password rotation", Serial, func() {
	for _, variant := range []struct{ schema, label string }{
		{"v1", utils.ConfigProcessingSecretTransitionsV1},
		{"v2", utils.ConfigProcessingSecretTransitionsV2},
	} {
		schema := variant.schema
		Describe(schema, Ordered, Label(variant.label), func() {
			var api promv1.API
			var cluster string
			jobs := []string{"podMonitor/ama-settings-auth/settings-auth/0", "serviceMonitor/ama-settings-auth/settings-auth/0"}
			metrics := []string{"ama-settings-auth/settings-auth", "settings-auth-target"}
			BeforeAll(func() {
				restoreSettingsAfterTransition()
				secret, err := K8sClient.CoreV1().Secrets("ama-settings-auth").Get(context.Background(),
					"settings-auth-creds", metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				original := secret.DeepCopy().Data
				DeferCleanup(func() { replaceAuthSecret(original) })
				api, cluster = transitionAPI(), transitionCluster()
				AddReportEntry("true-credential-rotation", "deferred by user; only invalid-to-valid auth recovery is exercised")
			})
			recovered := func(after time.Time) {
				Eventually(func() error { return monitorTransitionState(jobs, "admin", false) },
					8*time.Minute, 15*time.Second).Should(Succeed())
				Eventually(func() error {
					targets, err := transitionTargets()
					if err != nil {
						return err
					}
					return validateTransitionTargets(targets, jobs, "up", after)
				}, 8*time.Minute, 15*time.Second).Should(Succeed())
				for _, job := range metrics {
					expectTransitionMetric(api, "my_metric", job, cluster, "", after)
				}
			}
			It("reloads the dummy Secret from rejected credentials to authenticated fresh ingestion without a restart", func() {
				started, restarts := time.Now(), collectorRestarts()
				Eventually(func() error { return monitorTransitionState(jobs, "deliberately-invalid-test-user", false) },
					8*time.Minute, 15*time.Second).Should(Succeed())
				Eventually(func() error {
					targets, err := transitionTargets()
					if err != nil {
						return err
					}
					return validateTransitionTargets(targets, jobs, "down", started)
				}, 8*time.Minute, 15*time.Second).Should(Succeed())
				recovered(recoverAuthSecret())
				Expect(collectorRestarts()).To(Equal(restarts), "Secret reload must not restart collectors, allocator or endpoint")
			})
			It("revokes cached Secret access for empty and omitted allowlists, then restores it on the supported scoped path", func() {
				version, err := K8sClient.Discovery().ServerVersion()
				Expect(err).NotTo(HaveOccurred())
				parsed, err := utilversion.ParseSemantic(version.GitVersion)
				Expect(err).NotTo(HaveOccurred())
				if !parsed.AtLeast(utilversion.MustParseSemantic("v1.37.0")) {
					Skip("unsupported below Kubernetes1.37; auth recovery passed separately, scoped enforcement is not covered")
				}
				for _, phase := range []string{"empty.yaml", "omitted.yaml"} {
					started := settingsPhase("secret-transitions-"+schema, phase)
					replaceAuthSecret(map[string][]byte{"username": []byte("deliberately-invalid-test-user"), "password": []byte("pwd")})
					check := func() error {
						if err := monitorTransitionState(jobs, "", true); err != nil {
							return err
						}
						for _, job := range metrics {
							samples, err := freshTransitionSamples(api,
								fmt.Sprintf("my_metric{job=%q,cluster=%q}", job, cluster), started)
							if err != nil {
								return err
							}
							if len(samples) != 0 {
								return fmt.Errorf("revoked monitor %s continues authenticated ingestion", job)
							}
						}
						return nil
					}
					Eventually(check, 8*time.Minute, 15*time.Second).Should(Succeed())
					Consistently(check, time.Minute, 15*time.Second).Should(Succeed())
					settingsPhase("secret-transitions-"+schema, "allowed.yaml")
					recovered(recoverAuthSecret())
				}
			})
		})
	}
})

var _ = Describe("Dedicated Windows kube-proxy ingestion", Label(utils.ConfigProcessingWindowsKubeProxy), func() {
	It("observes the actual node endpoint and fresh healthy per-node AMW samples", func() {
		expectWindowsKubeProxy(transitionAPI(), transitionCluster(), time.Now())
	})
})

func TestSettingsTransitionGuards(t *testing.T) {
	now := time.Now()
	valid := transitionTarget{ScrapePool: "expected/0", Health: "down", LastError: "HTTP401", LastScrape: now}
	if err := validateTransitionTargets([]transitionTarget{valid}, []string{"expected/0"}, "down", now); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []transitionTarget{
		{ScrapePool: "other/0", Health: "down", LastError: "HTTP401", LastScrape: now},
		{ScrapePool: "expected/0", Health: "up", LastError: "HTTP401", LastScrape: now},
		{ScrapePool: "expected/0", Health: "down", LastError: "connection refused", LastScrape: now},
		{ScrapePool: "expected/0", Health: "down", LastError: "HTTP401", LastScrape: now.Add(-time.Second)},
	} {
		if err := validateTransitionTargets([]transitionTarget{bad}, []string{"expected/0"}, "down", now); err == nil {
			t.Fatalf("wrong identity/health/auth/freshness must fail: %+v", bad)
		}
	}
	if err := validateTransitionTargets(nil, []string{"expected/0"}, "up", now); err == nil {
		t.Fatal("empty targets must fail")
	}
	for _, test := range []struct{ cluster, mac, want string }{
		{"/subscriptions/s/resourcegroups/g/providers/p/clusters/real", "true", "real"},
		{"Arc-Cluster.Unmodified", "false", "Arc-Cluster.Unmodified"},
		{"raw/Arc-Cluster.Unmodified", "false", "raw/Arc-Cluster.Unmodified"},
	} {
		got, err := fallbackCluster(test.cluster, test.mac)
		if err != nil || got != test.want {
			t.Fatalf("independent cluster identity: got=%q err=%v want=%q", got, err, test.want)
		}
	}
	if _, err := fallbackCluster("", "true"); err == nil {
		t.Fatal("missing independent identity must fail")
	}
	if err := validateAnnotationConfig(nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := validateAnnotationConfig(nil, "^ama-settings-included$"); err == nil {
		t.Fatal("missing admitted annotation job must fail")
	}
	if err := validateAnnotationConfig([]*config.ScrapeConfig{{JobName: "kubernetes-pods"}}, ""); err == nil {
		t.Fatal("retained annotation job after removal must fail")
	}
	cfg, err := config.Load(`
scrape_configs:
  - job_name: kubernetes-pods
    relabel_configs:
      - source_labels: [__meta_kubernetes_namespace]
        action: keep
        regex: ^ama-settings-included$
`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateAnnotationConfig(cfg.ScrapeConfigs, "^ama-settings-included$"); err != nil {
		t.Fatal(err)
	}
	if err := validateAnnotationConfig(cfg.ScrapeConfigs, "^ama-settings-excluded$"); err == nil {
		t.Fatal("stale annotation namespace selector must fail")
	}
	cfg.ScrapeConfigs[0].RelabelConfigs[0].Regex = relabel.MustNewRegexp("")
	if err := validateAnnotationConfig(cfg.ScrapeConfigs, ""); err == nil {
		t.Fatal("an empty namespace selector does not remove the annotation job")
	}
}

func TestAnnotationTargetSet(t *testing.T) {
	now := time.Now()
	target := func(namespace, pod string) transitionTarget {
		return transitionTarget{
			ScrapePool: "kubernetes-pods", Health: "up", LastScrape: now,
			Labels:           map[string]string{"kubernetes_namespace": namespace, "kubernetes_pod_name": pod},
			DiscoveredLabels: map[string]string{"__meta_kubernetes_namespace": namespace, "__meta_kubernetes_pod_name": pod},
		}
	}
	allowed := []string{"ama-settings-included", "ama-settings-excluded"}
	valid := []transitionTarget{target(allowed[0], "annotation-target"), target(allowed[1], "annotation-target")}
	if err := validateAnnotationTargets(valid, allowed, now); err != nil {
		t.Fatal(err)
	}
	wildcard := append(slices.Clone(valid), target("ama-settings-included-similar", "annotation-target"),
		target("unrelated-workload", "annotation-target"))
	if err := validateAnnotationTargets(wildcard, append(slices.Clone(allowed), "ama-settings-included-similar"), now); err != nil {
		t.Fatal(err)
	}
	unhealthy := slices.Clone(valid)
	unhealthy[1].Health = "down"
	stale := slices.Clone(valid)
	stale[1].LastScrape = now.Add(-time.Second)
	missingIdentity := slices.Clone(valid)
	missingIdentity[1].Labels = nil
	for _, test := range []struct {
		name    string
		targets []transitionTarget
		allowed []string
	}{
		{"missing-all", nil, allowed},
		{"missing-second-alternative", valid[:1], allowed},
		{"similar-prefix", wildcard, allowed},
		{"unannotated", append(slices.Clone(valid), target(allowed[0], "unannotated-target")), allowed},
		{"unhealthy-second", unhealthy, allowed},
		{"stale-second", stale, allowed},
		{"missing-exported-identity", missingIdentity, allowed},
		{"missing-exported-identity-disabled", missingIdentity[1:], nil},
		{"retained-after-disabled", valid, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := validateAnnotationTargets(test.targets, test.allowed, now); err == nil {
				t.Fatal("wrong fixture target set must fail")
			}
		})
	}
	if err := validateAnnotationTargets(nil, nil, now); err != nil {
		t.Fatal(err)
	}
}
