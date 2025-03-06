package configprocessing

// import (
// 	"prometheus-collector/otelcollector/test/utils"

// 	. "github.com/onsi/ginkgo/v2"
// 	. "github.com/onsi/gomega"
// )

// /*
// - For each of the pods that we deploy in our chart, ensure each container within that pod doesn't have errors in the logs.
// - The replicaset, daemonset, and kube-state-metrics are always deployed.
// - The operator-targets and node-exporter workloads are checked if the 'operator' or 'arc-extension' label is included in the test run.
// - The label and values are provided to get a list of pods only with that label.
// */
// var _ = DescribeTable("The container logs for replicaset and config-reader should contain errors",
// 	func(namespace string, controllerLabelName string, controllerLabelValue string) {
// 		err := utils.CheckContainerLogsForErrors(K8sClient, namespace, controllerLabelName, controllerLabelValue)
// 		Expect(err).HaveOccurred()
// 	},
// 	Entry("when checking the ama-metrics-win-node", "kube-system", "dsName", "ama-metrics-win-node", Label(utils.WindowsLabel)),
// )
