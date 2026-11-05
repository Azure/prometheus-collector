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
	return utils.FreshMetricQuery(fmt.Sprintf("%s{job=%q,cluster=%q}", metric, job, ExpectedCluster), time.Time{})
}

var _ = AfterSuite(func() {
	By("tearing down the test environment")
})
