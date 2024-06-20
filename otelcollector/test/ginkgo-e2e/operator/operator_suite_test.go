package operator

import (
	"testing"

	"k8s.io/client-go/kubernetes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	promOperatorClient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"k8s.io/client-go/rest"

	"prometheus-collector/otelcollector/test/utils"
)

var K8sClient 	*kubernetes.Clientset
var Cfg       	*rest.Config
var PromClient  promOperatorClient.Interface

/*
 * These tests MUST be run with the flag:
 * -ldflags="-s -X github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring.GroupName=azmonitoring.coreos.com"
 * in order for the prometheus-operator package to get CRs using our custom API group name.
 */
func TestPrometheusOperatorCRs(t *testing.T) {
  RegisterFailHandler(Fail)

  RunSpecs(t, "Prometheus Operator CRs Test Suite")
}

var _ = BeforeSuite(func() {
  var err error
  K8sClient, Cfg, err = utils.SetupKubernetesClient()
  Expect(err).NotTo(HaveOccurred())
	PromClient, err = promOperatorClient.NewForConfig(Cfg)
  Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
  By("tearing down the test environment")
})
