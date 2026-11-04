package settings_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

func TestCustomerHelmSettings(t *testing.T) {
	_, source, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", ".."))
	chartSource := filepath.Join(root, "deploy", "chart", "prometheus-collector")
	valuesPath := filepath.Join(root, "test", "scenarios", "settings", "helm-customer-values", "values.yaml")
	data, err := os.ReadFile(valuesPath)
	if err != nil {
		t.Fatal(err)
	}
	var values struct {
		Alias     string            `json:"clusterAlias"`
		Targets   map[string]bool   `json:"scrapeTargets"`
		Intervals map[string]string `json:"scrapeIntervalTargets"`
		KeepLists map[string]string `json:"keepListRegexes"`
	}
	if err := yaml.Unmarshal(data, &values); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name     string
		template string
	}{
		{"settings", "prometheus-collector-settings-configmap.yaml"},
		{"deployment", "prometheus-collector-deployment.yaml"},
		{"advanced daemonset", "prometheus-collector-daemonset.yaml"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			selected := t.TempDir()
			if err := os.Mkdir(filepath.Join(selected, "templates"), 0700); err != nil {
				t.Fatal(err)
			}
			// Optional subcharts are separate; render the real customer templates offline.
			if err := os.WriteFile(filepath.Join(selected, "Chart.yaml"), []byte("apiVersion: v2\nname: prometheus-collector\nversion: 0.1.0\n"), 0600); err != nil {
				t.Fatal(err)
			}
			for from, to := range map[string]string{
				filepath.Join(chartSource, "values-template.yaml"):      filepath.Join(selected, "values.yaml"),
				filepath.Join(chartSource, "templates", "_helpers.tpl"): filepath.Join(selected, "templates", "_helpers.tpl"),
				filepath.Join(chartSource, "templates", tt.template):    filepath.Join(selected, "templates", tt.template),
			} {
				content, err := os.ReadFile(from)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(to, content, 0600); err != nil {
					t.Fatal(err)
				}
			}
			args := []string{"template", "customer-settings", selected, "--namespace", "settings-test", "--values", valuesPath}
			if tt.name != "settings" {
				args = append(args,
					"--set-string", "prometheus-node-exporter.service.targetPort=19100",
					"--set-string", "prometheus-node-exporter.image.repository=test/node",
					"--set-string", "prometheus-node-exporter.image.tag=test",
					"--set-string", "kube-state-metrics.image.repository=test/ksm",
					"--set-string", "kube-state-metrics.image.tag=test")
			}
			rendered, err := exec.Command("helm", args...).CombinedOutput()
			if err != nil {
				t.Fatalf("helm template: %v\n%s", err, rendered)
			}
			if tt.name == "settings" {
				var cm struct {
					Data map[string]string `json:"data"`
				}
				if err := yaml.Unmarshal(rendered, &cm); err != nil {
					t.Fatal(err)
				}
				if !slices.Contains(strings.Split(cm.Data["prometheus-collector-settings"], "\n"), `cluster_alias = "`+values.Alias+`"`) {
					t.Errorf("cluster alias was not rendered: %s", cm.Data["prometheus-collector-settings"])
				}
				for helmKey, configKey := range map[string]string{
					"coreDns": "coredns", "kubelet": "kubelet", "cAdvisor": "cadvisor",
					"kubeProxy": "kubeproxy", "apiServer": "apiserver", "kubeState": "kubestate",
					"nodeExporter": "nodeexporter", "windowsExporter": "windowsexporter", "windowsKubeProxy": "windowskubeproxy",
					"prometheusCollectorHealth": "prometheuscollectorhealth",
				} {
					wantTarget := configKey + " = false"
					if values.Targets[helmKey] {
						wantTarget = configKey + " = true"
					}
					expected := map[string]string{
						"default-scrape-settings-enabled":          wantTarget,
						"default-targets-scrape-interval-settings": configKey + ` = "` + values.Intervals[helmKey] + `"`,
					}
					if regex, ok := values.KeepLists[helmKey]; ok {
						expected["default-targets-metrics-keep-list"] = configKey + ` = "` + regex + `"`
					}
					for section, want := range expected {
						if !slices.Contains(strings.Split(cm.Data[section], "\n"), want) {
							t.Errorf("%s missing %s", section, want)
						}
					}
				}
				if !slices.Contains(strings.Split(cm.Data["debug-mode"], "\n"), "enabled = true") {
					t.Error("debug mode was not enabled")
				}
				return
			}
			workloads, linux, windows := 0, 0, 0
			for _, doc := range strings.Split(string(rendered), "\n---") {
				if strings.TrimSpace(doc) == "" {
					continue
				}
				var workload struct {
					Kind string `json:"kind"`
					Spec struct {
						Progress int                    `json:"progressDeadlineSeconds"`
						Template corev1.PodTemplateSpec `json:"template"`
					} `json:"spec"`
				}
				if err := yaml.Unmarshal([]byte(doc), &workload); err != nil {
					t.Fatal(err)
				}
				workloads++
				if len(workload.Spec.Template.Spec.Containers) != 1 {
					t.Fatal("expected one customer collector container")
				}
				containerName := workload.Spec.Template.Spec.Containers[0].Name
				switch containerName {
				case "prometheus-collector":
					linux++
				case "prometheus-collector-win":
					windows++
				default:
					t.Errorf("unexpected customer collector: %s", containerName)
				}
				if containerName == "prometheus-collector" {
					if workload.Spec.Template.Labels["settings-test"] != "customer" || workload.Spec.Template.Annotations["settings-test"] != "customer" {
						t.Error("customer pod labels/annotations missing on Linux workload")
					}
					if workload.Spec.Template.Spec.PriorityClassName != "settings-priority" {
						t.Error("customer priority class missing on Linux workload")
					}
				}
				if tt.name == "deployment" && workload.Spec.Progress != 700 {
					t.Error("customer progress deadline missing")
				}
				if tt.name == "deployment" {
					if workload.Kind != "Deployment" {
						t.Error("customer deployment did not render a Deployment")
					}
					requests := workload.Spec.Template.Spec.Containers[0].Resources.Requests
					if requests.Cpu().String() != "250m" || requests.Memory().String() != "512Mi" {
						t.Error("customer deployment resource requests missing")
					}
				}
				if tt.name == "advanced daemonset" && workload.Kind != "DaemonSet" {
					t.Error("advanced mode did not render a DaemonSet")
				}
				want := map[string]string{"MODE": "advanced", "WINMODE": "advanced"}
				for _, container := range workload.Spec.Template.Spec.Containers {
					for _, env := range container.Env {
						if env.Name == "MINIMAL_INGESTION_PROFILE" {
							t.Error("disabled minimal ingestion should not set its enable-only environment variable")
						}
						if value, ok := want[env.Name]; ok && env.Value == value {
							delete(want, env.Name)
						}
					}
				}
				if len(want) != 0 {
					t.Errorf("missing customer settings in workload environment: %v", want)
				}
			}
			expected := 1
			if tt.name == "advanced daemonset" {
				expected = 2
			}
			if workloads != expected {
				t.Errorf("rendered %d workloads, want %d", workloads, expected)
			}
			if linux != 1 || windows != expected-1 {
				t.Errorf("rendered Linux=%d Windows=%d collectors, want 1 and %d", linux, windows, expected-1)
			}
		})
	}
}
