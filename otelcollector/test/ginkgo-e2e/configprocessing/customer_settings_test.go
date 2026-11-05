package configprocessing

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/prometheus/config"
	_ "github.com/prometheus/prometheus/discovery/install"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"prometheus-collector/otelcollector/test/utils"
)

var _ = Describe("Customer ConfigMap identity and annotation selection",
	Label("config-processing-customer-settings-v1", "config-processing-customer-settings-v2"), func() {
		DescribeTable("normalizes the alias in every running Linux collector",
			func(labelKey, labelValue string) {
				Eventually(func(g Gomega) {
					pods, err := utils.GetPodsWithLabel(K8sClient, "kube-system", labelKey, labelValue)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(pods).NotTo(BeEmpty())
					for _, pod := range pods {
						// Inspect the running collector, not an exec shell's startup environment.
						out, stderr, err := utils.ExecCmd(K8sClient, Cfg, pod.Name, "prometheus-collector", "kube-system",
							[]string{"sh", "-c", `for pid in $(pidof otelcollector); do tr '\000' '\n' < /proc/$pid/environ | grep '^AZMON_CLUSTER_LABEL='; done`})
						g.Expect(err).NotTo(HaveOccurred(), stderr)
						g.Expect(strings.Fields(out)).To(ConsistOf("AZMON_CLUSTER_LABEL=Customer_Config_POC"), pod.Name)
					}
				}, 8*time.Minute, 15*time.Second).Should(Succeed())
			},
			Entry("ReplicaSet", "rsName", "ama-metrics"),
			Entry("Linux DaemonSet", "dsName", "ama-metrics-node"),
		)

		DescribeTable("honors annotation settings in every replica's effective scrape job", func(setting string) {
			Eventually(func(g Gomega) {
				responses, err := utils.QueryPromUIFromPods(K8sClient, Cfg, "kube-system", "rsName", "ama-metrics",
					"prometheus-collector", "/api/v1/status/config", true)
				g.Expect(err).NotTo(HaveOccurred())
				for _, response := range responses {
					found := false
					var result promv1.ConfigResult
					g.Expect(json.Unmarshal(response.Data, &result)).To(Succeed())
					cfg, err := config.Load(result.YAML, nil)
					g.Expect(err).NotTo(HaveOccurred())
					for _, job := range cfg.ScrapeConfigs {
						if job.JobName != "kubernetes-pods" {
							continue
						}
						found = true
						namespaceFilter, keepList := false, false
						for _, rule := range job.RelabelConfigs {
							if string(rule.Action) == "keep" && rule.SourceLabels.String() == "__meta_kubernetes_namespace" && rule.Regex.String() == "^ama-settings-included$" {
								namespaceFilter = true
							}
						}
						for _, rule := range job.MetricRelabelConfigs {
							if string(rule.Action) == "keep" && rule.SourceLabels.String() == "__name__" && rule.Regex.MatchString("weather_temperature") && !rule.Regex.MatchString("unrelated_metric") {
								keepList = true
							}
						}
						switch setting {
						case "interval":
							g.Expect(time.Duration(job.ScrapeInterval)).To(Equal(45 * time.Second))
						case "namespace":
							g.Expect(namespaceFilter).To(BeTrue(), "missing exact namespace selector")
						case "keep-list":
							g.Expect(keepList).To(BeTrue(), "missing annotation metric keep-list")
						default:
							Fail("unknown annotation setting: " + setting)
						}
					}
					g.Expect(found).To(BeTrue(), "%s has no kubernetes-pods job", response.PodName)
				}
			}, 8*time.Minute, 15*time.Second).Should(Succeed())
		},
			Entry("namespace", "namespace"),
			Entry("interval", "interval"),
			Entry("metric keep-list", "keep-list"),
		)

		It("discovers the included annotated pod but never scrapes the excluded pod", func() {
			Eventually(func(g Gomega) {
				for _, ns := range []string{"ama-settings-included", "ama-settings-excluded"} {
					pod, err := K8sClient.CoreV1().Pods(ns).Get(context.Background(), "annotation-target", metav1.GetOptions{})
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(string(pod.Status.Phase)).To(Equal("Running"), ns)
					g.Expect(pod.Status.PodIP).NotTo(BeEmpty(), ns)
				}
				responses, err := utils.QueryPromUIFromPods(K8sClient, Cfg, "kube-system", "rsName", "ama-metrics",
					"prometheus-collector", "/api/v1/targets", true)
				g.Expect(err).NotTo(HaveOccurred())
				included, excluded := false, false
				for _, response := range responses {
					var result struct {
						ActiveTargets []struct {
							ScrapePool string            `json:"scrapePool"`
							Labels     map[string]string `json:"labels"`
							Health     string            `json:"health"`
							LastScrape time.Time         `json:"lastScrape"`
						} `json:"activeTargets"`
					}
					g.Expect(json.Unmarshal(response.Data, &result)).To(Succeed())
					for _, target := range result.ActiveTargets {
						if target.ScrapePool != "kubernetes-pods" || target.Labels["kubernetes_pod_name"] != "annotation-target" {
							continue
						}
						switch target.Labels["kubernetes_namespace"] {
						case "ama-settings-included":
							included = included || target.Health == "up"
							g.Expect(target.LastScrape).To(BeTemporally(">", time.Now().Add(-5*time.Minute)))
						case "ama-settings-excluded":
							excluded = true
						}
					}
				}
				g.Expect(included).To(BeTrue(), "included annotation target must be discovered and healthy")
				g.Expect(excluded).To(BeFalse(), "excluded annotation target must not be scraped")
			}, 8*time.Minute, 15*time.Second).Should(Succeed())
		})
	})
