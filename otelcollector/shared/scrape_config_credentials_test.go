package shared

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v2"
)

func TestDropScrapeConfigsWithFileRefs(t *testing.T) {
	input := `scrape_configs:
- job_name: safe
  file_sd_configs:
  - files: [/etc/prometheus/targets.yaml]
- job_name: exposed
  bearer_token_file: /var/run/secrets/token
- job_name: certificate
  tls_config:
    cert_file: /var/run/secrets/cert
`

	filtered, dropped, err := DropScrapeConfigsWithFileRefs(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(dropped) != 2 || !strings.Contains(dropped[0], "exposed") || !strings.Contains(dropped[1], "certificate") {
		t.Fatalf("dropped jobs = %v, want exposed and certificate", dropped)
	}

	var config map[interface{}]interface{}
	if err := yaml.Unmarshal([]byte(filtered), &config); err != nil {
		t.Fatal(err)
	}
	jobs, ok := config["scrape_configs"].([]interface{})
	if !ok || len(jobs) != 1 {
		t.Fatalf("filtered scrape_configs = %#v, want only the safe job", config["scrape_configs"])
	}
	job := jobs[0].(map[interface{}]interface{})
	if job["job_name"] != "safe" {
		t.Fatalf("remaining job_name = %v, want safe", job["job_name"])
	}
}

func TestDropScrapeConfigsWithFileRefsLeavesUnchangedConfig(t *testing.T) {
	input := "scrape_configs:\n- job_name: safe\n  static_configs: []\n"
	filtered, dropped, err := DropScrapeConfigsWithFileRefs(input)
	if err != nil {
		t.Fatal(err)
	}
	if filtered != input || len(dropped) != 0 {
		t.Fatalf("got (%q, %v), want original config and no dropped jobs", filtered, dropped)
	}
}

func TestDropScrapeConfigsWithFileRefsRejectsMalformedConfig(t *testing.T) {
	input := "scrape_configs: ["
	filtered, dropped, err := DropScrapeConfigsWithFileRefs(input)
	if err == nil || filtered != input || len(dropped) != 0 {
		t.Fatalf("got (%q, %v, %v), want original config, no dropped jobs, and an error", filtered, dropped, err)
	}
}
