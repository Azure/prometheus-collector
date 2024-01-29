package livenessprobe

import (
	"time"

	"prometheus-collector/otelcollector/test/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = DescribeTable("The liveness probe should restart the replica pod", Ordered,
 	func(namespace, labelName, labelValue, containerName, terminatedMessage, processName string, timeout int64) {
 		err := utils.CheckLivenessProbeRestartForProcess(K8sClient, Cfg, labelName, labelValue, namespace, containerName, terminatedMessage, processName, timeout)
 		Expect(err).NotTo(HaveOccurred())

		// Wait for all processes in pod to start up before running any other tests
		time.Sleep(90 * time.Second)
 	},
 	Entry("when otelcollector is not running", "kube-system", "rsName", "ama-metrics", "prometheus-collector", "OpenTelemetryCollector is not running", "otelcollector", int64(120)),
	Entry("when MetricsExtension is not running", "kube-system", "rsName", "ama-metrics", "prometheus-collector", "Metrics Extension is not running (configuration exists)", "MetricsExtension", int64(120)),
	Entry("when mdsd is not running", "kube-system", "rsName", "ama-metrics", "prometheus-collector", "mdsd is not running (configuration exists)", "mdsd -a -A -e", int64(120)),
)

var _ = DescribeTable("The liveness probe should restart the daemonset pod", Ordered,
 	func(namespace, labelName, labelValue, containerName, terminatedMessage, processName string, timeout int64) {
 		err := utils.CheckLivenessProbeRestartForProcess(K8sClient, Cfg, labelName, labelValue, namespace, containerName, terminatedMessage, processName, timeout)
 		Expect(err).NotTo(HaveOccurred())

		// Wait for all processes in pod to start up before running any other tests
		time.Sleep(90 * time.Second)
 	},
 	Entry("when otelcollector is not running", "kube-system", "dsName", "ama-metrics-node", "prometheus-collector", "OpenTelemetryCollector is not running", "otelcollector", int64(120)),
	Entry("when MetricsExtension is not running", "kube-system", "dsName", "ama-metrics-node", "prometheus-collector", "Metrics Extension is not running (configuration exists)", "MetricsExtension", int64(120)),
	Entry("when mdsd is not running", "kube-system", "dsName", "ama-metrics-node", "prometheus-collector", "mdsd is not running (configuration exists)", "mdsd -a -A -e", int64(120)),
)
