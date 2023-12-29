package main

// FilesystemConfigLoader implements ConfigLoader for file-based configuration loading.
type FilesystemConfigLoader struct {
	ConfigMapMountPath string
}

// ConfigProcessor handles the processing of configuration settings.
type ConfigProcessor struct {
	DefaultMetricAccountName string
	ClusterAlias             string
	ClusterLabel             string
	IsOperatorEnabled        string
	IsOperatorEnabledChartSetting string
}

// ConfigParser is an interface for parsing configurations.
type ConfigParser interface {
	PopulateSettingValuesFromConfigMap(parsedConfig map[string]string)
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
}
