package configmapsettings

import (
	"os"
	"path/filepath"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
	"github.com/prometheus-collector/shared"
	"gopkg.in/yaml.v2"
)

var _ = ginkgo.Describe("When parsing debug mode settings", func() {
	var settings map[string]map[string]string

	ginkgo.BeforeEach(func() {
		oldEnvPath, oldConfigPath := debugModeEnvVarPath, replicaSetCollectorConfig
		ginkgo.DeferCleanup(func() {
			debugModeEnvVarPath, replicaSetCollectorConfig = oldEnvPath, oldConfigPath
		})
		directory := ginkgo.GinkgoT().TempDir()
		debugModeEnvVarPath = filepath.Join(directory, "debug-env")
		replicaSetCollectorConfig = filepath.Join(directory, "collector.yml")
		content, err := os.ReadFile("testdata/collector-config-replicaset.yml")
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(os.WriteFile(replicaSetCollectorConfig, content, 0600)).To(gomega.Succeed())
		ginkgo.GinkgoT().Setenv("AZMON_AGENT_CFG_SCHEMA_VERSION", "v1")
		ginkgo.GinkgoT().Setenv("CONTROLLER_TYPE", "ReplicaSet")
		ginkgo.GinkgoT().Setenv("OS_TYPE", "linux")
		settings = map[string]map[string]string{"debug-mode": {"enabled": "true"}}
	})

	ginkgo.DescribeTable("configures enabled debug mode for the parsed schema and controller",
		func(schema, controller, platform string) {
			ginkgo.GinkgoT().Setenv("AZMON_AGENT_CFG_SCHEMA_VERSION", schema)
			ginkgo.GinkgoT().Setenv("CONTROLLER_TYPE", controller)
			ginkgo.GinkgoT().Setenv("OS_TYPE", platform)
			if schema == "v2" {
				settings = map[string]map[string]string{"prometheus-collector-settings": {"debug-mode": "true"}}
			}
			original, err := os.ReadFile(replicaSetCollectorConfig)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(ConfigureDebugModeSettings(settings)).To(gomega.Succeed())
			content, err := os.ReadFile(debugModeEnvVarPath)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(string(content)).To(gomega.Equal("DEBUG_MODE_ENABLED=true\n"))
			content, err = os.ReadFile(replicaSetCollectorConfig)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			if controller == "ReplicaSet" {
				var config shared.OtelConfig
				gomega.Expect(yaml.Unmarshal(content, &config)).To(gomega.Succeed())
				gomega.Expect(config.Service.Pipelines.Metrics.Exporters).To(gomega.Equal([]interface{}{"otlp_grpc", "prometheus"}))
			} else {
				gomega.Expect(content).To(gomega.Equal(original))
			}
		},
		ginkgo.Entry("v1 Linux replica", "v1", "ReplicaSet", "linux"),
		ginkgo.Entry("v1 Linux daemonset", "v1", "DaemonSet", "linux"),
		ginkgo.Entry("v1 Windows daemonset", "v1", "DaemonSet", "windows"),
		ginkgo.Entry("v2 Linux replica", "v2", "ReplicaSet", "linux"),
		ginkgo.Entry("v2 Linux daemonset", "v2", "DaemonSet", "linux"),
		ginkgo.Entry("v2 Windows daemonset", "v2", "DaemonSet", "windows"),
	)

	ginkgo.It("rejects a missing parsed config map", func() {
		err := ConfigureDebugModeSettings(nil)
		gomega.Expect(err).To(gomega.HaveOccurred())
		gomega.Expect(err.Error()).To(gomega.ContainSubstring("configmap section not mounted"))
	})

	ginkgo.It("leaves debug mode disabled for an unrecognized setting", func() {
		settings["debug-mode"]["enabled"] = "invalid"
		gomega.Expect(ConfigureDebugModeSettings(settings)).To(gomega.Succeed())
		content, err := os.ReadFile(debugModeEnvVarPath)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(string(content)).To(gomega.Equal("DEBUG_MODE_ENABLED=false\n"))
	})

	ginkgo.It("reports an unwritable environment variable file", func() {
		debugModeEnvVarPath = filepath.Join(ginkgo.GinkgoT().TempDir(), "missing", "env")
		err := ConfigureDebugModeSettings(settings)
		gomega.Expect(err).To(gomega.HaveOccurred())
		gomega.Expect(err.Error()).To(gomega.ContainSubstring("Exception while opening file for writing"))
	})

	ginkgo.It("reports an unreadable replica config", func() {
		replicaSetCollectorConfig = filepath.Join(ginkgo.GinkgoT().TempDir(), "missing.yml")
		err := ConfigureDebugModeSettings(settings)
		gomega.Expect(err).To(gomega.HaveOccurred())
		gomega.Expect(err.Error()).To(gomega.ContainSubstring("Exception while setting prometheus"))
	})
})
