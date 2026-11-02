package ccpconfigmapsettings

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v2"
)

func TestDefaultScrapeMetricsAccountControlplane(t *testing.T) {
	dir := t.TempDir()
	oldAccount, oldOutput, oldRegex := defaultScrapeMetricsAccountPath, mergedDefaultConfigPath, regexHashFile
	oldConfig, oldRegexHash := mergedDefaultConfigs, regexHash
	t.Cleanup(func() {
		defaultScrapeMetricsAccountPath, mergedDefaultConfigPath, regexHashFile = oldAccount, oldOutput, oldRegex
		mergedDefaultConfigs, regexHash = oldConfig, oldRegexHash
	})
	defaultScrapeMetricsAccountPath = filepath.Join(dir, "default-scrape-metrics-account")
	mergedDefaultConfigPath, regexHashFile = filepath.Join(dir, "defaults.yml"), filepath.Join(dir, "keep-list")
	regexHash = map[string]string{}
	templates := map[*string]string{
		&controlplaneApiserverDefaultFile:      "controlplane_apiserver.yml",
		&controlplaneKubeSchedulerDefaultFile:  "controlplane_kube_scheduler.yml",
		&controlplaneKubeControllerManagerFile: "controlplane_kube_controller_manager.yml",
		&controlplaneClusterAutoscalerFile:     "controlplane_cluster_autoscaler.yml",
		&controlplaneNodeAutoProvisioningFile:  "controlplane_node_auto_provisioning.yml",
		&controlplaneEtcdDefaultFile:           "controlplane_etcd.yml",
	}
	for path, name := range templates {
		old := *path
		t.Cleanup(func() { *path = old })
		content, err := os.ReadFile(filepath.Join("../../../configmapparser/default-prom-configs", name))
		if err != nil {
			t.Fatal(err)
		}
		*path = filepath.Join(dir, name)
		if err := os.WriteFile(*path, content, 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("CONTROLLER_TYPE", "replicaset")
	t.Setenv("POD_NAMESPACE", "kube-system")
	t.Setenv("AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED", "false")
	for _, name := range []string{"APISERVER", "KUBE_SCHEDULER", "KUBE_CONTROLLER_MANAGER", "CLUSTER_AUTOSCALER", "NODE_AUTO_PROVISIONING", "ETCD"} {
		t.Setenv("AZMON_PROMETHEUS_CONTROLPLANE_"+name+"_ENABLED", "true")
	}
	for _, account := range []string{"", "service", ""} {
		if err := os.WriteFile(defaultScrapeMetricsAccountPath, []byte(account), 0600); err != nil {
			t.Fatal(err)
		}
		prometheusCcpConfigMerger()
		content, err := os.ReadFile(mergedDefaultConfigPath)
		if err != nil {
			t.Fatal(err)
		}
		var config map[interface{}]interface{}
		if err := yaml.Unmarshal(content, &config); err != nil {
			t.Fatal(err)
		}
		jobs, ok := config["scrape_configs"].([]interface{})
		if !ok || len(jobs) != 6 {
			t.Fatalf("expected six control-plane jobs, got %v", config)
		}
		for _, item := range jobs {
			job := item.(map[interface{}]interface{})
			rules := job["relabel_configs"].([]interface{})
			count := 0
			for _, item := range rules {
				rule := item.(map[interface{}]interface{})
				if rule["target_label"] == "microsoft_metrics_account" {
					count++
					if rule["replacement"] != account {
						t.Fatalf("unexpected routing value: %v", rule)
					}
				}
			}
			want := 0
			if account != "" {
				want = 1
			}
			if count != want {
				t.Fatalf("job %v: got %d routing rules, want %d", job["job_name"], count, want)
			}
		}
	}
}
