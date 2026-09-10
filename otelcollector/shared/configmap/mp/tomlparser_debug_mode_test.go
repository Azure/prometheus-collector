package configmapsettings

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus-collector/shared"
	"gopkg.in/yaml.v2"
)

var _ = Describe("When parsing debug mode settings", func() {
	var configDir string
	var originalCollectorConfig []byte

	BeforeEach(func() {
		configDir = GinkgoT().TempDir()
		outputDir := GinkgoT().TempDir()
		setTestPath(&configMapDebugMountPath, filepath.Join(configDir, "debug-mode"))
		setTestPath(&debugModeEnvVarPath, filepath.Join(outputDir, "debug-mode-env"))
		setTestPath(&replicaSetCollectorConfig, filepath.Join(outputDir, "collector-config-replicaset.yml"))
		GinkgoT().Setenv("AZMON_AGENT_CFG_SCHEMA_VERSION", "v1")
		GinkgoT().Setenv("CONTROLLER_TYPE", "ReplicaSet")
		GinkgoT().Setenv("OS_TYPE", "linux")
		GinkgoT().Setenv("CCP_METRICS_ENABLED", "")

		var err error
		originalCollectorConfig, err = os.ReadFile(filepath.Join("testdata", "collector-config-replicaset.yml"))
		Expect(err).NotTo(HaveOccurred())
		Expect(os.WriteFile(replicaSetCollectorConfig, originalCollectorConfig, 0600)).To(Succeed())
	})

	parseSettings := func(content string) map[string]map[string]string {
		Expect(os.WriteFile(configMapDebugMountPath, []byte(content), 0600)).To(Succeed())
		settings, err := shared.ParseV1Config(configDir)
		Expect(err).NotTo(HaveOccurred())
		return settings
	}

	expectDebugMode := func(enabled string) {
		content, err := os.ReadFile(debugModeEnvVarPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(content)).To(Equal("DEBUG_MODE_ENABLED=" + enabled + "\n"))
	}

	expectUnchangedCollectorConfig := func() {
		content, err := os.ReadFile(replicaSetCollectorConfig)
		Expect(err).NotTo(HaveOccurred())
		Expect(content).To(Equal(originalCollectorConfig))
	}

	Context("when debug mode is enabled", func() {
		It("should configure debug mode settings for a linux replica", func() {
			Expect(ConfigureDebugModeSettings(parseSettings(`enabled = true`))).To(Succeed())
			expectDebugMode("true")

			content, err := os.ReadFile(replicaSetCollectorConfig)
			Expect(err).NotTo(HaveOccurred())
			var config shared.OtelConfig
			Expect(yaml.Unmarshal(content, &config)).To(Succeed())
			Expect(config.Service.Pipelines.Metrics.Exporters).To(Equal([]interface{}{"otlp_grpc", "prometheus"}))
			Expect(config.Service.Pipelines.MetricsTelemetry.Receivers).To(Equal([]interface{}{"prometheus"}))
			Expect(config.Service.Pipelines.MetricsTelemetry.Exporters).To(Equal([]interface{}{"prometheus/telemetry"}))
			Expect(config.Service.Pipelines.MetricsTelemetry.Processors).To(Equal([]interface{}{"filter/telemetry"}))
		})

		DescribeTable("should leave the replica config unchanged for a daemonset",
			func(osType string) {
				GinkgoT().Setenv("CONTROLLER_TYPE", "DaemonSet")
				GinkgoT().Setenv("OS_TYPE", osType)

				Expect(ConfigureDebugModeSettings(parseSettings(`enabled = true`))).To(Succeed())
				expectDebugMode("true")
				expectUnchangedCollectorConfig()
			},
			Entry("linux", "linux"),
			Entry("windows", "windows"),
		)
	})

	It("should use the disabled default when the debug mode file is missing", func() {
		settings, err := shared.ParseV1Config(configDir)
		Expect(err).NotTo(HaveOccurred())
		Expect(settings).NotTo(HaveKey("debug-mode"))

		Expect(ConfigureDebugModeSettings(settings)).To(Succeed())
		expectDebugMode("false")
		expectUnchangedCollectorConfig()
	})

	It("should report a missing parsed config map", func() {
		settings, err := shared.ParseV1Config(filepath.Join(configDir, "missing"))
		Expect(err).To(MatchError(ContainSubstring("failed to read config directory")))

		Expect(ConfigureDebugModeSettings(settings)).To(MatchError("configmap section not mounted, using defaults"))
		Expect(debugModeEnvVarPath).NotTo(BeAnExistingFile())
		expectUnchangedCollectorConfig()
	})

	DescribeTable("should use the disabled default for invalid or absent enabled settings",
		func(content string) {
			Expect(ConfigureDebugModeSettings(parseSettings(content))).To(Succeed())
			expectDebugMode("false")
			expectUnchangedCollectorConfig()
		},
		Entry("malformed setting without an enabled key", `[ invalid_key = true`),
		Entry("invalid boolean", `enabled = invalid`),
		Entry("empty section", ""),
		Entry("explicitly disabled", `enabled = false`),
	)

	It("should handle an error while opening the environment variable file", func() {
		debugModeEnvVarPath = filepath.Join(configDir, "missing", "debug-mode-env")

		Expect(ConfigureDebugModeSettings(parseSettings(`enabled = true`))).To(MatchError(ContainSubstring("Exception while opening file for writing prometheus-collector config environment variables")))
		expectUnchangedCollectorConfig()
	})

	It("should handle an error while reading the replicaset collector config file", func() {
		replicaSetCollectorConfig = filepath.Join(configDir, "missing", "collector-config.yml")

		Expect(ConfigureDebugModeSettings(parseSettings(`enabled = true`))).To(MatchError(ContainSubstring("Exception while setting prometheus in the exporter metrics for service pipeline when debug mode is enabled")))
		expectDebugMode("true")
	})

	It("should handle an invalid replicaset collector config", func() {
		Expect(os.WriteFile(replicaSetCollectorConfig, []byte("service: ["), 0600)).To(Succeed())

		Expect(ConfigureDebugModeSettings(parseSettings(`enabled = true`))).To(MatchError(ContainSubstring("Exception while setting prometheus in the exporter metrics for service pipeline when debug mode is enabled")))
		expectDebugMode("true")
	})

	It("should read debug mode from v2 prometheus collector settings", func() {
		GinkgoT().Setenv("AZMON_AGENT_CFG_SCHEMA_VERSION", "v2")
		GinkgoT().Setenv("CONTROLLER_TYPE", "DaemonSet")
		settingsPath := filepath.Join(configDir, "prometheus-collector-settings")
		Expect(os.WriteFile(settingsPath, []byte("debug-mode = true\n"), 0600)).To(Succeed())
		settings, err := shared.ParseMetricsFiles([]string{settingsPath})
		Expect(err).NotTo(HaveOccurred())

		Expect(ConfigureDebugModeSettings(settings)).To(Succeed())
		expectDebugMode("true")
		expectUnchangedCollectorConfig()
	})
})
