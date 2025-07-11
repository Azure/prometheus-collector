package ccpconfigmapsettings

var (
	configSettingsPrefix             = "/etc/config/settings/"
	configMapParserPrefix            = "/opt/microsoft/configmapparser/"
	schemaVersionFile                = configSettingsPrefix + "schema-version"
	configVersionFile                = configSettingsPrefix + "config-version"
	collectorSettingsEnvVarPath      = configMapParserPrefix + "config_prometheus_collector_settings_env_var"
	defaultSettingsEnvVarPath        = configMapParserPrefix + "config_default_scrape_settings_env_var"
	defaultSettingsMountPath         = configSettingsPrefix + "default-scrape-settings-enabled"
	defaultSettingsMountPathv2       = configSettingsPrefix + "default-targets-scrape-enabled"
	collectorSettingsMountPath       = configSettingsPrefix + "prometheus-collector-settings"
	configMapKeepListMountPath       = configSettingsPrefix + "default-targets-metrics-keep-list"
	configMapKeepListEnvVarPath      = configMapParserPrefix + "config_def_targets_metrics_keep_list_hash"
	scrapeConfigDefinitionPathPrefix = "/opt/microsoft/otelcollector/default-prom-configs/"
	mergedDefaultConfigPath          = "/opt/defaultsMergedConfig.yml"
)

// FilesystemConfigLoader implements ConfigLoader for file-based configuration loading.
type FilesystemConfigLoader struct {
	ConfigMapMountPath string
}

// ConfigProcessor handles the processing of configuration settings.
type ConfigProcessor struct {
	DefaultMetricAccountName          string
	ClusterAlias                      string
	ClusterLabel                      string
	IsOperatorEnabled                 bool
	IsOperatorEnabledChartSetting     bool
	ControlplaneKubeControllerManager string
	ControlplaneKubeScheduler         string
	ControlplaneApiserver             string
	ControlplaneClusterAutoscaler     string
	ControlplaneNodeAutoProvisioning  string
	ControlplaneEtcd                  string
	NoDefaultsEnabled                 bool
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
