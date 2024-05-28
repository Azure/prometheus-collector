package configmapsettings

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
	minimalingestionprofile    string
}

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
	ControlplaneEtcd                  string
	NoDefaultsEnabled                 bool

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
