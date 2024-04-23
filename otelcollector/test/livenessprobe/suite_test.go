package livenessprobe

import (
	"testing"

	"k8s.io/client-go/kubernetes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/rest"

	"prometheus-collector/otelcollector/test/utils"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var K8sClient 	*kubernetes.Clientset
var Cfg       	*rest.Config

func TestLivenessProbe(t *testing.T) {
  RegisterFailHandler(Fail)

  RunSpecs(t, "Liveness Probe Test Suite")
}

var _ = BeforeSuite(func() {
  var err error
  K8sClient, Cfg, err = utils.SetupKubernetesClient()
  Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
  By("tearing down the test environment")
})
