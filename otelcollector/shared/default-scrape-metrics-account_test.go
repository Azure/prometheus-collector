package shared

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gopkg.in/yaml.v2"
)

func TestApplyDefaultScrapeMetricsAccount(t *testing.T) {
	for _, tc := range []struct {
		name, value, replacement string
		missing, invalid         bool
	}{
		{name: "omitted", missing: true},
		{name: "empty"},
		{name: "whitespace", value: " \n\t"},
		{name: "enabled", value: "service", replacement: "service"},
		{name: "block scalar newline", value: "service\n", replacement: "service"},
		{name: "literal account", value: "service-1.foo_bar:prod", replacement: "service-1.foo_bar:prod"},
		{name: "replacement expression", value: "service-$1-${name}", invalid: true},
		{name: "node substitution", value: "$NODE_NAME", invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "default-scrape-metrics-account")
			if !tc.missing {
				if err := os.WriteFile(path, []byte(tc.value), 0600); err != nil {
					t.Fatal(err)
				}
			}
			var config map[interface{}]interface{}
			if err := yaml.Unmarshal([]byte(`scrape_configs:
- job_name: existing-rules
  relabel_configs:
  - action: keep
    source_labels: [__meta_kubernetes_namespace]
    regex: kube-system
  - target_label: microsoft_metrics_account
    replacement: previous
  metric_relabel_configs:
  - action: keep
    source_labels: [__name__]
    regex: node_cpu_seconds_total
- job_name: no-rules
- job_name: null-rules
  relabel_configs: null
`), &config); err != nil {
				t.Fatal(err)
			}
			before, _ := yaml.Marshal(config)
			account, err := ReadDefaultScrapeMetricsAccount(path)
			if (err != nil) != tc.invalid {
				t.Fatalf("unexpected error for %q: %v", tc.value, err)
			}
			if account != tc.replacement {
				t.Fatalf("account = %q, want %q", account, tc.replacement)
			}
			AddDefaultScrapeMetricsAccount(config, account)
			if tc.replacement == "" {
				after, _ := yaml.Marshal(config)
				if string(before) != string(after) {
					t.Fatal("disabled setting changed the scrape configuration")
				}
				return
			}
			var original map[interface{}]interface{}
			if err := yaml.Unmarshal(before, &original); err != nil {
				t.Fatal(err)
			}
			for i, item := range config["scrape_configs"].([]interface{}) {
				job := item.(map[interface{}]interface{})
				rules := job["metric_relabel_configs"].([]interface{})
				want := map[interface{}]interface{}{
					"action": "replace", "target_label": "microsoft_metrics_account", "replacement": tc.replacement,
					"source_labels": []interface{}{"microsoft_metrics_account"}, "regex": "",
				}
				if !reflect.DeepEqual(rules[len(rules)-1], want) {
					t.Fatalf("job %v: unexpected final rule %v", job["job_name"], rules)
				}
				// Removing only the added rule must recover the original job,
				// including its metric filters and existing target rules.
				oldJob := original["scrape_configs"].([]interface{})[i].(map[interface{}]interface{})
				if len(rules) > 1 {
					job["metric_relabel_configs"] = rules[:len(rules)-1]
				} else if old, exists := oldJob["metric_relabel_configs"]; exists {
					job["metric_relabel_configs"] = old
				} else {
					delete(job, "metric_relabel_configs")
				}
				if !reflect.DeepEqual(job, oldJob) {
					t.Fatalf("job %v: existing configuration changed", job["job_name"])
				}
			}
		})
	}
}

func TestDefaultScrapeMetricsAccountReadError(t *testing.T) {
	if _, err := ReadDefaultScrapeMetricsAccount(t.TempDir()); err == nil {
		t.Fatal("expected an error when the settings path is a directory")
	}
}

func TestDefaultScrapeMetricsAccountExamplesShipDisabled(t *testing.T) {
	for _, version := range []string{"v1", "v2"} {
		t.Run(version, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("../configmaps", "ama-metrics-settings-configmap-"+version+".yaml"))
			if err != nil {
				t.Fatal(err)
			}
			var cm struct {
				Data map[string]string `yaml:"data"`
			}
			if err := yaml.Unmarshal(content, &cm); err != nil {
				t.Fatal(err)
			}
			const key = "default-scrape-metrics-account"
			if value, exists := cm.Data[key]; !exists || value != "" {
				t.Fatal("sample must ship disabled")
			}

		})
	}
}
