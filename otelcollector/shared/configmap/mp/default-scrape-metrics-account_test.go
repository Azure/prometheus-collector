package configmapsettings

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v2"
)

// Use copies of the shipped templates: the merger writes intermediate files.
func TestDefaultScrapeMetricsAccountGeneratedConfigs(t *testing.T) {
	templateDir, err := filepath.Abs("../../../configmapparser/default-prom-configs")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, controller, osType, container string
		operator                            bool
		jobs                                []string
	}{
		{"linux-daemonset", "daemonset", "linux", "", false, []string{"kubelet"}},
		{"operator-replicaset", "replicaset", "linux", "", true, []string{"kube-state-metrics", "kubernetes-pods", "ztunnel"}},
		{"target-allocator", "", "linux", "ConfigReaderSidecar", true, []string{"kube-state-metrics", "kubernetes-pods", "ztunnel"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			cwd, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Chdir(dir); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.Chdir(cwd); err != nil {
					t.Error(err)
				}
			})
			oldPrefix, oldOutput, oldMergedOutput := defaultPromConfigPathPrefix, mergedDefaultConfigPath, promMergedConfigPath
			oldAccount, oldRegex, oldInterval := defaultScrapeMetricsAccountPath, regexHashFile, intervalHashFile
			oldConfig, oldRegexHash, oldIntervalHash := mergedDefaultConfigs, regexHash, intervalHash
			t.Cleanup(func() {
				defaultPromConfigPathPrefix, mergedDefaultConfigPath, promMergedConfigPath = oldPrefix, oldOutput, oldMergedOutput
				defaultScrapeMetricsAccountPath, regexHashFile, intervalHashFile = oldAccount, oldRegex, oldInterval
				mergedDefaultConfigs, regexHash, intervalHash = oldConfig, oldRegexHash, oldIntervalHash
			})
			defaultPromConfigPathPrefix = templateDir + string(os.PathSeparator)
			mergedDefaultConfigPath, promMergedConfigPath = filepath.Join(dir, "defaults.yml"), filepath.Join(dir, "merged.yml")
			defaultScrapeMetricsAccountPath = filepath.Join(dir, "default-scrape-metrics-account")
			regexHashFile, intervalHashFile = filepath.Join(dir, "keep-list"), filepath.Join(dir, "intervals")
			regexHash, intervalHash = map[string]string{}, map[string]string{}
			for _, entry := range os.Environ() {
				key, _, _ := strings.Cut(entry, "=")
				if strings.HasPrefix(key, "AZMON_PROMETHEUS_") {
					t.Setenv(key, "")
				}
			}
			for key, value := range map[string]string{
				"CONTROLLER_TYPE": tc.controller, "OS_TYPE": tc.osType, "CONTAINER_TYPE": tc.container,
				"MODE": "advanced", "WINMODE": "advanced", "POD_NAMESPACE": "kube-system", "KUBE_STATE_NAME": "ama-metrics-ksm",
				"AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED":       "false",
				"AZMON_PROMETHEUS_POD_ANNOTATION_SCRAPING_ENABLED":   "true",
				"AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX":   "^payments$",
				"AZMON_PROMETHEUS_ZTUNNEL_SCRAPING_ENABLED":          "true",
				"AZMON_PROMETHEUS_KUBELET_SCRAPING_ENABLED":          "true",
				"AZMON_PROMETHEUS_KUBESTATE_SCRAPING_ENABLED":        "true",
				"AZMON_PROMETHEUS_WINDOWSEXPORTER_SCRAPING_ENABLED":  "true",
				"AZMON_PROMETHEUS_WINDOWSKUBEPROXY_SCRAPING_ENABLED": "true",
			} {
				t.Setenv(key, value)
			}
			setDefaultFileScrapeInterval("30s")
			for _, account := range []string{"", "service", ""} {
				if err := os.WriteFile(defaultScrapeMetricsAccountPath, []byte(account), 0600); err != nil {
					t.Fatal(err)
				}
				mergedDefaultConfigs = make(map[interface{}]interface{})
				generated := writeDefaultScrapeTargetsFile(tc.operator)
				jobs := scrapeJobsByName(generated)
				var names []string
				for name, job := range jobs {
					names = append(names, name)
					assertMetricsAccountRule(t, name, job["relabel_configs"], "")
					assertMetricsAccountRule(t, name, job["metric_relabel_configs"], account)
				}
				slices.Sort(names)
				if !slices.Equal(names, tc.jobs) {
					t.Fatalf("generated jobs = %v, want %v", names, tc.jobs)
				}
				if account == "" {
					continue
				}
				custom := `scrape_configs:
- job_name: custom-labeled
  static_configs:
  - targets: [localhost:9090]
  relabel_configs:
  - target_label: microsoft_metrics_account
    replacement: hcp
- job_name: custom-unlabeled
  static_configs:
  - targets: [localhost:9091]
`
				mergeDefaultAndCustomScrapeConfigs(custom, generated)
				content, err := os.ReadFile(promMergedConfigPath)
				if err != nil {
					t.Fatal(err)
				}
				var merged map[interface{}]interface{}
				if err := yaml.Unmarshal(content, &merged); err != nil {
					t.Fatal(err)
				}
				mergedJobs := scrapeJobsByName(merged)
				for name, expected := range map[string]string{"custom-labeled": "hcp", "custom-unlabeled": ""} {
					job, ok := mergedJobs[name]
					if !ok {
						t.Fatalf("missing %s", name)
					}
					assertMetricsAccountRule(t, name, job["relabel_configs"], expected)
					assertMetricsAccountRule(t, name, job["metric_relabel_configs"], "")
				}
			}
		})
	}
}

func scrapeJobsByName(config map[interface{}]interface{}) map[string]map[interface{}]interface{} {
	jobs, _ := config["scrape_configs"].([]interface{})
	byName := make(map[string]map[interface{}]interface{}, len(jobs))
	for _, item := range jobs {
		job := item.(map[interface{}]interface{})
		byName[job["job_name"].(string)] = job
	}
	return byName
}

func assertMetricsAccountRule(t *testing.T, job string, relabelConfigs interface{}, account string) {
	t.Helper()
	rules, _ := relabelConfigs.([]interface{})
	found := 0
	for _, item := range rules {
		rule := item.(map[interface{}]interface{})
		if rule["target_label"] == "microsoft_metrics_account" {
			found++
			if rule["replacement"] != account {
				t.Fatalf("%s: unexpected routing rule %v", job, rule)
			}
		}
	}
	want := 0
	if account != "" {
		want = 1
	}
	if found != want {
		t.Fatalf("%s: got %d routing rules, want %d", job, found, want)
	}
}
