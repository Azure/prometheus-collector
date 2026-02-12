package configmapsettings

import (
	"log"

	cmcommon "github.com/prometheus-collector/shared/configmap/common"
)

func ConfigureOpentelemetryMetricsSettings(metricsConfigBySection map[string]map[string]string) error {
	return cmcommon.WriteOtelMetricsEnvFile(metricsConfigBySection, opentelemetryMetricsEnvVarPath, func(format string, args ...interface{}) {
		log.Printf(format, args...)
	})
}
