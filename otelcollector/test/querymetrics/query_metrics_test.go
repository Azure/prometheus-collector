package querymetrics

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"prometheus-collector/otelcollector/test/utils"
)

var _ = Describe("Query Metrics Test Suite", func() {
	Context("when querying metrics", func() {
		It("should return the expected results", func() {
			client, err := utils.CreatePrometheusAPIClient()
			Expect(err).NotTo(HaveOccurred())
			Expect(client).NotTo(BeNil())
			_, err = utils.RunQuery(client, "up")
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

