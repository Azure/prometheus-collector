package livenessprobe

import (
	"fmt"
	"prometheus-collector/otelcollector/test/utils"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// All events that restart the replicaset pods need to be run sequentially to not conflict.
var _ = Describe("When replicaset prometheus-collector container liveness probe detects that", Ordered, func() {
	// Check restarts for each process not running.
	DescribeTable("the process", Ordered,
		func(namespace, labelName, labelValue, containerName, terminatedMessage, processName string, timeout int64) {
			restartCommand := []string{
				"sh",
				"-c",
				fmt.Sprintf("kill -9 $(ps ax | grep \"%s\" | fgrep -v grep | awk '{ print $1 }')", processName),
			}
			err := utils.CheckLivenessProbeRestartForProcess(K8sClient, Cfg, labelName, labelValue, namespace, containerName, terminatedMessage, processName, restartCommand, timeout)
			Expect(err).NotTo(HaveOccurred())

			// Wait for all processes in pod to start up before running any other tests
			time.Sleep(120 * time.Second)
		},
		Entry("otelcollector is not running, the container should restart", "kube-system", "rsName", "ama-metrics", "prometheus-collector",
			"OpenTelemetryCollector is not running", "otelcollector", int64(120),
		),
		Entry("MetricsExtension is not running, the container should restart", "kube-system", "rsName", "ama-metrics", "prometheus-collector",
			"Metrics Extension is not running (configuration exists)", "MetricsExtension", int64(120),
		),
		Entry("mdsd is not running, the container should restart", "kube-system", "rsName", "ama-metrics", "prometheus-collector",
			"mdsd is not running (configuration exists)", "mdsd -a -A -e", int64(120), Label(utils.MDSDLabel),
		),
	)

	Specify("the ama-metrics-prometheus-config configmap has updated, the container should restart", func() {
		err := utils.GetAndUpdateConfigMap(K8sClient, "ama-metrics-prometheus-config", "kube-system")
		Expect(err).NotTo(HaveOccurred())
		err = utils.WatchForPodRestart(K8sClient, "kube-system", "rsName", "ama-metrics", 120, "prometheus-collector",
			"inotifyoutput.txt has been updated - config changed",
		)
		Expect(err).NotTo(HaveOccurred())
		time.Sleep(120 * time.Second)
	})

	Specify("the TokenConfig.json file has updated from mdsd, the container should restart", Label(utils.MDSDLabel), func() {
		err := utils.GetAndUpdateTokenConfig(K8sClient, Cfg, "kube-system", "rsName", "ama-metrics", "prometheus-collector", []string{"bash", "-c", "echo ' ' >> /etc/mdsd.d/config-cache/metricsextension/TokenConfig.json"})
		Expect(err).NotTo(HaveOccurred())
		err = utils.WatchForPodRestart(K8sClient, "kube-system", "rsName", "ama-metrics", 180, "prometheus-collector",
			"inotifyoutput-mdsd-config.txt has been updated - mdsd config changed",
		)
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = Describe("When the daemonset prometheus-collector container liveness probe detects that", Ordered, func() {
	DescribeTable("the process", Ordered,
		func(namespace, labelName, labelValue, containerName, terminatedMessage, processName string, timeout int64) {
			restartCommand := []string{"sh", "-c", fmt.Sprintf("kill -9 $(ps ax | grep \"%s\" | fgrep -v grep | awk '{ print $1 }')", processName)}
			err := utils.CheckLivenessProbeRestartForProcess(K8sClient, Cfg, labelName, labelValue, namespace, containerName, terminatedMessage, processName, restartCommand, timeout)
			Expect(err).NotTo(HaveOccurred())

			// Wait for all processes in pod to start up before running any other tests
			time.Sleep(180 * time.Second)
		},
		Entry("otelcollector is not running, the container should restart", "kube-system", "dsName", "ama-metrics-node", "prometheus-collector",
			"OpenTelemetryCollector is not running", "otelcollector", int64(180), FlakeAttempts(2),
		),
		Entry("MetricsExtension is not running, the container should restart", "kube-system", "dsName", "ama-metrics-node", "prometheus-collector",
			"Metrics Extension is not running (configuration exists)", "MetricsExtension", int64(180), FlakeAttempts(2),
		),
		Entry("mdsd is not running, the container should restart", "kube-system", "dsName", "ama-metrics-node", "prometheus-collector",
			"mdsd is not running (configuration exists)", "mdsd -a -A -e", int64(180), FlakeAttempts(2), Label(utils.MDSDLabel),
		),
	)

	It("the ama-metrics-config-node configmap has updated, the container should restart", Label(utils.LinuxDaemonsetCustomConfig), func() {
		err := utils.GetAndUpdateConfigMap(K8sClient, "ama-metrics-prometheus-config-node", "kube-system")
		Expect(err).NotTo(HaveOccurred())
		err = utils.WatchForPodRestart(K8sClient, "kube-system", "dsName", "ama-metrics-node", 180, "prometheus-collector",
			"inotifyoutput.txt has been updated - config changed",
		)
		Expect(err).NotTo(HaveOccurred())
		time.Sleep(180 * time.Second)
	})

	Specify("the TokenConfig.json file has updated from mdsd, the container should restart", Label(utils.MdsdLabel), func() {
		err := utils.GetAndUpdateTokenConfig(K8sClient, Cfg, "kube-system", "dsName", "ama-metrics-node", "prometheus-collector", []string{"bash", "-c", "echo ' ' >> /etc/mdsd.d/config-cache/metricsextension/TokenConfig.json"})
		Expect(err).NotTo(HaveOccurred())
		err = utils.WatchForPodRestart(K8sClient, "kube-system", "dsName", "ama-metrics-node", 180, "prometheus-collector",
			"inotifyoutput-mdsd-config.txt has been updated - mdsd config changed",
		)
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = Describe("When the windows prometheus-collector container liveness probe detects that", Ordered, Label(utils.WindowsLabel), func() {
	DescribeTable("the process", Ordered,
		func(namespace, labelName, labelValue, containerName, terminatedMessage, processName string, timeout int64) {
			restartCommand := []string{"powershell", fmt.Sprintf("get-process \"%s\" | stop-process", processName)}
			err := utils.CheckLivenessProbeRestartForProcess(K8sClient, Cfg, labelName, labelValue, namespace, containerName, terminatedMessage, processName, restartCommand, timeout)
			Expect(err).NotTo(HaveOccurred())

			// Wait for all processes in pod to start up before running any other tests
			time.Sleep(240 * time.Second)
		},
		Entry("otelcollector is not running, the container should restart", "kube-system", "dsName", "ama-metrics-win-node", "prometheus-collector",
			"", "otelcollector", int64(300), FlakeAttempts(2),
		),
		Entry("MetricsExtension is not running, the container should restart", "kube-system", "dsName", "ama-metrics-win-node", "prometheus-collector",
			"", "MetricsExtension.Native", int64(300), FlakeAttempts(2),
		),
		Entry("mdsd is not running, the container should restart", "kube-system", "dsName", "ama-metrics-win-node", "prometheus-collector",
			"", "MonAgentLauncher", int64(300), FlakeAttempts(2), Label(utils.MDSDLabel),
		),
	)

	It("the prometheus config for the windows daemonset has updated, the container should restart", func() {
		err := utils.GetAndUpdateConfigMap(K8sClient, "ama-metrics-prometheus-config-node-windows", "kube-system")
		Expect(err).NotTo(HaveOccurred())
		err = utils.WatchForPodRestart(K8sClient, "kube-system", "dsName", "ama-metrics-win-node", 300, "prometheus-collector",
			"",
		)
		Expect(err).NotTo(HaveOccurred())
	})

	Specify("the TokenConfig.json file has updated from mdsd, the container should restart", Label(utils.MdsdLabel), func() {
		err := utils.GetAndUpdateTokenConfig(K8sClient, Cfg, "kube-system", "dsName", "ama-metrics-win-node", "prometheus-collector", []string{"powershell", "-Command", "echo ' ' >> C:\\opt\\genevamonitoringagent\\datadirectory\\mcs\\metricsextension\\TokenConfig.json"})
		Expect(err).NotTo(HaveOccurred())
		err = utils.WatchForPodRestart(K8sClient, "kube-system", "dsName", "ama-metrics-win-node", 300, "prometheus-collector",
			"",
		)
		Expect(err).NotTo(HaveOccurred())
	})
})
