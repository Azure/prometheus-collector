package livenessprobe

import (
	"fmt"
	"prometheus-collector/otelcollector/test/utils"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = DescribeTable("The liveness probe should restart the replica pod", Ordered,
 	func(namespace, labelName, labelValue, containerName, terminatedMessage, processName string, timeout int64) {
		restartCommand := []string{"sh", "-c", fmt.Sprintf("kill -9 $(ps ax | grep \"%s\" | fgrep -v grep | awk '{ print $1 }')", processName)}
 		err := utils.CheckLivenessProbeRestartForProcess(K8sClient, Cfg, labelName, labelValue, namespace, containerName, terminatedMessage, processName, restartCommand, timeout)
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
		restartCommand := []string{"sh", "-c", fmt.Sprintf("kill -9 $(ps ax | grep \"%s\" | fgrep -v grep | awk '{ print $1 }')", processName)}
 		err := utils.CheckLivenessProbeRestartForProcess(K8sClient, Cfg, labelName, labelValue, namespace, containerName, terminatedMessage, processName, restartCommand, timeout)
 		Expect(err).NotTo(HaveOccurred())

		// Wait for all processes in pod to start up before running any other tests
		time.Sleep(90 * time.Second)
 	},
 	Entry("when otelcollector is not running", "kube-system", "dsName", "ama-metrics-node", "prometheus-collector", "OpenTelemetryCollector is not running", "otelcollector", int64(120)),
	Entry("when MetricsExtension is not running", "kube-system", "dsName", "ama-metrics-node", "prometheus-collector", "Metrics Extension is not running (configuration exists)", "MetricsExtension", int64(120)),
	Entry("when mdsd is not running", "kube-system", "dsName", "ama-metrics-node", "prometheus-collector", "mdsd is not running (configuration exists)", "mdsd -a -A -e", int64(120)),
)

// var _ = DescribeTable("The liveness probe should restart the windows pod", Ordered,
//  	func(namespace, labelName, labelValue, containerName, terminatedMessage, processName string, timeout int64) {
// 		restartCommand := []string{"powershell", fmt.Sprintf("get-process \"%s\" | stop-process", processName)}
//  		err := utils.CheckLivenessProbeRestartForProcess(K8sClient, Cfg, labelName, labelValue, namespace, containerName, terminatedMessage, processName, restartCommand, timeout)
//  		Expect(err).NotTo(HaveOccurred())

// 		// Wait for all processes in pod to start up before running any other tests
// 		time.Sleep(180 * time.Second)
//  	},
//  	Entry("when otelcollector is not running", "kube-system", "dsName", "ama-metrics-win-node", "prometheus-collector", "otelcollector is not running", "otelcollector", int64(300)),
// 	Entry("when MetricsExtension is not running", "kube-system", "dsName", "ama-metrics-win-node", "prometheus-collector", "Metrics Extension is not running (configuration exists)", "MetricsExtension.Native", int64(300)),
// 	Entry("when mdsd is not running", "kube-system", "dsName", "ama-metrics-win-node", "prometheus-collector", "MonAgentLauncher is not running (configuration exists)", "MonAgentLauncher", int64(300)),
// )

// var _ = Describe("The liveness probe should restart the pods", func() {
// 	It("when the settings configmap is updated", func() {
// 		err := utils.GetAndUpdateConfigMap(K8sClient)
// 		Expect(err).NotTo(HaveOccurred())
// 		pods, err := GetPodsWithLabel(K8sClient, namespace, labelName, labelValue)
// 		if err != nil {
// 			return err
// 		}
// 		for _, pod := range pods.Items {

// 	})

// 	It("when the prometheus config for the replicaset is updated", func() {
// 		err := utils.GetAndUpdatePrometheusConfig(K8sClient)
// 		Expect(err).NotTo(HaveOccurred())
// 	})

// 	It("when the prometheus config for the daemonset is updated", func() {
// 		err := utils.GetAndUpdatePrometheusConfig(K8sClient)
// 		Expect(err).NotTo(HaveOccurred())
// 	})
// })
