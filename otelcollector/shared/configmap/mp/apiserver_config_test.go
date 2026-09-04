package configmapsettings

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v2"
)

func TestAPIServerDefaultScrapeVerifiesTLS(t *testing.T) {
	configPath := filepath.Join(
		"..",
		"..",
		"..",
		"configmapparser",
		"default-prom-configs",
		"apiserverDefault.yml",
	)
	contents, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read API server default config: %v", err)
	}

	var config struct {
		ScrapeConfigs []struct {
			JobName         string `yaml:"job_name"`
			BearerTokenFile string `yaml:"bearer_token_file"`
			TLSConfig       struct {
				CAFile             string `yaml:"ca_file"`
				ServerName         string `yaml:"server_name"`
				InsecureSkipVerify *bool  `yaml:"insecure_skip_verify"`
			} `yaml:"tls_config"`
		} `yaml:"scrape_configs"`
	}
	if err := yaml.Unmarshal(contents, &config); err != nil {
		t.Fatalf("parse API server default config: %v", err)
	}

	if len(config.ScrapeConfigs) != 1 || config.ScrapeConfigs[0].JobName != "kube-apiserver" {
		t.Fatalf("expected one kube-apiserver scrape config, got %#v", config.ScrapeConfigs)
	}

	scrapeConfig := config.ScrapeConfigs[0]
	if scrapeConfig.BearerTokenFile != "/var/run/secrets/kubernetes.io/serviceaccount/token" {
		t.Fatalf("unexpected API server bearer token file %q", scrapeConfig.BearerTokenFile)
	}
	if scrapeConfig.TLSConfig.CAFile != "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt" {
		t.Fatalf("unexpected API server CA file %q", scrapeConfig.TLSConfig.CAFile)
	}
	if scrapeConfig.TLSConfig.ServerName != "kubernetes.default.svc" {
		t.Fatalf("API server TLS server name = %q, want kubernetes.default.svc", scrapeConfig.TLSConfig.ServerName)
	}
	if scrapeConfig.TLSConfig.InsecureSkipVerify == nil || *scrapeConfig.TLSConfig.InsecureSkipVerify {
		t.Fatal("API server TLS certificate verification must be explicitly enabled")
	}
}
