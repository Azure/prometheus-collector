package configmapsettings

var (
	schemaVersionFile                            = "/etc/config/settings/schema-version"
	configVersionFile                            = "/etc/config/settings/config-version"
	configMapDebugMountPath                      = "/etc/config/settings/debug-mode"
	configMapOpentelemetryMetricsMountPath       = "/etc/config/settings/opentelemetry-metrics"
	replicaSetCollectorConfig                    = "/opt/microsoft/otelcollector/collector-config-replicaset.yml"
	debugModeEnvVarPath                          = "/opt/microsoft/configmapparser/config_debug_mode_env_var"
	defaultSettingsMountPath                     = "/etc/config/settings/default-scrape-settings-enabled"
	defaultSettingsMountPathv2                   = "/etc/config/settings/default-targets-scrape-enabled"
	defaultSettingsEnvVarPath                    = "/opt/microsoft/configmapparser/config_default_scrape_settings_env_var"
	configMapMountPathForPodAnnotation           = "/etc/config/settings/pod-annotation-based-scraping"
	podAnnotationEnvVarPath                      = "/opt/microsoft/configmapparser/config_def_pod_annotation_based_scraping"
	collectorSettingsMountPath                   = "/etc/config/settings/prometheus-collector-settings"
	collectorSettingsEnvVarPath                  = "/opt/microsoft/configmapparser/config_prometheus_collector_settings_env_var"
	opentelemetryMetricsEnvVarPath               = "/opt/microsoft/configmapparser/config_opentelemetry_metrics_env_var"
	ksmConfigEnvVarPath                          = "/opt/microsoft/configmapparser/config_ksm_config_env_var"
	configMapKeepListMountPath                   = "/etc/config/settings/default-targets-metrics-keep-list"
	configMapKeepListEnvVarPath                  = "/opt/microsoft/configmapparser/config_def_targets_metrics_keep_list_hash"
	configMapScrapeIntervalMountPath             = "/etc/config/settings/default-targets-scrape-interval-settings"
	scrapeIntervalEnvVarPath                     = "/opt/microsoft/configmapparser/config_def_targets_scrape_intervals_hash"
	promMergedConfigPath                         = "/opt/promMergedConfig.yml"
	mergedDefaultConfigPath                      = "/opt/defaultsMergedConfig.yml"
	defaultPromConfigPathPrefix                  = "/opt/microsoft/otelcollector/default-prom-configs/"
	regexHashFile                                = "/opt/microsoft/configmapparser/config_def_targets_metrics_keep_list_hash"
	intervalHashFile                             = "/opt/microsoft/configmapparser/config_def_targets_scrape_intervals_hash"
	kubeletDefaultFileRsSimple                   = "kubeletDefaultRsSimple.yml"
	kubeletDefaultFileRsAdvanced                 = "kubeletDefaultRsAdvanced.yml"
	kubeletDefaultFileDs                         = "kubeletDefaultDs.yml"
	kubeletDefaultFileRsAdvancedWindowsDaemonset = "kubeletDefaultRsAdvancedWindowsDaemonset.yml"
	coreDNSDefaultFile                           = "corednsDefault.yml"
	cadvisorDefaultFileRsSimple                  = "cadvisorDefaultRsSimple.yml"
	cadvisorDefaultFileRsAdvanced                = "cadvisorDefaultRsAdvanced.yml"
	cadvisorDefaultFileDs                        = "cadvisorDefaultDs.yml"
	kubeProxyDefaultFile                         = "kubeproxyDefault.yml"
	apiserverDefaultFile                         = "apiserverDefault.yml"
	kubeStateDefaultFile                         = "kubestateDefault.yml"
	nodeExporterDefaultFileRsSimple              = "nodeexporterDefaultRsSimple.yml"
	nodeExporterDefaultFileRsAdvanced            = "nodeexporterDefaultRsAdvanced.yml"
	nodeExporterDefaultFileDs                    = "nodeexporterDefaultDs.yml"
	prometheusCollectorHealthDefaultFile         = "prometheusCollectorHealth.yml"
	windowsExporterDefaultRsSimpleFile           = "windowsexporterDefaultRsSimple.yml"
	windowsExporterDefaultDsFile                 = "windowsexporterDefaultDs.yml"
	windowsKubeProxyDefaultFileRsSimpleFile      = "windowskubeproxyDefaultRsSimple.yml"
	windowsKubeProxyDefaultDsFile                = "windowskubeproxyDefaultDs.yml"
	podAnnotationsDefaultFile                    = "podannotationsDefault.yml"
	kappieBasicDefaultFileDs                     = "kappieBasicDefaultDs.yml"
	networkObservabilityRetinaDefaultFileDs      = "networkobservabilityRetinaDefaultDs.yml"
	networkObservabilityHubbleDefaultFileDs      = "networkobservabilityHubbleDefaultDs.yml"
	networkObservabilityCiliumDefaultFileDs      = "networkobservabilityCiliumDefaultDs.yml"
	acstorCapacityProvisionerDefaultFile         = "acstorCapacityProvisionerDefaultFile.yml"
	acstorMetricsExporterDefaultFile             = "acstorMetricsExporterDefaultFile.yml"
	LocalCSIDriverDefaultFile                    = "localCSIDriverDefaultFile.yml"
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
	localcsidriver             string
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
	ControlplaneNodeAutoProvisioning        string
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
	LocalCSIDriver             string
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
