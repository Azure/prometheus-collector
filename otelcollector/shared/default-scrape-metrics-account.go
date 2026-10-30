package shared

import (
	"fmt"
	"os"
	"strings"
)

// ApplyDefaultScrapeMetricsAccount adds the fixed Azure Monitor routing label to
// built-in scrape jobs. Call this before merging custom scrape configurations.
// The setting is a top-level ConfigMap data key in both schema versions so one
// value can be used by both the cluster and control-plane collectors.
func ApplyDefaultScrapeMetricsAccount(config map[interface{}]interface{}, settingsPath string) error {
	content, err := os.ReadFile(settingsPath)
	if os.IsNotExist(err) {
		return nil // Omitted means disabled.
	}
	if err != nil {
		return fmt.Errorf("reading default-scrape-metrics-account: %w", err)
	}
	account := strings.TrimSpace(string(content))
	if account == "" {
		return nil
	}

	jobs, _ := config["scrape_configs"].([]interface{})
	for _, job := range jobs {
		scrapeConfig, ok := job.(map[interface{}]interface{})
		if !ok {
			continue
		}
		appendMetricsAccountRule(scrapeConfig, "relabel_configs", account)
		// Some built-in jobs honor exporter labels. Assign the account again
		// after scraping for those jobs so an exporter cannot override routing.
		// The target rule still labels synthetic series such as up.
		if scrapeConfig["honor_labels"] == true {
			appendMetricsAccountRule(scrapeConfig, "metric_relabel_configs", account)
		}
	}
	return nil
}

func appendMetricsAccountRule(scrapeConfig map[interface{}]interface{}, key, account string) {
	rules, _ := scrapeConfig[key].([]interface{})
	// Allocate a separate slice for each job, even if their existing rules
	// share backing storage. Preserve discovery and authentication rules.
	updated := make([]interface{}, len(rules), len(rules)+1)
	copy(updated, rules)
	scrapeConfig[key] = append(updated, map[interface{}]interface{}{
		"action":       "replace",
		"target_label": "microsoft_metrics_account",
		// Prometheus replacement strings expand $ references. This setting
		// is a literal value, not a relabeling expression.
		"replacement": strings.ReplaceAll(account, "$", "$$"),
	})
}
