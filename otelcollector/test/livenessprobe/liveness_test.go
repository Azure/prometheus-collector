package livenessprobe

import (
	"fmt"
	"prometheus-collector/otelcollector/test/utils"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = When("the liveness probe", Ordered, func() {

  // An update to ama-metrics-settings-configmap causes all pods to restart. This needs to be run separately from the other tests.
  Context("detects that the ama-metrics-settings-configmap has been updated", func() {
    It("should restart all pods", func() {
    })
  })

  // All other liveness events can be run in parallel for the replicaset, daemonset, and windows daemonset.
  Describe("dectects that", func() {

    // All events that restart the replicaset pods need to be run sequentially to not conflict.
    Context("on the replicaset prometheus-collector container", Ordered, func() {

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
          "mdsd is not running (configuration exists)", "mdsd -a -A -e", int64(120),
        ),
      )

      Specify("the ama-metrics-prometheus-config configmap has updated, the container should restart", func() {
        err := utils.GetAndUpdateConfigMap(K8sClient, "ama-metrics-prometheus-config", "kube-system")
        Expect(err).NotTo(HaveOccurred())
        err = utils.WatchForPodRestart(K8sClient, "kube-system", "rsName", "ama-metrics", 120, "prometheus-collector",
          "inotifyoutput.txt has been updated - config changed",
        )
      })
    })

    Describe("on the daemonset prometheus-collector container", Ordered, func() {
      DescribeTable("the process", Ordered,
        func(namespace, labelName, labelValue, containerName, terminatedMessage, processName string, timeout int64) {
          restartCommand := []string{"sh", "-c", fmt.Sprintf("kill -9 $(ps ax | grep \"%s\" | fgrep -v grep | awk '{ print $1 }')", processName)}
          err := utils.CheckLivenessProbeRestartForProcess(K8sClient, Cfg, labelName, labelValue, namespace, containerName, terminatedMessage, processName, restartCommand, timeout)
          Expect(err).NotTo(HaveOccurred())

          // Wait for all processes in pod to start up before running any other tests
          time.Sleep(120 * time.Second)
        },
        Entry("otelcollector is not running, the container should restart", "kube-system", "dsName", "ama-metrics-node", "prometheus-collector",
          "OpenTelemetryCollector is not running", "otelcollector", int64(120),
        ),
        Entry("MetricsExtension is not running, the container should restart", "kube-system", "dsName", "ama-metrics-node", "prometheus-collector",
          "Metrics Extension is not running (configuration exists)", "MetricsExtension", int64(120),
        ),
        Entry("mdsd is not running, the container should restart", "kube-system", "dsName", "ama-metrics-node", "prometheus-collector",
          "mdsd is not running (configuration exists)", "mdsd -a -A -e", int64(120),
        ),
      )

      It("the ama-metrics-config-node configmap has updated, the container should restart", Label(utils.LinuxDaemonsetCustomConfig), func() {
        err := utils.GetAndUpdateConfigMap(K8sClient, "ama-metrics-prometheus-config-node", "kube-system")
        Expect(err).NotTo(HaveOccurred())
        err = utils.WatchForPodRestart(K8sClient, "kube-system", "dsName", "ama-metrics-node", 120, "prometheus-collector",
          "inotifyoutput.txt has been updated - config changed",
        )
      })
    })

    var _ = Describe("on the windows prometheus-collector container", Ordered, Label(utils.WindowsLabel), func() {
      DescribeTable("the process", Ordered,
        func(namespace, labelName, labelValue, containerName, terminatedMessage, processName string, timeout int64) {
          restartCommand := []string{"powershell", fmt.Sprintf("get-process \"%s\" | stop-process", processName)}
          err := utils.CheckLivenessProbeRestartForProcess(K8sClient, Cfg, labelName, labelValue, namespace, containerName, terminatedMessage, processName, restartCommand, timeout)
          Expect(err).NotTo(HaveOccurred())

          // Wait for all processes in pod to start up before running any other tests
          time.Sleep(180 * time.Second)
        },
        Entry("when otelcollector is not running, the container should restart", "kube-system", "dsName", "ama-metrics-win-node", "prometheus-collector",
          "", "otelcollector", int64(300),
        ),
        Entry("when MetricsExtension is not running, the container should restart", "kube-system", "dsName", "ama-metrics-win-node", "prometheus-collector",
          "", "MetricsExtension.Native", int64(300),
        ),
        Entry("when mdsd is not running, the container should restart", "kube-system", "dsName", "ama-metrics-win-node", "prometheus-collector",
          "", "MonAgentLauncher", int64(300),
        ),
      )

      It("the prometheus config for the windows daemonset has updated, the container should restart", func() {
        err := utils.GetAndUpdateConfigMap(K8sClient, "ama-metrics-prometheus-config-node-windows", "kube-system")
        Expect(err).NotTo(HaveOccurred())
        err = utils.WatchForPodRestart(K8sClient, "kube-system", "dsName", "ama-metrics-win-node", 200, "prometheus-collector",
          "inotifyoutput.txt has been updated - config changed",
        )
      })
    })
  })
})
