package utils

var(
	// Slices can't be constants
	LogLineErrorsToExclude = [...]string{
		"\"filepath\":\"/MetricsExtensionConsoleDebugLog.log\"",
		"create or renew cluster identity error"
	}
)