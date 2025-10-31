package testhelpers

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/prometheus-collector/shared"
	"gopkg.in/yaml.v2"
)

var collectorBaseEnvKeys = []string{
	"AZMON_AGENT_CFG_FILE_VERSION",
	"AZMON_AGENT_CFG_SCHEMA_VERSION",
	"AZMON_CLUSTER_ALIAS",
	"AZMON_CLUSTER_LABEL",
	"AZMON_DEFAULT_METRIC_ACCOUNT_NAME",
	"AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG",
	"AZMON_KSM_CONFIG_ENABLED",
	"AZMON_OPERATOR_ENABLED",
	"AZMON_OPERATOR_ENABLED_CHART_SETTING",
	"AZMON_OPERATOR_ENABLED_CFG_MAP_SETTING",
	"AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED",
	"AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX",
	"AZMON_PROMETHEUS_POD_ANNOTATION_SCRAPING_ENABLED",
	"AZMON_SET_GLOBAL_SETTINGS",
	"AZMON_USE_DEFAULT_PROMETHEUS_CONFIG",
	"CONFIGMAP_VERSION",
	"CONFIG_VALIDATOR_RUNNING_IN_AGENT",
	"CONTAINER_TYPE",
	"CONTROLLER_TYPE",
	"DEBUG_MODE_ENABLED",
	"KUBE_STATE_NAME",
	"MAC",
	"MODE",
	"NODE_EXPORTER_TARGETPORT",
	"NODE_IP",
	"NODE_NAME",
	"OS_TYPE",
	"POD_NAMESPACE",
}

var controlPlaneEnvAlternateKeys = []string{
	"AZMON_PROMETHEUS_CLUSTER_AUTOSCALER_ENABLED",
	"AZMON_PROMETHEUS_KUBECONTROLLERMANAGER_ENABLED",
	"AZMON_PROMETHEUS_KUBESCHEDULER_ENABLED",
	"AZMON_PROMETHEUS_NODE_AUTO_PROVISIONING_ENABLED",
}

// CollectorPrefixes contains the configurable root directories used while setting up
// test artifacts for the configmap parser.
type CollectorPrefixes struct {
	ConfigSettingsPrefix  string
	ConfigMapParserPrefix string
}

// CollectorConfigPaths groups the mutable config map mount paths that need to be
// redirected to temporary files during tests. Fields can be left nil when the
// corresponding path is not used by a particular test suite.
type CollectorConfigPaths struct {
	ConfigMapDebugMountPath            *string
	ReplicaSetCollectorConfig          *string
	DefaultSettingsMountPath           *string
	ConfigMapMountPathForPodAnnotation *string
	CollectorSettingsMountPath         *string
	ConfigMapKeepListMountPath         *string
	ConfigMapScrapeIntervalMountPath   *string
	ConfigMapOpentelemetryMetricsPath  *string
}

// CollectorPrefixesFor builds the prefixes used by the configmap parser tests
// and selects between Managed Prometheus (data plane) and control-plane
// variants based on isDataPlane.
func CollectorPrefixesFor(configSettingsPrefix, configMapParserPrefix string, isDataPlane bool) CollectorPrefixes {
	mpPrefixes := CollectorPrefixes{
		ConfigSettingsPrefix:  configSettingsPrefix,
		ConfigMapParserPrefix: configMapParserPrefix,
	}

	ccpPrefixes := CollectorPrefixes{
		ConfigSettingsPrefix: configSettingsPrefix,
	}
	if configMapParserPrefix != "" {
		ccpPrefixes.ConfigMapParserPrefix = configMapParserPrefix
	}

	if isDataPlane {
		return mpPrefixes
	}
	return ccpPrefixes
}

// CollectorProcessedPaths groups the generated file paths produced while the
// configmap parser materializes intermediate results.
type CollectorProcessedPaths struct {
	PodAnnotationEnvVarPath          *string
	CollectorSettingsEnvVarPath      *string
	DefaultSettingsEnvVarPath        *string
	DebugModeEnvVarPath              *string
	ConfigMapKeepListEnvVarPath      *string
	ScrapeIntervalEnvVarPath         *string
	OpentelemetryMetricsEnvVarPath   *string
	KsmConfigEnvVarPath              *string
	ScrapeConfigDefinitionPathPrefix *string
	MergedDefaultConfigPath          *string
	PromMergedConfigPath             *string
	RegexHashFile                    *string
	IntervalHashFile                 *string
}

// DefaultCollectorEnvVarKeys returns the environment variable names that need to
// be reset between data-plane config map parser tests.
func DefaultCollectorEnvVarKeys() []string {
	keys := append([]string{}, collectorBaseEnvKeys...)
	keys = append(keys, CollectorJobEnvVarNames(shared.DefaultScrapeJobs, func(jobName string) string {
		return BuildEnvVarName(jobName, "_SCRAPING_ENABLED")
	})...)
	return uniqueStrings(keys)
}

// ControlPlaneCollectorEnvVarKeys returns the environment variables used by the
// control-plane config map parser tests.
func ControlPlaneCollectorEnvVarKeys() []string {
	keys := append([]string{}, collectorBaseEnvKeys...)
	keys = append(keys, CollectorJobEnvVarNames(shared.ControlPlaneDefaultScrapeJobs, func(jobName string) string {
		return BuildEnvVarName(jobName, "_ENABLED")
	})...)
	keys = append(keys, controlPlaneEnvAlternateKeys...)
	return uniqueStrings(keys)
}

// BuildEnvVarName composes an AZMON_PROMETHEUS environment variable name for a
// job following the supplied suffix convention.
func BuildEnvVarName(jobName, suffix string) string {
	return fmt.Sprintf("AZMON_PROMETHEUS_%s%s", strings.ToUpper(jobName), suffix)
}

// CollectorJobEnvVarNames converts a job catalog into environment variable
// names using the provided formatter.
func CollectorJobEnvVarNames(jobs map[string]*shared.DefaultScrapeJob, formatter func(string) string) []string {
	names := make([]string, 0, len(jobs))
	for jobName := range jobs {
		names = append(names, formatter(jobName))
	}
	return names
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	unique := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}
	return unique
}

// CloneJobEnabledStates copies the enabled flags for the supplied job catalog.
func CloneJobEnabledStates(jobs map[string]*shared.DefaultScrapeJob) map[string]bool {
	clone := make(map[string]bool, len(jobs))
	for jobName, job := range jobs {
		clone[jobName] = job.Enabled
	}
	return clone
}

// BuildEnvVarOverrides renders a map of environment variable overrides from the
// provided enablement map.
func BuildEnvVarOverrides(values map[string]bool, formatter func(string) string) map[string]string {
	overrides := make(map[string]string, len(values))
	for jobName, enabled := range values {
		overrides[formatter(jobName)] = strconv.FormatBool(enabled)
	}
	return overrides
}

// JobEnabledEnvVars generates the default job enablement environment variables
// for the provided job catalog.
func JobEnabledEnvVars(jobs map[string]*shared.DefaultScrapeJob, formatter func(string) string) map[string]string {
	result := make(map[string]string, len(jobs))
	for jobName, job := range jobs {
		result[formatter(jobName)] = strconv.FormatBool(job.Enabled)
	}
	return result
}

// DefaultManagedPrometheusEnvVars returns the baseline environment variables
// expected for Managed Prometheus tests before any overrides are applied.
func DefaultManagedPrometheusEnvVars() map[string]string {
	envVars := map[string]string{
		"AZMON_AGENT_CFG_SCHEMA_VERSION":                   "v1",
		"AZMON_AGENT_CFG_FILE_VERSION":                     "ver1",
		"AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX": "",
		"AZMON_DEFAULT_METRIC_ACCOUNT_NAME":                "",
		"AZMON_CLUSTER_LABEL":                              "",
		"AZMON_CLUSTER_ALIAS":                              "",
		"AZMON_OPERATOR_ENABLED_CHART_SETTING":             "true",
		"AZMON_OPERATOR_ENABLED":                           "true",
		"AZMON_OPERATOR_ENABLED_CFG_MAP_SETTING":           "",
		"AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED":     "false",
		"DEBUG_MODE_ENABLED":                               "false",
		"CONFIGMAP_VERSION":                                "not_present",
	}

	jobEnvVars := JobEnabledEnvVars(shared.DefaultScrapeJobs, func(jobName string) string {
		return BuildEnvVarName(jobName, "_SCRAPING_ENABLED")
	})
	for key, value := range jobEnvVars {
		envVars[key] = value
	}

	return envVars
}

// DefaultControlPlaneEnvVars returns the baseline environment variables
// expected for control-plane (CCP) tests before overrides are applied.
func DefaultControlPlaneEnvVars() map[string]string {
	envVars := map[string]string{
		"AZMON_AGENT_CFG_SCHEMA_VERSION":                   "v1",
		"AZMON_AGENT_CFG_FILE_VERSION":                     "ver1",
		"AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX": "",
		"AZMON_DEFAULT_METRIC_ACCOUNT_NAME":                "",
		"AZMON_CLUSTER_LABEL":                              "",
		"AZMON_CLUSTER_ALIAS":                              "",
		"AZMON_OPERATOR_ENABLED_CHART_SETTING":             "false",
		"AZMON_OPERATOR_ENABLED":                           "false",
		"AZMON_OPERATOR_ENABLED_CFG_MAP_SETTING":           "",
		"AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED":     "false",
		"AZMON_PROMETHEUS_APISERVER_ENABLED":               "true",
		"AZMON_PROMETHEUS_CLUSTER-AUTOSCALER_ENABLED":      "false",
		"AZMON_PROMETHEUS_KUBE-SCHEDULER_ENABLED":          "false",
		"AZMON_PROMETHEUS_KUBE-CONTROLLER-MANAGER_ENABLED": "false",
		"AZMON_PROMETHEUS_ETCD_ENABLED":                    "true",
		"AZMON_PROMETHEUS_NODE-AUTO-PROVISIONING_ENABLED":  "false",
		"DEBUG_MODE_ENABLED":                               "",
		"AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG":           "",
		"CONFIG_VALIDATOR_RUNNING_IN_AGENT":                "",
		"AZMON_USE_DEFAULT_PROMETHEUS_CONFIG":              "",
	}

	jobEnvVars := JobEnabledEnvVars(shared.ControlPlaneDefaultScrapeJobs, func(jobName string) string {
		return BuildEnvVarName(jobName, "_ENABLED")
	})
	for key, value := range jobEnvVars {
		envVars[key] = value
	}

	return envVars
}

// ManagedPrometheusEnvVars returns the environment variables that should be
// present for Managed Prometheus tests based on controller type and OS.
func ManagedPrometheusEnvVars(controllerType, osType string) map[string]string {
	switch osType {
	case shared.OSType.Windows:
		return map[string]string{
			"AZMON_OPERATOR_ENABLED":    "true",
			"CONTAINER_TYPE":            "",
			"CONTROLLER_TYPE":           shared.ControllerType.DaemonSet,
			"OS_TYPE":                   "windows",
			"MODE":                      "advanced",
			"KUBE_STATE_NAME":           "ama-metrics-ksm",
			"POD_NAMESPACE":             "kube-system",
			"MAC":                       "true",
			"NODE_NAME":                 "test-node",
			"NODE_IP":                   "192.168.1.1",
			"NODE_EXPORTER_TARGETPORT":  "9100",
			"AZMON_SET_GLOBAL_SETTINGS": "true",
		}
	case shared.OSType.Linux:
		switch controllerType {
		case shared.ControllerType.ReplicaSet:
			return map[string]string{
				"AZMON_OPERATOR_ENABLED":    "true",
				"CONTAINER_TYPE":            "",
				"CONTROLLER_TYPE":           shared.ControllerType.ReplicaSet,
				"OS_TYPE":                   "linux",
				"MODE":                      "advanced",
				"KUBE_STATE_NAME":           "ama-metrics-ksm",
				"POD_NAMESPACE":             "kube-system",
				"MAC":                       "true",
				"AZMON_SET_GLOBAL_SETTINGS": "true",
			}
		case shared.ControllerType.ConfigReaderSidecar:
			return map[string]string{
				"AZMON_OPERATOR_ENABLED":    "true",
				"CONTAINER_TYPE":            shared.ControllerType.ConfigReaderSidecar,
				"CONTROLLER_TYPE":           shared.ControllerType.ConfigReaderSidecar,
				"OS_TYPE":                   "linux",
				"MODE":                      "advanced",
				"KUBE_STATE_NAME":           "ama-metrics-ksm",
				"POD_NAMESPACE":             "kube-system",
				"MAC":                       "true",
				"AZMON_SET_GLOBAL_SETTINGS": "true",
			}
		case shared.ControllerType.DaemonSet:
			return map[string]string{
				"AZMON_OPERATOR_ENABLED":    "true",
				"CONTAINER_TYPE":            "",
				"CONTROLLER_TYPE":           shared.ControllerType.DaemonSet,
				"OS_TYPE":                   "linux",
				"MODE":                      "advanced",
				"KUBE_STATE_NAME":           "ama-metrics-ksm",
				"POD_NAMESPACE":             "kube-system",
				"MAC":                       "true",
				"NODE_NAME":                 "test-node",
				"NODE_IP":                   "192.168.1.1",
				"NODE_EXPORTER_TARGETPORT":  "9100",
				"AZMON_SET_GLOBAL_SETTINGS": "true",
			}
		}
	}

	return map[string]string{}
}

// SetManagedPrometheusEnvVars configures the Managed Prometheus environment
// variables for the supplied controller and OS combination.
func SetManagedPrometheusEnvVars(controllerType, osType string) error {
	return SetEnvVars(ManagedPrometheusEnvVars(controllerType, osType))
}

// ControlPlaneEnvVars returns the environment variables that should be present
// for control-plane tests based on controller type and OS.
func ControlPlaneEnvVars(controllerType, osType string) map[string]string {
	if osType != shared.OSType.Linux {
		return map[string]string{}
	}

	switch controllerType {
	case shared.ControllerType.ReplicaSet:
		return map[string]string{
			"AZMON_OPERATOR_ENABLED":    "false",
			"CONTAINER_TYPE":            "",
			"CONTROLLER_TYPE":           shared.ControllerType.ReplicaSet,
			"OS_TYPE":                   "linux",
			"MODE":                      "advanced",
			"KUBE_STATE_NAME":           "ama-metrics-ksm",
			"POD_NAMESPACE":             "kube-system",
			"MAC":                       "true",
			"AZMON_SET_GLOBAL_SETTINGS": "true",
		}
	default:
		return map[string]string{}
	}
}

// SetControlPlaneEnvVars configures the control-plane environment variables for
// the supplied controller and OS combination.
func SetControlPlaneEnvVars(controllerType, osType string) error {
	return SetEnvVars(ControlPlaneEnvVars(controllerType, osType))
}

// MergeStringMaps copies the provided maps into a freshly allocated one.
func MergeStringMaps(maps ...map[string]string) map[string]string {
	merged := make(map[string]string)
	for _, m := range maps {
		for key, value := range m {
			merged[key] = value
		}
	}
	return merged
}

// SetupCollectorConfigFiles updates the supplied config map mount paths so that
// the configmap parser writes into temporary files during tests.
func SetupCollectorConfigFiles(defaultPath bool, prefixes CollectorPrefixes, paths CollectorConfigPaths) error {
	if err := os.MkdirAll(prefixes.ConfigSettingsPrefix, 0755); err != nil {
		return fmt.Errorf("failed to create config settings directory %s: %w", prefixes.ConfigSettingsPrefix, err)
	}

	if defaultPath {
		if paths.ConfigMapDebugMountPath != nil {
			*paths.ConfigMapDebugMountPath = "/etc/config/settings/debug-mode"
		}
		if paths.ReplicaSetCollectorConfig != nil {
			*paths.ReplicaSetCollectorConfig = "/opt/microsoft/otelcollector/collector-config-replicaset.yml"
		}
		if paths.DefaultSettingsMountPath != nil {
			*paths.DefaultSettingsMountPath = "/etc/config/settings/default-scrape-settings"
		}
		if paths.ConfigMapKeepListMountPath != nil {
			*paths.ConfigMapKeepListMountPath = "/etc/config/settings/default-targets-metrics-keep-list"
		}
		if paths.ConfigMapMountPathForPodAnnotation != nil {
			*paths.ConfigMapMountPathForPodAnnotation = "/etc/config/settings/pod-annotation-based-scraping"
		}
		if paths.CollectorSettingsMountPath != nil {
			*paths.CollectorSettingsMountPath = "/etc/config/settings/prometheus-collector-settings"
		}
		if paths.ConfigMapOpentelemetryMetricsPath != nil {
			*paths.ConfigMapOpentelemetryMetricsPath = "/etc/config/settings/opentelemetry-metrics"
		}
		if paths.ConfigMapScrapeIntervalMountPath != nil {
			*paths.ConfigMapScrapeIntervalMountPath = "/etc/config/settings/default-targets-scrape-interval-settings"
		}

		if _, err := CreateTempFile(prefixes.ConfigSettingsPrefix, "cluster-metrics", ""); err != nil {
			return err
		}
		if _, err := CreateTempFile(prefixes.ConfigSettingsPrefix, "prometheus-collector-settings", ""); err != nil {
			return err
		}
		return nil
	}

	create := func(target *string, name string) error {
		if target == nil {
			return nil
		}
		path, err := CreateTempFile(prefixes.ConfigSettingsPrefix, name, "")
		if err != nil {
			return err
		}
		*target = path
		return nil
	}

	if err := create(paths.ConfigMapMountPathForPodAnnotation, "pod-annotation-based-scraping"); err != nil {
		return err
	}
	if err := create(paths.CollectorSettingsMountPath, "prometheus-collector-settings"); err != nil {
		return err
	}
	if err := create(paths.DefaultSettingsMountPath, "default-scrape-settings-enabled"); err != nil {
		return err
	}
	if err := create(paths.ConfigMapDebugMountPath, "debug-mode"); err != nil {
		return err
	}
	if err := create(paths.ConfigMapKeepListMountPath, "default-targets-metrics-keep-list"); err != nil {
		return err
	}
	if err := create(paths.ConfigMapScrapeIntervalMountPath, "default-targets-scrape-interval-settings"); err != nil {
		return err
	}
	if err := create(paths.ConfigMapOpentelemetryMetricsPath, "opentelemetry-metrics"); err != nil {
		return err
	}
	if _, err := CreateTempFile(prefixes.ConfigSettingsPrefix, "cluster-metrics", ""); err != nil {
		return err
	}
	if _, err := CreateTempFile(prefixes.ConfigSettingsPrefix, "prometheus-collector-settings", ""); err != nil {
		return err
	}
	if paths.ReplicaSetCollectorConfig != nil {
		*paths.ReplicaSetCollectorConfig = "./testdata/collector-config-replicaset.yml"
	}
	return nil
}

// SetupCollectorProcessedFiles prepares the intermediate files consumed by the
// configmap parser during tests.
func SetupCollectorProcessedFiles(prefixes CollectorPrefixes, paths CollectorProcessedPaths) error {
	if err := os.MkdirAll(prefixes.ConfigMapParserPrefix, 0755); err != nil {
		return fmt.Errorf("failed to create configmap parser directory %s: %w", prefixes.ConfigMapParserPrefix, err)
	}

	create := func(target *string, name string) error {
		if target == nil {
			return nil
		}
		path, err := CreateTempFile(prefixes.ConfigMapParserPrefix, name, "")
		if err != nil {
			return err
		}
		*target = path
		return nil
	}

	if err := create(paths.PodAnnotationEnvVarPath, "config_def_pod_annotation_based_scraping"); err != nil {
		return err
	}
	if err := create(paths.CollectorSettingsEnvVarPath, "config_prometheus_collector_settings_env_var"); err != nil {
		return err
	}
	if err := create(paths.DefaultSettingsEnvVarPath, "config_default_scrape_settings_env_var"); err != nil {
		return err
	}
	if err := create(paths.DebugModeEnvVarPath, "config_debug_mode_env_var"); err != nil {
		return err
	}
	if err := create(paths.ConfigMapKeepListEnvVarPath, "config_def_targets_metrics_keep_list_hash"); err != nil {
		return err
	}
	if err := create(paths.ScrapeIntervalEnvVarPath, "config_def_targets_scrape_intervals_hash"); err != nil {
		return err
	}
	if err := create(paths.OpentelemetryMetricsEnvVarPath, "config_opentelemetry_metrics_env_var"); err != nil {
		return err
	}
	if err := create(paths.KsmConfigEnvVarPath, "config_ksm_config_env_var"); err != nil {
		return err
	}

	srcDir := "../../../configmapparser/default-prom-configs/"
	testDir := "../../../configmapparser/default-prom-configs/test/"
	if err := CopyDirectoryFiles(srcDir, testDir); err != nil {
		return err
	}
	if paths.ScrapeConfigDefinitionPathPrefix != nil {
		*paths.ScrapeConfigDefinitionPathPrefix = testDir
	}

	if err := create(paths.MergedDefaultConfigPath, "defaultsMergedConfig.yaml"); err != nil {
		return err
	}
	if err := create(paths.PromMergedConfigPath, "promMergedConfig.yaml"); err != nil {
		return err
	}

	if paths.RegexHashFile != nil && paths.ConfigMapKeepListEnvVarPath != nil {
		*paths.RegexHashFile = *paths.ConfigMapKeepListEnvVarPath
	}
	if paths.IntervalHashFile != nil && paths.ScrapeIntervalEnvVarPath != nil {
		*paths.IntervalHashFile = *paths.ScrapeIntervalEnvVarPath
	}

	return nil
}

// CreateTempFile writes content to a file at dir/name, ensuring the directory exists.
func CreateTempFile(dir, name, content string) (string, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file %s: %w", path, err)
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("failed to stat file %s: %w", path, err)
	}
	if info.Size() != int64(len(content)) {
		return "", fmt.Errorf("file %s has incorrect size: expected %d, got %d", path, len(content), info.Size())
	}

	return path, nil
}

// MustCreateTempFile wraps CreateTempFile and panics if any error occurs.
// Intended for use in tests where panics fail the test immediately.
func MustCreateTempFile(dir, name, content string) string {
	path, err := CreateTempFile(dir, name, content)
	if err != nil {
		panic(err)
	}
	return path
}

// CopyDirectoryFiles copies all non-directory files from srcDir into dstDir.
func CopyDirectoryFiles(srcDir, dstDir string) error {
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %w", dstDir, err)
	}

	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("failed to read source directory %s: %w", srcDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		srcPath := filepath.Join(srcDir, entry.Name())
		dstPath := filepath.Join(dstDir, entry.Name())

		data, err := os.ReadFile(srcPath)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", srcPath, err)
		}

		if err := os.WriteFile(dstPath, data, 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", dstPath, err)
		}
	}

	return nil
}

// CheckEnvVars verifies that the environment matches the expected key/value pairs.
func CheckEnvVars(expected map[string]string) error {
	for key, value := range expected {
		if os.Getenv(key) != value {
			return fmt.Errorf("expected %s to be %s, but got %s", key, value, os.Getenv(key))
		}
	}
	return nil
}

// SetEnvVars applies the provided environment variables.
func SetEnvVars(values map[string]string) error {
	for key, value := range values {
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("failed setting %s: %w", key, err)
		}
	}
	return nil
}

// UnsetEnvVars removes the supplied environment variables.
func UnsetEnvVars(keys []string) {
	for _, key := range keys {
		_ = os.Unsetenv(key)
	}
}

// ReadYAMLStringMap loads a YAML file containing a string -> string map.
func ReadYAMLStringMap(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}

	if len(data) == 0 {
		return map[string]string{}, nil
	}

	result := make(map[string]string)
	if err := yaml.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML from %s: %w", path, err)
	}

	return result, nil
}

// ExpectedKeepListMap returns the desired keep-list map for the supplied jobs.
// When includeMinimalKeepList is true the resulting value combines the provided
// value and the minimal keep list regex via "value|minimal" to mirror legacy
// behaviour. The collector selector distinguishes between data-plane (Managed
// Prometheus) and control-plane collectors when deriving environment variable
// names.
func ExpectedKeepListMap(
	jobs map[string]*shared.DefaultScrapeJob,
	includeMinimalKeepList bool,
	value string,
	isDataPlane bool,
) map[string]string {
	expected := make(map[string]string, len(jobs))
	for jobName, job := range jobs {
		var key string
		if isDataPlane {
			key = fmt.Sprintf("%s_METRICS_KEEP_LIST_REGEX", strings.ToUpper(jobName))
		} else {
			key = fmt.Sprintf("CONTROLPLANE_%s_KEEP_LIST_REGEX", strings.ToUpper(jobName))
		}
		if includeMinimalKeepList {
			expected[key] = fmt.Sprintf("%s|%s", value, job.MinimalKeepListRegex)
		} else {
			expected[key] = value
		}
	}
	return expected
}

// ExpectedScrapeIntervalMap returns the desired scrape interval map for the
// supplied jobs. When override is empty the job's configured scrape interval is
// used.
func ExpectedScrapeIntervalMap(
	jobs map[string]*shared.DefaultScrapeJob,
	override string,
) map[string]string {
	expected := make(map[string]string, len(jobs))
	for jobName, job := range jobs {
		key := strings.ToUpper(jobName) + "_SCRAPE_INTERVAL"
		if override == "" {
			expected[key] = job.ScrapeInterval
		} else {
			expected[key] = override
		}
	}
	return expected
}
