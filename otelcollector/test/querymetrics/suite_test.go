package querymetrics

import (
	"os"
	"testing"

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
  K8sClient 							*kubernetes.Clientset
  Cfg       							*rest.Config
  PrometheusQueryClient   v1.API
)

func TestQueryingMetrics(t *testing.T) {
  RegisterFailHandler(Fail)

  RunSpecs(t, "Query Metrics Test Suite")
}

var _ = BeforeSuite(func() {
  var err error
  K8sClient, Cfg, err = utils.SetupKubernetesClient()
  Expect(err).NotTo(HaveOccurred())

  amwQueryEndpoint := os.Getenv("AMW_QUERY_ENDPOINT")
  Expect(amwQueryEndpoint).NotTo(BeEmpty())
  clientID := os.Getenv("QUERY_ACCESS_CLIENT_ID")
  Expect(clientID).NotTo(BeEmpty())
  clientSecret := os.Getenv("QUERY_ACCESS_CLIENT_SECRET")
  Expect(clientSecret).NotTo(BeEmpty())
  
  PrometheusQueryClient, err = utils.CreatePrometheusAPIClient(
    amwQueryEndpoint,
    clientID,
    clientSecret,
  )
  Expect(err).NotTo(HaveOccurred())
  Expect(PrometheusQueryClient).NotTo(BeNil())
})

var _ = AfterSuite(func() {
  By("tearing down the test environment")
})
