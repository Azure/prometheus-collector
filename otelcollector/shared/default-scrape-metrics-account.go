package shared

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

var metricsAccountPattern = regexp.MustCompile(`^[A-Za-z0-9_.:-]+$`)

// ReadDefaultScrapeMetricsAccount reads the optional top-level ConfigMap setting.
func ReadDefaultScrapeMetricsAccount(settingsPath string) (string, error) {
	content, err := os.ReadFile(settingsPath)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("reading default-scrape-metrics-account: %w", err)
	}
	account := strings.TrimSpace(string(content))
	// Prometheus and the collector configuration loaders interpret dollar
	// references differently. Accept identifiers that remain literal in both.
	if account != "" && !metricsAccountPattern.MatchString(account) {
		return "", fmt.Errorf("default-scrape-metrics-account accepts only letters, digits, underscores, periods, colons, and hyphens")
	}
	return account, nil
}

// AddDefaultScrapeMetricsAccount labels built-in jobs before custom jobs are merged.
func AddDefaultScrapeMetricsAccount(config map[interface{}]interface{}, account string) {
	if account == "" {
		return
	}

	jobs, _ := config["scrape_configs"].([]interface{})
	for _, job := range jobs {
		scrapeConfig, ok := job.(map[interface{}]interface{})
		if !ok {
			continue
		}
		rules, _ := scrapeConfig["metric_relabel_configs"].([]interface{})
		updated := make([]interface{}, len(rules), len(rules)+1)
		copy(updated, rules)
		scrapeConfig["metric_relabel_configs"] = append(updated, map[interface{}]interface{}{
			"action":        "replace",
			"source_labels": []interface{}{"microsoft_metrics_account"},
			"regex":         "",
			"target_label":  "microsoft_metrics_account",
			"replacement":   account,
		})
	}
}
