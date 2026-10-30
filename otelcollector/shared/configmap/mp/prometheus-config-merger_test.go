package configmapsettings

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
)

const teamRelabelConfigs = `- action: replace
  source_labels: [__meta_kubernetes_pod_label_team]
  target_label: team
- action: labeldrop
  regex: temp_.*
`

// commentedOutRelabelConfigs is the default-targets-relabel-configs sample shipped in the settings configmaps.
const commentedOutRelabelConfigs = `# - action: replace
#   source_labels: [__meta_kubernetes_pod_label_team]
#   target_label: team`

// clusterMetricsTemplate is the cluster-metrics section of a v2 settings configmap, with a placeholder for the
// default-targets-relabel-configs section.
const clusterMetricsTemplate = `default-targets-scrape-enabled: |-
  kubelet = true
pod-annotation-based-scraping: |-
  podannotationnamespaceregex = ""
%s
default-targets-metrics-keep-list: |-
  kubelet = ""
`

var _ = Describe("default-targets-relabel-configs", func() {
	BeforeEach(func() {
		settingsDir := makeTempDirForTest()
		overrideForTest(&defaultRelabelConfigsMountPath, filepath.Join(settingsDir, "default-targets-relabel-configs"))
		overrideForTest(&clusterMetricsMountPath, filepath.Join(settingsDir, "cluster-metrics"))
	})

	Context("with schema v1", func() {
		BeforeEach(func() {
			setEnvForTest("AZMON_AGENT_CFG_SCHEMA_VERSION", "v1")
		})

		It("appends the relabel configs to every job, after the job's own relabel configs", func() {
			writeFileForTest(defaultRelabelConfigsMountPath, teamRelabelConfigs)
			configs := newRelabelTestScrapeConfigs()

			appendGlobalRelabelConfigs(configs)

			global := parseRelabelConfigs(teamRelabelConfigs)
			original := relabelConfigsByJob(newRelabelTestScrapeConfigs())
			Expect(relabelConfigsByJob(configs)).To(Equal(map[string][]interface{}{
				"no-relabel-configs":   global,
				"null-relabel-configs": global,
				"own-relabel-configs":  append(original["own-relabel-configs"], global...),
			}))
		})

		It("does nothing when the setting is not provided", func() {
			configs := newRelabelTestScrapeConfigs()

			appendGlobalRelabelConfigs(configs)

			Expect(configs).To(Equal(newRelabelTestScrapeConfigs()))
		})

		DescribeTable("does nothing when the setting has no valid relabel configs",
			func(setting string) {
				writeFileForTest(defaultRelabelConfigsMountPath, setting)
				configs := newRelabelTestScrapeConfigs()

				appendGlobalRelabelConfigs(configs)

				Expect(configs).To(Equal(newRelabelTestScrapeConfigs()))
			},
			Entry("commented-out sample", commentedOutRelabelConfigs),
			Entry("empty", ""),
			Entry("invalid YAML", "- action: [replace"),
			Entry("a mapping instead of a list", "action: replace\ntarget_label: team"),
			Entry("a list of non-mappings", "- action=replace"),
		)
	})

	Context("with schema v2", func() {
		BeforeEach(func() {
			setEnvForTest("AZMON_AGENT_CFG_SCHEMA_VERSION", "v2")
		})

		DescribeTable("reads the setting from the cluster-metrics section",
			func(section string) {
				writeFileForTest(clusterMetricsMountPath, fmt.Sprintf(clusterMetricsTemplate, section))
				configs := newRelabelTestScrapeConfigs()

				appendGlobalRelabelConfigs(configs)

				Expect(relabelConfigsByJob(configs)).To(HaveKeyWithValue("no-relabel-configs", parseRelabelConfigs(teamRelabelConfigs)))
			},
			Entry("as a block scalar like the other sections", `default-targets-relabel-configs: |-
  - action: replace
    source_labels: [__meta_kubernetes_pod_label_team]
    target_label: team
  - action: labeldrop
    regex: temp_.*`),
			Entry("as a YAML list", `default-targets-relabel-configs:
  - action: replace
    source_labels: [__meta_kubernetes_pod_label_team]
    target_label: team
  - action: labeldrop
    regex: temp_.*`),
		)

		DescribeTable("does nothing when the section has no relabel configs",
			func(section string) {
				writeFileForTest(clusterMetricsMountPath, fmt.Sprintf(clusterMetricsTemplate, section))
				configs := newRelabelTestScrapeConfigs()

				appendGlobalRelabelConfigs(configs)

				Expect(configs).To(Equal(newRelabelTestScrapeConfigs()))
			},
			Entry("commented-out sample", `default-targets-relabel-configs: |-
  # - action: replace
  #   source_labels: [__meta_kubernetes_pod_label_team]
  #   target_label: team`),
			Entry("section absent", ""),
		)
	})
})

var _ = Describe("default-targets-relabel-configs in the generated prometheus config", func() {
	BeforeEach(func() {
		workDir := makeTempDirForTest()
		overrideForTest(&defaultPromConfigPathPrefix, copyDefaultPromConfigTemplates(workDir))
		// The default job files are generated in the working directory.
		chdirForTest(workDir)
		overrideForTest(&mergedDefaultConfigPath, filepath.Join(workDir, "defaultsMergedConfig.yml"))
		overrideForTest(&promMergedConfigPath, filepath.Join(workDir, "promMergedConfig.yml"))
		overrideForTest(&regexHashFile, filepath.Join(workDir, "keep-list-hash"))
		overrideForTest(&intervalHashFile, filepath.Join(workDir, "scrape-interval-hash"))
		overrideForTest(&defaultRelabelConfigsMountPath, filepath.Join(workDir, "default-targets-relabel-configs"))
		originalMergedDefaultConfigs := mergedDefaultConfigs
		DeferCleanup(func() { mergedDefaultConfigs = originalMergedDefaultConfigs })
		setEnvForTest("AZMON_AGENT_CFG_SCHEMA_VERSION", "v1")
		setEnvForTest("AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED", "false")
		setEnvForTest("MODE", "advanced")
		setEnvForTest("OS_TYPE", "linux")
	})

	DescribeTable("appends the setting to every job in the default scrape config file",
		func(env map[string]string, job string) {
			for key, value := range env {
				setEnvForTest(key, value)
			}
			withoutSetting := generateDefaultScrapeConfig()
			Expect(withoutSetting).To(HaveKey(job))

			writeFileForTest(defaultRelabelConfigsMountPath, teamRelabelConfigs)
			withSetting := generateDefaultScrapeConfig()

			global := parseRelabelConfigs(teamRelabelConfigs)
			Expect(withSetting).To(HaveLen(len(withoutSetting)))
			for jobName, ownRelabelConfigs := range withoutSetting {
				Expect(withSetting).To(HaveKeyWithValue(jobName, append(ownRelabelConfigs, global...)))
			}
		},
		Entry("ReplicaSet, where kube-state-metrics has no relabel configs of its own", map[string]string{
			"CONTROLLER_TYPE": "ReplicaSet",
			"KUBE_STATE_NAME": "ama-metrics-ksm",
			"POD_NAMESPACE":   "kube-system",
			"AZMON_PROMETHEUS_KUBESTATE_SCRAPING_ENABLED": "true",
		}, "kube-state-metrics"),
		Entry("DaemonSet, where kubelet has relabel configs of its own", map[string]string{
			"CONTROLLER_TYPE": "DaemonSet",
			"AZMON_PROMETHEUS_KUBELET_SCRAPING_ENABLED": "true",
		}, "kubelet"),
	)

	It("does not add the setting to custom scrape jobs", func() {
		setEnvForTest("CONTROLLER_TYPE", "ReplicaSet")
		setEnvForTest("AZMON_PROMETHEUS_KUBESTATE_SCRAPING_ENABLED", "true")
		writeFileForTest(defaultRelabelConfigsMountPath, teamRelabelConfigs)
		defaultJobsRelabelConfigs := generateDefaultScrapeConfig()
		Expect(defaultJobsRelabelConfigs).To(HaveKeyWithValue("kube-state-metrics", ContainElements(parseRelabelConfigs(teamRelabelConfigs))))

		// Same as prometheusConfigMerger does after writing the default scrape config file.
		mergeDefaultAndCustomScrapeConfigs(`scrape_configs:
- job_name: custom-job
  static_configs:
  - targets: ['localhost:9090']
  relabel_configs:
  - source_labels: [__address__]
    target_label: custom
`, mergedDefaultConfigs)

		relabelConfigs := relabelConfigsByJob(readYAMLFileForTest(promMergedConfigPath))
		Expect(relabelConfigs).To(HaveKeyWithValue("kube-state-metrics", defaultJobsRelabelConfigs["kube-state-metrics"]))
		Expect(relabelConfigs).To(HaveKeyWithValue("custom-job", parseRelabelConfigs("- source_labels: [__address__]\n  target_label: custom")))
	})
})

// newRelabelTestScrapeConfigs returns merged default scrape configs with jobs that have no, null and own relabel_configs.
func newRelabelTestScrapeConfigs() map[interface{}]interface{} {
	var configs map[interface{}]interface{}
	Expect(yaml.Unmarshal([]byte(`scrape_configs:
- job_name: no-relabel-configs
- job_name: null-relabel-configs
  relabel_configs:
- job_name: own-relabel-configs
  relabel_configs:
  - source_labels: [__address__]
    target_label: instance
`), &configs)).To(Succeed())
	return configs
}

func parseRelabelConfigs(content string) []interface{} {
	var relabelConfigs []interface{}
	Expect(yaml.Unmarshal([]byte(content), &relabelConfigs)).To(Succeed())
	return relabelConfigs
}

// relabelConfigsByJob returns the relabel_configs of every scrape job in a prometheus config, keyed by job name.
func relabelConfigsByJob(config map[interface{}]interface{}) map[string][]interface{} {
	scrapeConfigs, ok := config["scrape_configs"].([]interface{})
	Expect(ok).To(BeTrue())
	relabelConfigs := map[string][]interface{}{}
	for _, scrapeConfig := range scrapeConfigs {
		job := scrapeConfig.(map[interface{}]interface{})
		jobRelabelConfigs, _ := job["relabel_configs"].([]interface{})
		relabelConfigs[job["job_name"].(string)] = jobRelabelConfigs
	}
	return relabelConfigs
}

// generateDefaultScrapeConfig writes the default scrape config file like prometheusConfigMerger does when there is
// no custom config, and returns the relabel_configs of every job in it.
func generateDefaultScrapeConfig() map[string][]interface{} {
	setDefaultFileScrapeInterval("30s")
	writeDefaultScrapeTargetsFile(false)
	return relabelConfigsByJob(readYAMLFileForTest(mergedDefaultConfigPath))
}

func readYAMLFileForTest(path string) map[interface{}]interface{} {
	content, err := os.ReadFile(path)
	Expect(err).NotTo(HaveOccurred())
	var config map[interface{}]interface{}
	Expect(yaml.Unmarshal(content, &config)).To(Succeed())
	return config
}

// copyDefaultPromConfigTemplates copies the default scrape job templates into dir and returns the copy's path
// prefix, since some jobs are generated in place and tests must not modify the templates in the repository.
func copyDefaultPromConfigTemplates(dir string) string {
	const templatesDir = "../../../configmapparser/default-prom-configs"
	templatesCopyDir := filepath.Join(dir, "default-prom-configs")
	Expect(os.Mkdir(templatesCopyDir, 0755)).To(Succeed())
	entries, err := os.ReadDir(templatesDir)
	Expect(err).NotTo(HaveOccurred())
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		content, err := os.ReadFile(filepath.Join(templatesDir, entry.Name()))
		Expect(err).NotTo(HaveOccurred())
		writeFileForTest(filepath.Join(templatesCopyDir, entry.Name()), string(content))
	}
	return templatesCopyDir + string(os.PathSeparator)
}

func makeTempDirForTest() string {
	dir, err := os.MkdirTemp("", "default-targets-relabel-configs")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, dir)
	return dir
}

func chdirForTest(dir string) {
	originalDir, err := os.Getwd()
	Expect(err).NotTo(HaveOccurred())
	Expect(os.Chdir(dir)).To(Succeed())
	DeferCleanup(os.Chdir, originalDir)
}

// overrideForTest sets a package-level variable for the current spec only.
func overrideForTest(variable *string, value string) {
	original := *variable
	*variable = value
	DeferCleanup(func() { *variable = original })
}

// setEnvForTest sets an environment variable for the current spec only.
func setEnvForTest(key, value string) {
	original, wasSet := os.LookupEnv(key)
	Expect(os.Setenv(key, value)).To(Succeed())
	DeferCleanup(func() {
		if wasSet {
			os.Setenv(key, original)
		} else {
			os.Unsetenv(key)
		}
	})
}

func writeFileForTest(path, content string) {
	Expect(os.WriteFile(path, []byte(content), 0644)).To(Succeed())
}
