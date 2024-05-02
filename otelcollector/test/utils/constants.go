package utils

var (
	// Slices can't be constants
	LogLineErrorsToExclude = [...]string{
		// Metrics Extension
		"\"filepath\":\"/MetricsExtensionConsoleDebugLog.log\"",
		// Arc token adapter
		"create or renew cluster identity error",
		"get token from status error",
		// KSM
		"ended with: an error on the server",
		"Objects listed",
	}
)

const (
	OperatorLabel              = "operator"
	ArcExtensionLabel          = "arc-extension"
	WindowsLabel               = "windows"
	ARM64Label                 = "arm64"
	FIPSLabel                  = "fips"
	LinuxDaemonsetCustomConfig = "linux-daemonset-custom-config"
)
