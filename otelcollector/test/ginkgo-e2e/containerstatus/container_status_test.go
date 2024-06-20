package containerstatus

import (
	"prometheus-collector/otelcollector/test/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

/*
 * For each of the pods that we deploy in our chart, ensure each container within that pod has status 'Running'.
 * The replicaset, daemonset, and kube-state-metrics are always deployed.
 * The operator-targets and node-exporter workloads are checked if the 'operator' or 'arc-extension' label is included in the test run.
 * The label and values are provided to get a list of pods only with that label.
 */
var _ = DescribeTable("The containers should be running",
	func(namespace string, controllerLabelName string, controllerLabelValue string) {
		err := utils.CheckIfAllContainersAreRunning(K8sClient, namespace, controllerLabelName, controllerLabelValue)
		Expect(err).NotTo(HaveOccurred())
	},
	Entry("when checking the ama-metrics replica pod(s)", "kube-system", "rsName", "ama-metrics"),
	Entry("when checking the ama-metrics-node", "kube-system", "dsName", "ama-metrics-node"),
	Entry("when checking the ama-metrics-win-node pod", "kube-system", "dsName", "ama-metrics-win-node", Label(utils.WindowsLabel)),
	Entry("when checking the ama-metrics-ksm pod", "kube-system", "app.kubernetes.io/name", "ama-metrics-ksm"),
	Entry("when checking the ama-metrics-operator-targets pod", "kube-system", "rsName", "ama-metrics-operator-targets", Label(utils.OperatorLabel)),
	Entry("when checking the prometheus-node-exporter pod", "kube-system", "app", "prometheus-node-exporter", Label(utils.ArcExtensionLabel)),
)

/*
 * For each of the DS pods that we deploy in our chart, ensure that all nodes have been used to schedule these pods.
 * The label and values are provided to get a list of pods only with that label.
 * The osLabel is provided to check on all DS pods based on the OS.
 */
var _ = DescribeTable("The pods should be scheduled in all nodes",
	func(namespace string, controllerLabelName string, controllerLabelValue string, osLabel string) {
		err := utils.CheckIfAllPodsScheduleOnNodes(K8sClient, namespace, controllerLabelName, controllerLabelValue, osLabel)
		Expect(err).NotTo(HaveOccurred())
	},
	Entry("when checking the ama-metrics-node", "kube-system", "dsName", "ama-metrics-node", "linux"),
	Entry("when checking the ama-metrics-win-node pod", "kube-system", "dsName", "ama-metrics-win-node", "windows", Label(utils.WindowsLabel)),
)

/*
 * For each of the DS pods that we deploy in our chart, ensure that all specific nodes like ARM64,FIPS have been used to schedule these pods.
 * The label and values are provided to get a list of pods only with that label.
 */
var _ = DescribeTable("The pods should be scheduled in all Fips and ARM64 nodes",
	func(namespace string, controllerLabelName string, controllerLabelValue string, nodeLabelKey string, nodeLabelValue string) {
		err := utils.CheckIfAllPodsScheduleOnSpecificNodesLabels(K8sClient, namespace, controllerLabelName, controllerLabelValue, nodeLabelKey, nodeLabelValue)
		Expect(err).NotTo(HaveOccurred())
	},
	Entry("when checking the ama-metrics-node", "kube-system", "dsName", "ama-metrics-node", "kubernetes.azure.com/fips_enabled", "true", Label(utils.FIPSLabel)),
	Entry("when checking the ama-metrics-win-node pod", "kube-system", "dsName", "ama-metrics-win-node", "kubernetes.azure.com/fips_enabled", "true", Label(utils.WindowsLabel), Label(utils.FIPSLabel)),
	Entry("when checking the ama-metrics-node", "kube-system", "dsName", "ama-metrics-node", "kubernetes.io/arch", "arm64", Label(utils.ARM64Label)),
)

/*
 * For each of the pods that have the prometheus-collector container, check all expected processes are running.
 * The linux replicaset and daemonset will should have the same processes running.
 */
var _ = DescribeTable("All processes are running",
	func(namespace, labelName, labelValue, containerName string, processes []string) {
		err := utils.CheckAllProcessesRunning(K8sClient, Cfg, labelName, labelValue, namespace, containerName, processes)
		Expect(err).NotTo(HaveOccurred())
	},
	Entry("when checking the ama-metrics replica pod(s)", "kube-system", "rsName", "ama-metrics", "prometheus-collector",
		[]string{
			"fluent-bit",
			"otelcollector",
			"mdsd -a -A -e",
			"MetricsExtension",
			"inotifywait /etc/config/settings",
			"inotifywait /etc/mdsd.d",
			"crond",
		},
	),
	Entry("when checking the ama-metrics-node daemonset pods", "kube-system", "dsName", "ama-metrics-node", "prometheus-collector",
		[]string{
			"fluent-bit",
			"otelcollector",
			"mdsd -a -A -e",
			"MetricsExtension",
			"inotifywait /etc/config/settings",
			"inotifywait /etc/mdsd.d",
			"crond",
		},
	),
)

/*
 * For windows daemonset pods that have the prometheus-collector container, check all expected processes are running.
 */
var _ = DescribeTable("All processes are running",
	func(namespace, labelName, labelValue, containerName string, processes []string) {
		err := utils.CheckAllWindowsProcessesRunning(K8sClient, Cfg, labelName, labelValue, namespace, containerName, processes)
		Expect(err).NotTo(HaveOccurred())
	},
	Entry("when checking the ama-metrics-win-node daemonset pods", "kube-system", "dsName", "ama-metrics-win-node", "prometheus-collector",
		[]string{
			"fluent-bit",
			"otelcollector",
			"MetricsExtension",
			"MonAgentLauncher",
			"MonAgentHost",
			"MonAgentManager",
			"MonAgentCore",
		},
		Label(utils.WindowsLabel),
		FlakeAttempts(3),
	),
)

/*
- For each of the pods that we deploy in our chart, ensure each container within that pod doesn't have errors in the logs.
- The replicaset, daemonset, and kube-state-metrics are always deployed.
- The operator-targets and node-exporter workloads are checked if the 'operator' or 'arc-extension' label is included in the test run.
- The label and values are provided to get a list of pods only with that label.
*/
var _ = DescribeTable("The container logs should not contain errors",
	func(namespace string, controllerLabelName string, controllerLabelValue string) {
		err := utils.CheckContainerLogsForErrors(K8sClient, namespace, controllerLabelName, controllerLabelValue)
		Expect(err).NotTo(HaveOccurred())
	},
	Entry("when checking the ama-metrics replica pods", "kube-system", "rsName", "ama-metrics"),
	Entry("when checking the ama-metrics-node", "kube-system", "dsName", "ama-metrics-node"),
	Entry("when checking the ama-metrics replica pods", "kube-system", "rsName", "ama-metrics", Label(utils.ARM64Label)),
	Entry("when checking the ama-metrics-node", "kube-system", "dsName", "ama-metrics-node", Label(utils.ARM64Label)),
	Entry("when checking the ama-metrics-win-node", "kube-system", "dsName", "ama-metrics-win-node", Label(utils.WindowsLabel)),
	Entry("when checking the ama-metrics-ksm pod", "kube-system", "app.kubernetes.io/name", "ama-metrics-ksm"),
	Entry("when checking the ama-metrics-operator-targets pod", "kube-system", "rsName", "ama-metrics-operator-targets", Label(utils.OperatorLabel)),
	Entry("when checking the prometheus-node-exporter pod", "kube-system", "app", "prometheus-node-exporter", Label(utils.ArcExtensionLabel)),
)
