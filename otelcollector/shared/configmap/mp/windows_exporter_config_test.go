package configmapsettings

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
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
		configured     bool
		wantTarget     string
	}{
		{
			name:       "uses AKS port by default",
			wantTarget: "$$NODE_IP$$:19182",
		},
		{
			name:           "uses configured port",
			configuredPort: "9182",
			configured:     true,
			wantTarget:     "$$NODE_IP$$:9182",
		},
		{
			name:           "ignores invalid configured port",
			configuredPort: "70000",
			configured:     true,
			wantTarget:     "$$NODE_IP$$:19182",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.configured {
				t.Setenv(windowsExporterPortEnv, test.configuredPort)
			} else {
				t.Setenv(windowsExporterPortEnv, "")
			}
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

func TestNormalizeWindowsExporterPort(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		want      string
		wantValid bool
	}{
		{name: "minimum", value: "1", want: "1", wantValid: true},
		{name: "maximum", value: "65535", want: "65535", wantValid: true},
		{name: "trims whitespace", value: " 9182 ", want: "9182", wantValid: true},
		{name: "empty", value: "", wantValid: false},
		{name: "not numeric", value: "http", wantValid: false},
		{name: "zero", value: "0", wantValid: false},
		{name: "too large", value: "65536", wantValid: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, valid := normalizeWindowsExporterPort(test.value)
			if got != test.want || valid != test.wantValid {
				t.Fatalf("normalizeWindowsExporterPort(%q) = (%q, %t), want (%q, %t)", test.value, got, valid, test.want, test.wantValid)
			}
		})
	}
}

func TestWindowsExporterMinimalMetricsCompatibility(t *testing.T) {
	requiredMetrics := []string{
		"windows_system_boot_time_timestamp_seconds",
		"windows_system_boot_time_timestamp",
		"windows_system_system_up_time",
		"windows_cpu_time_total",
		"windows_memory_available_bytes",
		"windows_os_visible_memory_bytes",
		"windows_memory_physical_total_bytes",
		"windows_memory_cache_bytes",
		"windows_memory_modified_page_list_bytes",
		"windows_memory_standby_cache_core_bytes",
		"windows_memory_standby_cache_normal_priority_bytes",
		"windows_memory_standby_cache_reserve_bytes",
		"windows_memory_swap_page_operations_total",
		"windows_logical_disk_read_seconds_total",
		"windows_logical_disk_write_seconds_total",
		"windows_logical_disk_size_bytes",
		"windows_logical_disk_free_bytes",
		"windows_net_bytes_total",
		"windows_net_packets_received_discarded_total",
		"windows_net_packets_outbound_discarded_total",
		"windows_container_available",
		"windows_container_cpu_usage_seconds_total",
		"windows_container_memory_usage_commit_bytes",
		"windows_container_memory_usage_private_working_set_bytes",
		"windows_container_network_receive_bytes_total",
		"windows_container_network_transmit_bytes_total",
	}

	configuredMetrics := make(map[string]struct{})
	for _, metric := range strings.Split(windowsExporterMinimalMetricsRegex, "|") {
		configuredMetrics[metric] = struct{}{}
	}
	for _, metric := range requiredMetrics {
		if _, ok := configuredMetrics[metric]; !ok {
			t.Errorf("minimal ingestion profile is missing %s", metric)
		}
	}
}

func TestPopulateWindowsExporterPortSetting(t *testing.T) {
	tests := []struct {
		name     string
		settings map[string]map[string]string
		wantPort string
	}{
		{
			name:     "uses native default when setting is absent",
			settings: map[string]map[string]string{},
			wantPort: windowsExporterDefaultPort,
		},
		{
			name: "accepts legacy exporter port",
			settings: map[string]map[string]string{
				"prometheus-collector-settings": {"windowsexporter_port": "9182"},
			},
			wantPort: "9182",
		},
		{
			name: "rejects invalid port",
			settings: map[string]map[string]string{
				"prometheus-collector-settings": {"windowsexporter_port": "invalid"},
			},
			wantPort: windowsExporterDefaultPort,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			processor := &ConfigProcessor{}
			processor.PopulateSettingValuesFromConfigMap(test.settings)
			if processor.WindowsExporterPort != test.wantPort {
				t.Fatalf("WindowsExporterPort = %q, want %q", processor.WindowsExporterPort, test.wantPort)
			}
		})
	}
}

func TestWindowsExporterPortWrittenToEnvironmentFile(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "collector-settings.env")
	writer := &FileConfigWriter{}
	processor := &ConfigProcessor{WindowsExporterPort: "9182"}
	if err := writer.WriteConfigToFile(configPath, processor); err != nil {
		t.Fatalf("write collector settings: %v", err)
	}

	contents, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read collector settings: %v", err)
	}
	if !bytes.Contains(contents, []byte("AZMON_WINDOWS_EXPORTER_PORT=9182\n")) {
		t.Fatalf("collector settings do not contain Windows exporter port:\n%s", contents)
	}
}

func TestWindowsExporterDefaultReplicaSetPort(t *testing.T) {
	configPath := filepath.Join(
		"..",
		"..",
		"..",
		"configmapparser",
		"default-prom-configs",
		"windowsexporterDefaultRsSimple.yml",
	)
	contents, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read Windows exporter replica set config: %v", err)
	}

	t.Setenv(windowsExporterPortEnv, "9182")
	configuredContents := configureWindowsExporterTarget(contents)
	if !bytes.Contains(configuredContents, []byte("replacement: $$1:9182")) {
		t.Fatalf("configured replica set target does not use port 9182:\n%s", configuredContents)
	}
}
