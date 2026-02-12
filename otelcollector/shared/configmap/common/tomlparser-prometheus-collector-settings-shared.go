package common

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

// CollectorSettings captures the shared prometheus-collector-settings values
// that both MP and CCP flows care about.
type CollectorSettings struct {
	MetricAccountName    string
	ClusterAlias         string
	ClusterLabel         string
	OperatorEnabled      bool
	OperatorEnabledChart bool
}

var nonAlphaNumeric = regexp.MustCompile(`[^0-9a-zA-Z]+`)

// PopulateSharedCollectorSettings hydrates the supplied CollectorSettings with
// values from the prometheus-collector-settings section and relevant
// environment variables.
func PopulateSharedCollectorSettings(result *CollectorSettings, metricsConfig map[string]map[string]string, logFn LogFunc) {
	logger := ensureLogFunc(logFn)

	settingsSection, ok := metricsConfig["prometheus-collector-settings"]
	if !ok {
		logger("prometheus-collector-settings section not found in metricsConfigBySection, using defaults\n")
	}

	if ok {
		if value, exists := settingsSection["default_metric_account_name"]; exists {
			result.MetricAccountName = value
			logger("Using configmap setting for default metric account name: %s\n", result.MetricAccountName)
		}

		if value, exists := settingsSection["cluster_alias"]; exists {
			alias := strings.TrimSpace(value)
			logger("Got configmap setting for cluster_alias: %s\n", alias)
			if alias != "" {
				alias = nonAlphaNumeric.ReplaceAllString(alias, "_")
				alias = strings.Trim(alias, "_")
				logger("After replacing non-alpha-numeric characters with '_': %s\n", alias)
			}
			result.ClusterAlias = alias
		}
	}

	operatorEnabledEnv := strings.ToLower(strings.TrimSpace(os.Getenv("AZMON_OPERATOR_ENABLED")))
	if operatorEnabledEnv == "true" {
		result.OperatorEnabledChart = true
		if ok {
			if value, exists := settingsSection["operator_enabled"]; exists {
				result.OperatorEnabled = strings.ToLower(strings.TrimSpace(value)) == "true"
				logger("Configmap setting enabling operator: %t\n", result.OperatorEnabled)
			}
		}
	} else {
		result.OperatorEnabledChart = false
		result.OperatorEnabled = false
	}

	result.ClusterLabel = deriveClusterLabel(result.ClusterAlias)
}

func deriveClusterLabel(clusterAlias string) string {
	cluster := strings.TrimSpace(os.Getenv("CLUSTER"))
	if strings.EqualFold(strings.TrimSpace(os.Getenv("MAC")), "true") && cluster != "" {
		segments := strings.Split(cluster, "/")
		cluster = segments[len(segments)-1]
	}

	if clusterAlias != "" {
		cluster = clusterAlias
	}

	return cluster
}

// WriteSharedCollectorSettings writes the shared collector settings to the
// provided writer.
func WriteSharedCollectorSettings(w io.Writer, settings CollectorSettings) error {
	if _, err := fmt.Fprintf(w, "AZMON_DEFAULT_METRIC_ACCOUNT_NAME=%s\n", settings.MetricAccountName); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "AZMON_CLUSTER_LABEL=%s\n", settings.ClusterLabel); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "AZMON_CLUSTER_ALIAS=%s\n", settings.ClusterAlias); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "AZMON_OPERATOR_ENABLED_CHART_SETTING=%t\n", settings.OperatorEnabledChart); err != nil {
		return err
	}
	if settings.OperatorEnabled {
		if _, err := fmt.Fprintf(w, "AZMON_OPERATOR_ENABLED=%t\n", settings.OperatorEnabled); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "AZMON_OPERATOR_ENABLED_CFG_MAP_SETTING=%t\n", settings.OperatorEnabled); err != nil {
			return err
		}
	}
	return nil
}
