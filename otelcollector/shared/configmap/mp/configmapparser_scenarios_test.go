//go:build linux

package configmapsettings

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/prometheus-collector/shared"
	"gopkg.in/yaml.v2"
)

const (
	testConfigmapDir = "testdata/configmaps/"
	publishedV1      = "../../../configmaps/ama-metrics-settings-configmap-v1.yaml"
	publishedV2      = "../../../configmaps/ama-metrics-settings-configmap-v2.yaml"
	allTargetsOffV1  = "../../../test/test-cluster-yamls/configmaps/default-config-map/ama-metrics-settings-configmap-all-targets-disabled.yaml"
	allActionsCustom = "../../../test/test-cluster-yamls/configmaps/custom-config-map/ama-metrics-prometheus-config-configmap-all-actions.yaml"
)

type collectorPod struct {
	name string
	env  map[string]string
}

// Env mirrors the addon chart; ReplicaSet settings are processed by the config-reader sidecar.
var (
	replicaSetPod = collectorPod{"ReplicaSet", map[string]string{
		"CONTROLLER_TYPE": "ReplicaSet", "CONTAINER_TYPE": "ConfigReaderSidecar", "AZMON_OPERATOR_ENABLED": "true",
		"OS_TYPE": "linux", "MODE": "advanced", "MAC": "true"}}
	linuxDaemonSetPod = collectorPod{"Linux DaemonSet", map[string]string{
		"CONTROLLER_TYPE": "DaemonSet", "OS_TYPE": "linux", "MODE": "advanced", "WINMODE": "advanced", "MAC": "true",
		"NODE_IP": "10.224.0.4", "NODE_NAME": "aks-node-0", "NODE_EXPORTER_TARGETPORT": "19100"}}
	windowsDaemonSetPod = collectorPod{"Windows DaemonSet", map[string]string{
		"CONTROLLER_TYPE": "DaemonSet", "OS_TYPE": "windows", "MODE": "advanced", "WINMODE": "advanced", "MAC": "true",
		"NODE_IP": "10.224.0.5", "NODE_NAME": "akswin000000"}}
	allPods = []collectorPod{replicaSetPod, linuxDaemonSetPod, windowsDaemonSetPod}

	// Captured before tests change the working directory.
	packageDir, _ = os.Getwd()
)

// Configmapparser hardcodes the settings path and runs the validator, so tests
// run its parser sequence against temporary ConfigMaps instead.
func runConfigmapParser(t *testing.T, pod collectorPod, configmapFiles ...string) map[string]map[interface{}]interface{} {
	t.Helper()
	tmp, err := filepath.Abs(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	settingsDir := filepath.Join(tmp, "settings")
	if err := os.Mkdir(settingsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	contents := make([][]byte, len(configmapFiles))
	for i, file := range configmapFiles {
		content, err := os.ReadFile(filepath.Join(packageDir, file))
		if err != nil {
			t.Fatal(err)
		}
		contents[i] = content
	}

	isolateParserState(t, tmp)
	customConfigPath := configMapMountPath
	for _, content := range contents {
		var cm struct {
			Metadata struct{ Name string }
			Data     map[string]string
		}
		if err := yaml.Unmarshal(content, &cm); err != nil {
			t.Fatal(err)
		}
		if cm.Metadata.Name == "ama-metrics-prometheus-config" {
			// Custom scrape config is only mounted into the ReplicaSet.
			if pod.name == replicaSetPod.name {
				customConfigPath = filepath.Join(tmp, "prometheus-config")
				if err := os.WriteFile(customConfigPath, []byte(cm.Data["prometheus-config"]), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			continue
		}
		for key, value := range cm.Data {
			if err := os.WriteFile(filepath.Join(settingsDir, key), []byte(value), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	configMapMountPath = customConfigPath

	for key, value := range pod.env {
		os.Setenv(key, value)
	}
	schema, _ := os.ReadFile(filepath.Join(settingsDir, "schema-version"))
	os.Setenv("AZMON_AGENT_CFG_SCHEMA_VERSION", strings.TrimSpace(string(schema)))

	var sections map[string]map[string]string
	if strings.TrimSpace(string(schema)) == "v2" {
		sections, err = shared.ParseMetricsFiles([]string{
			filepath.Join(settingsDir, "cluster-metrics"), filepath.Join(settingsDir, "prometheus-collector-settings")})
	} else {
		sections, err = shared.ParseV1Config(settingsDir)
	}
	if err != nil {
		t.Fatal(err)
	}

	parseSettingsForPodAnnotations(sections)
	parsePrometheusCollectorConfig(sections)
	parseDefaultScrapeSettings(sections)
	tomlparserTargetsMetricsKeepList(sections)
	tomlparserScrapeInterval(sections)
	prometheusConfigMerger(pod.env["AZMON_OPERATOR_ENABLED"] == "true" || pod.env["CONTAINER_TYPE"] == "ConfigReaderSidecar")

	collectorConfig := mergedDefaultConfigPath
	if shared.FileExists(promMergedConfigPath) {
		collectorConfig = promMergedConfigPath
	}
	jobs := map[string]map[interface{}]interface{}{}
	content, err := os.ReadFile(collectorConfig)
	if os.IsNotExist(err) {
		return jobs
	}
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		ScrapeConfigs []map[interface{}]interface{} `yaml:"scrape_configs"`
	}
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		t.Fatal(err)
	}
	for _, job := range cfg.ScrapeConfigs {
		jobs[job["job_name"].(string)] = job
	}
	return jobs
}

// isolateParserState points every parser path and the process environment at test-owned
// state, because the parser communicates through package variables, files and env vars.
func isolateParserState(t *testing.T, tmp string) {
	t.Helper()
	paths := []*string{&podAnnotationEnvVarPath, &collectorSettingsEnvVarPath, &defaultSettingsEnvVarPath,
		&configMapKeepListEnvVarPath, &scrapeIntervalEnvVarPath, &regexHashFile, &intervalHashFile,
		&mergedDefaultConfigPath, &promMergedConfigPath, &configMapMountPath, &configMapSettingsDir,
		&defaultPromConfigPathPrefix}
	saved := make([]string, len(paths))
	for i, p := range paths {
		saved[i] = *p
	}
	savedRegexHash, savedIntervalHash, savedMerged := regexHash, intervalHash, mergedDefaultConfigs
	environ := os.Environ()
	t.Cleanup(func() {
		for i, p := range paths {
			*p = saved[i]
		}
		regexHash, intervalHash, mergedDefaultConfigs = savedRegexHash, savedIntervalHash, savedMerged
		os.Clearenv()
		for _, kv := range environ {
			key, value, _ := strings.Cut(kv, "=")
			os.Setenv(key, value)
		}
	})

	if err := copyDefaultConfigs(filepath.Join(packageDir, "../../../configmapparser/default-prom-configs"), tmp); err != nil {
		t.Fatal(err)
	}
	podAnnotationEnvVarPath = filepath.Join(tmp, "pod-annotation-env")
	collectorSettingsEnvVarPath = filepath.Join(tmp, "collector-settings-env")
	defaultSettingsEnvVarPath = filepath.Join(tmp, "default-settings-env")
	configMapKeepListEnvVarPath = filepath.Join(tmp, "keep-list-hash")
	scrapeIntervalEnvVarPath = filepath.Join(tmp, "interval-hash")
	regexHashFile = configMapKeepListEnvVarPath
	intervalHashFile = scrapeIntervalEnvVarPath
	mergedDefaultConfigPath = filepath.Join(tmp, "defaultsMergedConfig.yml")
	promMergedConfigPath = filepath.Join(tmp, "promMergedConfig.yml")
	configMapMountPath = filepath.Join(tmp, "no-custom-config")
	configMapSettingsDir = filepath.Join(tmp, "settings")
	regexHash = map[string]string{}
	intervalHash = map[string]string{}
	mergedDefaultConfigs = nil
	os.Clearenv()
	// The merger writes the per-target default files into the working directory.
	t.Chdir(tmp)
}

func expectJobs(t *testing.T, jobs map[string]map[interface{}]interface{}, want ...string) {
	t.Helper()
	got := []string{}
	for name := range jobs {
		got = append(got, name)
	}
	sort.Strings(got)
	sort.Strings(want)
	if want == nil {
		want = []string{}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("jobs = %v, want %v", got, want)
	}
}

func relabelRules(job map[interface{}]interface{}, key string) []map[interface{}]interface{} {
	var rules []map[interface{}]interface{}
	list, _ := job[key].([]interface{})
	for _, rule := range list {
		rules = append(rules, rule.(map[interface{}]interface{}))
	}
	return rules
}

func keepRegex(t *testing.T, job map[interface{}]interface{}) *regexp.Regexp {
	t.Helper()
	for _, rule := range relabelRules(job, "metric_relabel_configs") {
		if rule["action"] == "keep" && reflect.DeepEqual(rule["source_labels"], []interface{}{"__name__"}) {
			// Prometheus anchors relabel regexes.
			return regexp.MustCompile("^(?:" + rule["regex"].(string) + ")$")
		}
	}
	t.Fatalf("job %v has no __name__ keep rule", job["job_name"])
	return nil
}

func TestParserStateIsolation(t *testing.T) {
	root, err := filepath.Abs(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Join(root, "shared", "configmap", "mp")
	defaults := filepath.Join(root, "configmapparser", "default-prom-configs")
	fixtures := filepath.Join(pkg, testConfigmapDir)
	if err := os.MkdirAll(fixtures, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.CopyFS(defaults, os.DirFS(filepath.Join(packageDir, "../../../configmapparser/default-prom-configs"))); err != nil {
		t.Fatal(err)
	}
	fixture, err := os.ReadFile(filepath.Join(packageDir, testConfigmapDir, "all-targets-enabled-v2.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixtures, "all-targets-enabled-v2.yaml"), fixture, 0o644); err != nil {
		t.Fatal(err)
	}
	before := map[string][]byte{}
	for _, name := range []string{"istioCniDefault.yml", "ztunnelDefault.yml"} {
		before[name], err = os.ReadFile(filepath.Join(defaults, name))
		if err != nil {
			t.Fatal(err)
		}
	}
	oldPackageDir, oldRegex, oldInterval, oldMerged := packageDir, regexHash, intervalHash, mergedDefaultConfigs
	defer func() {
		packageDir, regexHash, intervalHash, mergedDefaultConfigs = oldPackageDir, oldRegex, oldInterval, oldMerged
	}()
	packageDir = pkg
	regexHash = map[string]string{"original": "regex"}
	intervalHash = map[string]string{"original": "interval"}
	mergedDefaultConfigs = map[interface{}]interface{}{"original": "config"}

	t.Run("parser", func(t *testing.T) {
		jobs := runConfigmapParser(t, replicaSetPod, testConfigmapDir+"all-targets-enabled-v2.yaml")
		if jobs["istio-cni"] == nil || jobs["ztunnel"] == nil {
			t.Fatal("enabled Istio and ztunnel jobs were not parsed")
		}
	})
	if !reflect.DeepEqual(regexHash, map[string]string{"original": "regex"}) ||
		!reflect.DeepEqual(intervalHash, map[string]string{"original": "interval"}) ||
		!reflect.DeepEqual(mergedDefaultConfigs, map[interface{}]interface{}{"original": "config"}) {
		t.Fatal("parser globals were not restored")
	}
	for name, original := range before {
		after, err := os.ReadFile(filepath.Join(defaults, name))
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(after, original) {
			t.Errorf("source template %s was modified", name)
		}
	}
}

func TestAllTargetsV2(t *testing.T) {
	enabled := map[string][]string{
		replicaSetPod.name: {"acstor-capacity-provisioner", "acstor-metrics-exporter", "kube-state-metrics", "kube-dns",
			"kube-proxy", "kube-apiserver", "local-csi-driver", "istio-cni", "ztunnel", "dcgm-exporter"},
		linuxDaemonSetPod.name: {"kubelet", "cadvisor", "node", "kappie-basic", "networkobservability-retina",
			"networkobservability-hubble", "networkobservability-cilium"},
		windowsDaemonSetPod.name: {"kubelet", "kappie-basic", "networkobservability-retina", "windows-exporter", "kube-proxy-windows"},
	}
	for _, pod := range allPods {
		t.Run("disabled/"+pod.name, func(t *testing.T) {
			expectJobs(t, runConfigmapParser(t, pod, testConfigmapDir+"all-targets-disabled-v2.yaml"))
		})
		t.Run("enabled/"+pod.name, func(t *testing.T) {
			expectJobs(t, runConfigmapParser(t, pod, testConfigmapDir+"all-targets-enabled-v2.yaml"), enabled[pod.name]...)
		})
	}
}

func TestPublishedSettings(t *testing.T) {
	rs := []string{"acstor-capacity-provisioner", "acstor-metrics-exporter", "dcgm-exporter", "kube-state-metrics", "local-csi-driver"}
	linux := []string{"cadvisor", "kappie-basic", "kubelet", "networkobservability-cilium", "networkobservability-hubble",
		"networkobservability-retina", "node"}
	windows := []string{"kappie-basic", "kubelet", "networkobservability-retina"}
	for _, tc := range []struct {
		file string
		want map[string][]string
	}{
		{publishedV1, map[string][]string{replicaSetPod.name: rs, linuxDaemonSetPod.name: linux, windowsDaemonSetPod.name: windows}},
		{publishedV2, map[string][]string{
			replicaSetPod.name:       append([]string{"kube-dns", "kube-proxy"}, rs...),
			linuxDaemonSetPod.name:   linux,
			windowsDaemonSetPod.name: append([]string{"windows-exporter", "kube-proxy-windows"}, windows...)}},
	} {
		for _, pod := range allPods {
			t.Run(filepath.Base(tc.file)+"/"+pod.name, func(t *testing.T) {
				expectJobs(t, runConfigmapParser(t, pod, tc.file), tc.want[pod.name]...)
			})
		}
	}
}

func TestDcgmCustomIntervalAndKeepList(t *testing.T) {
	job := runConfigmapParser(t, replicaSetPod, testConfigmapDir+"dcgm-custom.yaml")["dcgm-exporter"]
	if job == nil {
		t.Fatal("dcgm-exporter job missing")
	}
	if job["scrape_interval"] != "60s" {
		t.Fatalf("scrape_interval = %v, want 60s", job["scrape_interval"])
	}
	keep := keepRegex(t, job)
	for _, metric := range []string{"DCGM_FI_DEV_GPU_UTIL", "DCGM_FI_DEV_MEM_COPY_UTIL", "DCGM_FI_DEV_POWER_USAGE"} {
		if !keep.MatchString(metric) {
			t.Errorf("keep-list %q drops %s", keep, metric)
		}
	}
	if keep.MatchString("DCGM_FI_DEV_SM_CLOCK") {
		t.Errorf("keep-list %q keeps DCGM_FI_DEV_SM_CLOCK outside the custom list", keep)
	}
}

func TestCustomScrapeConfigWithDefaultTargetsDisabled(t *testing.T) {
	expectJobs(t, runConfigmapParser(t, replicaSetPod, allTargetsOffV1, allActionsCustom),
		"job-replace", "job-lowercase", "job-uppercase", "job-keep", "job-drop", "job-keepequal",
		"job-dropequal", "job-hashmod", "job-labelmap", "job-labeldrop", "job-labelkeep")
	for _, pod := range []collectorPod{linuxDaemonSetPod, windowsDaemonSetPod} {
		t.Run(pod.name, func(t *testing.T) {
			expectJobs(t, runConfigmapParser(t, pod, allTargetsOffV1, allActionsCustom))
		})
	}
}

func TestCustomerSettings(t *testing.T) {
	for _, schema := range []string{"v1", "v2"} {
		t.Run(schema, func(t *testing.T) {
			jobs := runConfigmapParser(t, replicaSetPod, testConfigmapDir+"customer-settings-"+schema+".yaml")
			if got := os.Getenv("AZMON_CLUSTER_LABEL"); got != "Customer_Config_POC" {
				t.Errorf("AZMON_CLUSTER_LABEL = %q, want Customer_Config_POC", got)
			}

			job := jobs["kubernetes-pods"]
			if job == nil {
				t.Fatal("kubernetes-pods job missing")
			}
			if job["scrape_interval"] != "45s" {
				t.Errorf("scrape_interval = %v, want 45s", job["scrape_interval"])
			}
			namespaceFilter := false
			for _, rule := range relabelRules(job, "relabel_configs") {
				namespaceFilter = namespaceFilter || (rule["action"] == "keep" &&
					reflect.DeepEqual(rule["source_labels"], []interface{}{"__meta_kubernetes_namespace"}) &&
					rule["regex"] == "^ama-settings-included$")
			}
			if !namespaceFilter {
				t.Error("missing exact namespace keep rule")
			}
			keep := keepRegex(t, job)
			if !keep.MatchString("weather_temperature") || keep.MatchString("unrelated_metric") {
				t.Errorf("keep-list %q does not select only weather_ metrics", keep)
			}
		})
	}
}
