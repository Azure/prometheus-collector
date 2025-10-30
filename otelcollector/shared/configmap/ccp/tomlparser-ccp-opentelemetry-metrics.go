package ccpconfigmapsettings

import (
	"log"

	cmcommon "github.com/prometheus-collector/shared/configmap/common"
)

var (
	opentelemetryMetricsEnvVarPath = "/opt/microsoft/configmapparser/config_opentelemetry_metrics_env_var"
)

func ConfigureOpentelemetryMetricsSettings(metricsConfigBySection map[string]map[string]string) error {
	return cmcommon.WriteOtelMetricsEnvFile(metricsConfigBySection, opentelemetryMetricsEnvVarPath, func(format string, args ...interface{}) {
		log.Printf(format, args...)
	})
}
