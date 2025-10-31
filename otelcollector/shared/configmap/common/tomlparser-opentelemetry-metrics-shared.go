package common

import (
	"fmt"
	"io"
	"os"
	"strconv"
)

// ResolveOtelMetricsEnabled inspects the "opentelemetry-metrics" section and
// returns true when the "enabled" entry parses to true. Missing or invalid
// values fall back to false.
func ResolveOtelMetricsEnabled(metricsConfig map[string]map[string]string, logFn LogFunc) bool {
	logger := ensureLogFunc(logFn)

	section, ok := metricsConfig["opentelemetry-metrics"]
	if !ok {
		return false
	}

	raw, ok := section["enabled"]
	if !ok {
		return false
	}

	enabled, err := strconv.ParseBool(raw)
	if err != nil {
		logger("Invalid value for opentelemetry-metrics enabled: %s, defaulting to false\n", raw)
		return false
	}

	logger("Using configmap setting for opentelemetry-metrics: %v\n", enabled)
	return enabled
}

// WriteOtelMetricsEnv writes the AZMON_FULL_OTLP_ENABLED flag to the provided
// writer, mirroring previous behaviour in both CCP and MP flows.
func WriteOtelMetricsEnv(w io.Writer, enabled bool, logFn LogFunc) error {
	if _, err := fmt.Fprintf(w, "AZMON_FULL_OTLP_ENABLED=%v\n", enabled); err != nil {
		return err
	}

	logger := ensureLogFunc(logFn)
	logger("Setting AZMON_FULL_OTLP_ENABLED environment variable: %v\n", enabled)

	return nil
}

// WriteOtelMetricsEnvFile is a convenience that resolves the enabled flag and
// writes directly to the target file path.
func WriteOtelMetricsEnvFile(metricsConfig map[string]map[string]string, dstPath string, logFn LogFunc) error {
	if metricsConfig == nil {
		return fmt.Errorf("configmap section not mounted, using defaults")
	}

	enabled := ResolveOtelMetricsEnabled(metricsConfig, logFn)

	file, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("exception while opening file for writing prometheus-collector config environment variables: %w", err)
	}
	defer file.Close()

	if err := WriteOtelMetricsEnv(file, enabled, logFn); err != nil {
		return err
	}

	return nil
}
