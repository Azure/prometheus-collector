package crds

import (
	"testing"

	"k8s.io/client-go/kubernetes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	promOperatorClient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"k8s.io/client-go/rest"

	"prometheus-collector/otelcollector/test/utils"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var K8sClient 	*kubernetes.Clientset
var Cfg       	*rest.Config
var PromClient  promOperatorClient.Interface

func TestCRDs(t *testing.T) {
  RegisterFailHandler(Fail)

  RunSpecs(t, "CRDs Test Suite")
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
