package configmapsettings

var (
	configSettingsPrefix               = "/etc/config/settings/"
	configMapParserPrefix              = "/opt/microsoft/configmapparser/"
	schemaVersionFile                  = configSettingsPrefix + "schema-version"
	configVersionFile                  = configSettingsPrefix + "config-version"
	configMapDebugMountPath            = configSettingsPrefix + "debug-mode"
	replicaSetCollectorConfig          = "/opt/microsoft/otelcollector/collector-config-replicaset.yml"
	debugModeEnvVarPath                = configMapParserPrefix + "config_debug_mode_env_var"
	defaultSettingsMountPath           = configSettingsPrefix + "default-scrape-settings-enabled"
	defaultSettingsMountPathv2         = configSettingsPrefix + "default-targets-scrape-enabled"
	defaultSettingsEnvVarPath          = configMapParserPrefix + "config_default_scrape_settings_env_var"
	configMapMountPathForPodAnnotation = configSettingsPrefix + "pod-annotation-based-scraping"
	podAnnotationEnvVarPath            = configMapParserPrefix + "config_def_pod_annotation_based_scraping"
	collectorSettingsMountPath         = configSettingsPrefix + "prometheus-collector-settings"
	collectorSettingsEnvVarPath        = configMapParserPrefix + "config_prometheus_collector_settings_env_var"
	configMapKeepListMountPath         = configSettingsPrefix + "default-targets-metrics-keep-list"
	configMapKeepListEnvVarPath        = configMapParserPrefix + "config_def_targets_metrics_keep_list_hash"
	configMapScrapeIntervalMountPath   = configSettingsPrefix + "default-targets-scrape-interval-settings"
	scrapeIntervalEnvVarPath           = configMapParserPrefix + "config_def_targets_scrape_intervals_hash"
	promMergedConfigPath               = "/opt/promMergedConfig.yaml"
	mergedDefaultConfigPath            = "/opt/defaultsMergedConfig.yaml"
	defaultPromConfigPathPrefix        = "/opt/microsoft/otelcollector/default-prom-configs/"
	regexHashFile                      = "/opt/microsoft/configmapparser/config_def_targets_metrics_keep_list_hash"
	intervalHashFile                   = "/opt/microsoft/configmapparser/config_def_targets_scrape_intervals_hash"
)

type RegexValues struct {
	kubelet                    string
	coredns                    string
	cadvisor                   string
	kubeproxy                  string
	apiserver                  string
	kubestate                  string
	nodeexporter               string
	kappiebasic                string
	windowsexporter            string
	windowskubeproxy           string
	networkobservabilityretina string
	networkobservabilityhubble string
	networkobservabilitycilium string
	podannotations             string
	minimalingestionprofile    string
	acstorcapacityprovisioner  string
	acstormetricsexporter      string
}

// FilesystemConfigLoader implements ConfigLoader for file-based configuration loading.
type FilesystemConfigLoader struct {
	ConfigMapMountPath string
}

// ConfigProcessor handles the processing of configuration settings.
type ConfigProcessor struct {
	DefaultMetricAccountName                string
	ClusterAlias                            string
	ClusterLabel                            string
	IsOperatorEnabled                       bool
	IsOperatorEnabledChartSetting           bool
	ControlplaneKubeControllerManager       string
	ControlplaneKubeScheduler               string
	ControlplaneApiserver                   string
	ControlplaneClusterAutoscaler           string
	ControlplaneEtcd                        string
	NoDefaultsEnabled                       bool
	TargetallocatorHttpsEnabled             bool
	TargetallocatorHttpsEnabledChartSetting bool

	Kubelet                    string
	Coredns                    string
	Cadvisor                   string
	Kubeproxy                  string
	Apiserver                  string
	Kubestate                  string
	NodeExporter               string
	PrometheusCollectorHealth  string
	PodAnnotation              string
	Windowsexporter            string
	Windowskubeproxy           string
	Kappiebasic                string
	NetworkObservabilityRetina string
	NetworkObservabilityHubble string
	NetworkObservabilityCilium string
	AcstorCapacityProvisioner  string
	AcstorMetricsExporter      string
}

// ConfigParser is an interface for parsing configurations.
type ConfigParser interface {
	PopulateSettingValuesFromConfigMap(parsedConfig map[string]string)
	PopulateSettingValues(parsedConfig map[string]string)
}

// Configurator is responsible for configuring the application.
type Configurator struct {
	ConfigLoader   *FilesystemConfigLoader
	ConfigParser   *ConfigProcessor
	ConfigWriter   *FileConfigWriter
	ConfigFilePath string
}

type FileConfigWriter struct {
	ConfigProcessor *ConfigProcessor
	Config          map[string]string
}

// ConfigLoader is an interface for loading configurations.
type ConfigLoader interface {
	ParseConfigMapForDefaultScrapeSettings() (map[string]string, error)
	SetDefaultScrapeSettings() (map[string]string, error)
}

// ConfigWriter is an interface for writing configurations to a file.
type ConfigWriter interface {
	WriteDefaultScrapeSettingsToFile(filename string) error
}
