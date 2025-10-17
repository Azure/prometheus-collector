package utils

var (
	// Slices can't be constants
	LogLineErrorsToExclude = [...]string{
		// Metrics Extension
		"\"filepath\":\"/MetricsExtensionConsoleDebugLog.log\"",
		"\"filepath\":\"C:\\\\MetricsExtensionConsoleDebugLog.log\"",
		// Arc token adapter
		"create or renew cluster identity error",
		"get token from status error",
		// Arc node-exporter
		"Failed to open directory, disabling udev device properties",
		// KSM
		"ended with: an error on the server",
		"Objects listed",
		"stat /etc/config/settings/ksm-config: no such file or directory",
		// Target allocator
		"client connection lost",
		"remote error: tls: bad certificate",
		"local error: tls: bad record",
		"client sent an HTTP request to an HTTPS server"
		// Config reader
		"AZMON_OPERATOR_HTTPS_ENABLED is not set/false or error in cert creation",
		// ReplicaSet
		"Failed to reach Target Allocator endpoint with HTTPS",
	}
)

const (
	OperatorLabel                                     = "operator"
	ArcExtensionLabel                                 = "arc-extension"
	WindowsLabel                                      = "windows"
	ARM64Label                                        = "arm64"
	FIPSLabel                                         = "fips"
	RetinaLabel                                       = "retina"
	MDSDLabel                                         = "mdsd"
	OTLPLabel                                         = "otlp"
	LinuxDaemonsetCustomConfig                        = "linux-daemonset-custom-config"
	ConfigProcessingCommonNoConfigMaps                = "config-processing-common-no-config-maps"
	ConfigProcessingCommonWithConfigMap               = "config-processing-common-with-config-maps"
	ConfigProcessingCommonWithErrorConfigMap          = "config-processing-common-with-error-config-maps"
	ConfigProcessingCommon                            = "config-processing-common"
	ConfigProcessingNoConfigMaps                      = "config-processing-no-config-maps"
	ConfigProcessingAllTargetsDisabled                = "config-processing-all-targets-disabled"
	ConfigProcessingDefaultTargetsEnabled             = "config-processing-default-targets-enabled"
	ConfigProcessingRsTargetsEnabled                  = "config-processing-rs-targets-enabled"
	ConfigProcessingDsTargetsEnabled                  = "config-processing-ds-targets-enabled"
	ConfigProcessingAllTargetsEnabled                 = "config-processing-all-targets-enabled"
	ConfigProcessingOnlyCustomConfigMapWithAllActions = "config-processing-only-config-map-all-actions"
	ConfigProcessingGlobalSettings                    = "config-processing-global-settings"
	// ConfigProcessingSettingsCustomAndGlobal           = "config-processing-settings-custom-global"
	ConfigProcessingSettingsNodeConfigMap = "config-processing-settings-node-config-map"
	ConfigProcessingSettingsError         = "config-processing-settings-error"
	ConfigProcessingCustomConfigMapError  = "config-processing-custom-config-map-error"
	ConfigProcessingGlobalSettingsError   = "config-processing-global-settings-error"
)
