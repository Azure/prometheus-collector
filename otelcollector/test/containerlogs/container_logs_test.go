package containerlogs

import (
	"prometheus-collector/otelcollector/test/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = DescribeTable("All processes are running",
	func(namespace, labelName, labelValue, containerName string, processes []string) {
		err := utils.CheckAllProcessesRunning(K8sClient, Cfg, labelName, labelValue, namespace, containerName, processes)
		Expect(err).NotTo(HaveOccurred())
	},
	Entry("when checking the ama-metrics replica pod(s)", "kube-system", "rsName", "ama-metrics", "prometheus-collector",
		[]string {
			"fluent-bit",
			"telegraf",
			"otelcollector",
			"mdsd -a -A -e",
			"MetricsExtension",
			"inotifywait /etc/config/settings",
			"inotifywait /etc/mdsd.d",
			"crond",
		},
	),
	Entry("when checking the ama-metrics-node daemonset pods", "kube-system", "dsName", "ama-metrics", "prometheus-collector",
	[]string {
		"fluent-bit",
		"telegraf",
		"otelcollector",
		"mdsd -a -A -e",
		"MetricsExtension",
		"inotifywait /etc/config/settings",
		"inotifywait /etc/mdsd.d",
		"crond",
	},
),
)

var _ = DescribeTable("The container logs should not contain errors",
	func(namespace string, controllerLabelName string, controllerLabelValue string) {
		err := utils.CheckContainerLogsForErrors(K8sClient, namespace, controllerLabelName, controllerLabelValue)
		Expect(err).NotTo(HaveOccurred())
	},
	Entry("when checking the ama-metrics replica pods", "kube-system", "rsName", "ama-metrics"),
	Entry("when checking the ama-metrics-node", "kube-system", "dsName", "ama-metrics-node"),
	Entry("when checking the ama-metrics-operator-targets pod", "kube-system", "rsName", "ama-metrics-operator-targets"),
	Entry("when checking the ama-metrics-ksm pod", "kube-system", "rsName", "ama-metrics-ksm"),
)
