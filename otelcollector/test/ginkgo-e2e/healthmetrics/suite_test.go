package healthmetrics

import (
	"testing"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
	"github.com/prometheus-collector/test/utils"
)

var (
	kubeClient *utils.KubeClient
)

func TestHealthMetrics(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Health Metrics Suite")
}

var _ = ginkgo.BeforeSuite(func() {
	var err error
	kubeClient, err = utils.GetKubeClient()
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
})
