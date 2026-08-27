package configmapsettings

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v2"
)

func TestWindowsExporterDefaultDaemonSetPort(t *testing.T) {
	configPath := filepath.Join(
		"..",
		"..",
		"..",
		"configmapparser",
		"default-prom-configs",
		"windowsexporterDefaultDs.yml",
	)
	contents, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read Windows exporter default config: %v", err)
	}

	tests := []struct {
		name           string
		configuredPort string
		wantTarget     string
	}{
		{
			name:       "uses AKS port by default",
			wantTarget: "$$NODE_IP$$:19182",
		},
		{
			name:           "uses configured port",
			configuredPort: "9182",
			wantTarget:     "$$NODE_IP$$:9182",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("NODE_EXPORTER_TARGETPORT", test.configuredPort)
			configuredContents := configureWindowsExporterTarget(contents)

			var config struct {
				ScrapeConfigs []struct {
					JobName       string `yaml:"job_name"`
					StaticConfigs []struct {
						Targets []string `yaml:"targets"`
					} `yaml:"static_configs"`
				} `yaml:"scrape_configs"`
			}
			if err := yaml.Unmarshal(configuredContents, &config); err != nil {
				t.Fatalf("parse Windows exporter default config: %v", err)
			}

			if len(config.ScrapeConfigs) != 1 || config.ScrapeConfigs[0].JobName != "windows-exporter" {
				t.Fatalf("expected one windows-exporter scrape config, got %#v", config.ScrapeConfigs)
			}
			staticConfigs := config.ScrapeConfigs[0].StaticConfigs
			if len(staticConfigs) != 1 || len(staticConfigs[0].Targets) != 1 {
				t.Fatalf("expected one Windows exporter target, got %#v", staticConfigs)
			}
			if got := staticConfigs[0].Targets[0]; got != test.wantTarget {
				t.Fatalf("Windows exporter target = %q, want %q", got, test.wantTarget)
			}
		})
	}
}
