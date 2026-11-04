package querymetrics

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"k8s.io/client-go/kubernetes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"k8s.io/client-go/rest"

	"prometheus-collector/otelcollector/test/utils"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.
var (
	K8sClient             *kubernetes.Clientset
	Cfg                   *rest.Config
	PrometheusQueryClient v1.API
	ExpectedCluster       string
	suiteStarted          time.Time
)

func TestQueryingMetrics(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Query Metrics Test Suite")
}

var _ = BeforeSuite(func() {
	suiteStarted = time.Now()
	var err error
	K8sClient, Cfg, err = utils.SetupKubernetesClient()
	Expect(err).NotTo(HaveOccurred())

	ExpectedCluster = os.Getenv("EXPECTED_CLUSTER")
	if ExpectedCluster == "" {
		pods, err := utils.GetPodsWithLabel(K8sClient, "kube-system", "rsName", "ama-metrics")
		Expect(err).NotTo(HaveOccurred())
		for _, env := range pods[0].Spec.Containers[0].Env {
			if env.Name == "CLUSTER" {
				parts := strings.Split(env.Value, "/")
				ExpectedCluster = strings.Trim(regexp.MustCompile(`[^0-9a-zA-Z]+`).ReplaceAllString(parts[len(parts)-1], "_"), "_")
			}
		}
	}
	Expect(ExpectedCluster).NotTo(BeEmpty(), "EXPECTED_CLUSTER or the collector CLUSTER resource ID is required")
	amwQueryEndpoint := os.Getenv("AMW_QUERY_ENDPOINT")
	Expect(amwQueryEndpoint).NotTo(BeEmpty())

	PrometheusQueryClient, err = utils.CreatePrometheusAPIClient(
		amwQueryEndpoint,
	)
	Expect(err).NotTo(HaveOccurred())
	Expect(PrometheusQueryClient).NotTo(BeNil())
})

func metricQuery(metric, job string) string {
	return freshQuery(fmt.Sprintf("%s{job=%q,cluster=%q}", metric, job, ExpectedCluster), time.Time{})
}

func freshQuery(selector string, notBefore time.Time) string {
	query := fmt.Sprintf("(%s) and (timestamp(%s) > time() - 300) and (timestamp(%s) < time() + 30)",
		selector, selector, selector)
	if !notBefore.IsZero() {
		query += fmt.Sprintf(" and (timestamp(%s) >= %.3f)", selector, float64(notBefore.UnixMilli())/1000)
	}
	return query
}

func TestFreshMetricQuery(t *testing.T) {
	selector := `up{job="kubelet",cluster="expected"}`
	expected := `(up{job="kubelet",cluster="expected"})` +
		` and (timestamp(up{job="kubelet",cluster="expected"}) > time() - 300)` +
		` and (timestamp(up{job="kubelet",cluster="expected"}) < time() + 30)`
	if query := freshQuery(selector, time.Time{}); query != expected {
		t.Fatalf("incorrect source timestamp bounds: %s", query)
	}
	expected += ` and (timestamp(up{job="kubelet",cluster="expected"}) >= 1000.000)`
	if query := freshQuery(selector, time.Unix(1000, 0)); query != expected {
		t.Fatalf("incorrect post-suite timestamp bound: %s", query)
	}
}

var _ = AfterSuite(func() {
	By("tearing down the test environment")
})
