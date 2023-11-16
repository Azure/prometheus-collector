package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = DescribeTable("The container logs should not contain errors",
	func(namespace string, controllerLabelName string, controllerLabelValue string) {
		err := CheckContainerLogsForErrors(namespace, controllerLabelName, controllerLabelValue)
		Expect(err).NotTo(HaveOccurred())
	},
	Entry("when checking for ama-metrics replica pods", "kube-system", "rsName", "ama-metrics"),
	Entry("when checking for ama-metrics-node", "kube-system", "dsName", "ama-metrics-node"),
	Entry("when checking for ama-metrics-operator-targets pod", "kube-system", "rsName", "ama-metrics-operator-targets"),
	Entry("when checking for ama-metrics-ksm pod", "kube-system", "rsName", "ama-metrics-ksm"),
)
