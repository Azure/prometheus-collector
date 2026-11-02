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
		missing                  bool
	}{
		{name: "omitted", missing: true},
		{name: "empty"},
		{name: "whitespace", value: " \n\t"},
		{name: "enabled", value: "service", replacement: "service"},
		{name: "block scalar newline", value: "service\n", replacement: "service"},
		{name: "literal replacement", value: "service-$1-${name}", replacement: "service-$$1-$${name}"},
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
			if err := ApplyDefaultScrapeMetricsAccount(config, path); err != nil {
				t.Fatal(err)
			}
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
				rules := job["relabel_configs"].([]interface{})
				want := map[interface{}]interface{}{
					"action": "replace", "target_label": "microsoft_metrics_account", "replacement": tc.replacement,
				}
				if !reflect.DeepEqual(rules[len(rules)-1], want) {
					t.Fatalf("job %v: unexpected final rule %v", job["job_name"], rules)
				}
				// Removing only the added rule must recover the original job,
				// including its metric filters and existing target rules.
				oldJob := original["scrape_configs"].([]interface{})[i].(map[interface{}]interface{})
				if len(rules) > 1 {
					job["relabel_configs"] = rules[:len(rules)-1]
				} else if old, exists := oldJob["relabel_configs"]; exists {
					job["relabel_configs"] = old
				} else {
					delete(job, "relabel_configs")
				}
				if !reflect.DeepEqual(job, oldJob) {
					t.Fatalf("job %v: existing configuration changed", job["job_name"])
				}
			}
		})
	}
}

func TestDefaultScrapeMetricsAccountReadError(t *testing.T) {
	config := map[interface{}]interface{}{}
	if err := ApplyDefaultScrapeMetricsAccount(config, t.TempDir()); err == nil {
		t.Fatal("expected an error when the settings path is a directory")
	}
	if len(config) != 0 {
		t.Fatal("read failure changed the configuration")
	}
}

func TestDefaultScrapeMetricsAccountRulesDoNotShareStorage(t *testing.T) {
	path := writeFile(t, t.TempDir(), "default-scrape-metrics-account", "service")
	sharedRules := make([]interface{}, 0, 1)
	first := map[interface{}]interface{}{"relabel_configs": sharedRules}
	second := map[interface{}]interface{}{"relabel_configs": sharedRules}
	config := map[interface{}]interface{}{"scrape_configs": []interface{}{first, second}}
	if err := ApplyDefaultScrapeMetricsAccount(config, path); err != nil {
		t.Fatal(err)
	}
	first["relabel_configs"].([]interface{})[0].(map[interface{}]interface{})["replacement"] = "changed"
	if got := second["relabel_configs"].([]interface{})[0].(map[interface{}]interface{})["replacement"]; got != "service" {
		t.Fatalf("jobs share routing rules: %v", got)
	}
}

func TestDefaultScrapeMetricsAccountHonoredExporterLabels(t *testing.T) {
	path := writeFile(t, t.TempDir(), "default-scrape-metrics-account", "service")
	originalRule := map[interface{}]interface{}{"action": "keep", "source_labels": []interface{}{"__name__"}, "regex": "requests_total"}
	job := map[interface{}]interface{}{
		"honor_labels":           true,
		"metric_relabel_configs": []interface{}{originalRule},
	}
	config := map[interface{}]interface{}{"scrape_configs": []interface{}{job}}
	if err := ApplyDefaultScrapeMetricsAccount(config, path); err != nil {
		t.Fatal(err)
	}
	rules := job["metric_relabel_configs"].([]interface{})
	if len(rules) != 2 || !reflect.DeepEqual(rules[0], originalRule) {
		t.Fatal("existing metric rules were not preserved")
	}
	if rule := rules[1].(map[interface{}]interface{}); rule["target_label"] != "microsoft_metrics_account" || rule["replacement"] != "service" {
		t.Fatalf("expected final account assignment, got %v", rule)
	}
	if rules := job["relabel_configs"].([]interface{}); len(rules) != 1 {
		t.Fatal("synthetic series need a target routing label")
	}
}

func TestDefaultScrapeMetricsAccountConfigMapSchemas(t *testing.T) {
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
			for _, account := range []string{"", "service"} {
				cm.Data[key] = account
				dir := t.TempDir()
				for name, value := range cm.Data {
					writeFile(t, dir, name, value)
				}
				job := map[interface{}]interface{}{"job_name": "built-in"}
				config := map[interface{}]interface{}{"scrape_configs": []interface{}{job}}
				if err := ApplyDefaultScrapeMetricsAccount(config, filepath.Join(dir, key)); err != nil {
					t.Fatal(err)
				}
				_, labeled := job["relabel_configs"]
				if labeled != (account != "") {
					t.Fatalf("schema %s: unexpected labeling for %q", version, account)
				}
			}
		})
	}
}
