package helmchart_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"text/template"

	sprig "github.com/go-task/slim-sprig/v3"
	"sigs.k8s.io/yaml"
)

func chartDirectory() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "deploy", "addon-chart", "azure-monitor-metrics-addon")
}

type apiVersions []string

func (versions apiVersions) Has(version string) bool {
	return slices.Contains(versions, version)
}

func TestHPALookupCompatibility(t *testing.T) {
	for _, tt := range []struct {
		name        string
		apiVersions apiVersions
		existingHPA map[string]interface{}
		wantMin     int
		wantMax     int
	}{
		{name: "autoscaling v2 unavailable", wantMin: 2, wantMax: 50},
		{name: "autoscaling v2 available without existing HPA", apiVersions: apiVersions{"autoscaling/v2"}, wantMin: 2, wantMax: 50},
		{
			name:        "preserve customized HPA",
			apiVersions: apiVersions{"autoscaling/v2"},
			existingHPA: map[string]interface{}{"spec": map[string]interface{}{"minReplicas": 3, "maxReplicas": 20}},
			wantMin:     3,
			wantMax:     20,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			lookupCalls := 0
			funcs := sprig.TxtFuncMap()
			// Unlike helm template's offline lookup, fail if the unavailable API is queried.
			funcs["lookup"] = func(apiVersion, kind, namespace, name string) (map[string]interface{}, error) {
				lookupCalls++
				if !tt.apiVersions.Has(apiVersion) {
					return nil, fmt.Errorf("API %s is unavailable", apiVersion)
				}
				if kind != "HorizontalPodAutoscaler" || namespace != "kube-system" || name != "ama-metrics-hpa" {
					return nil, fmt.Errorf("unexpected lookup: %s %s %s", kind, namespace, name)
				}
				return tt.existingHPA, nil
			}
			helper, err := template.New("helpers").Funcs(funcs).ParseFiles(filepath.Join(chartDirectory(), "templates", "_ama-metrics-helpers.tpl"))
			if err != nil {
				t.Fatal(err)
			}
			var rendered bytes.Buffer
			err = helper.ExecuteTemplate(&rendered, "ama-metrics-merge-custom-hpa", map[string]interface{}{
				"Values":       map[string]interface{}{"AzureMonitorMetrics": map[string]interface{}{}},
				"Capabilities": map[string]interface{}{"APIVersions": tt.apiVersions},
			})
			if err != nil {
				t.Fatal(err)
			}
			wantCalls := 0
			if tt.apiVersions.Has("autoscaling/v2") {
				wantCalls = 1
			}
			if lookupCalls != wantCalls {
				t.Fatalf("lookup called %d times, want %d", lookupCalls, wantCalls)
			}
			var values struct {
				Min int `json:"amaMetricsMinReplicasFromHelper"`
				Max int `json:"amaMetricsMaxReplicasFromHelper"`
			}
			if err := yaml.Unmarshal(rendered.Bytes(), &values); err != nil {
				t.Fatal(err)
			}
			if values.Min != tt.wantMin || values.Max != tt.wantMax {
				t.Fatalf("HPA replicas = %d..%d, want %d..%d", values.Min, values.Max, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestKSMArguments(t *testing.T) {
	for _, tt := range []struct {
		name   string
		values map[string][]string
	}{
		{name: "defaults"},
		{
			name: "ordinary lists",
			values: map[string][]string{
				"Collectors":                 {"pods", "nodes"},
				"MetricLabelsAllowList":      {"pods=[app]", "nodes=[zone]"},
				"MetricAnnotationsAllowList": {"pods=[annotation]"},
				"MetricAllowList":            {"kube_pod_info", "kube_node_info"},
			},
		},
		{name: "resources with YAML punctuation", values: map[string][]string{"Collectors": {`pods: selected`}}},
		{name: "labels with YAML punctuation", values: map[string][]string{"MetricLabelsAllowList": {`pods=[app: foo,"quoted",path\value]`}}},
		{name: "annotations with YAML punctuation", values: map[string][]string{"MetricAnnotationsAllowList": {`pods=[annotation: foo,"quoted",path\value]`}}},
		{name: "metrics with YAML punctuation", values: map[string][]string{"MetricAllowList": {`kube_pod_info: sample`}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			chart := t.TempDir()
			if err := os.Mkdir(filepath.Join(chart, "templates"), 0700); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"_ama-metrics-helpers.tpl", "_arc-extension-helpers.tpl", "ama-metrics-ksm-deployment.yaml"} {
				data, err := os.ReadFile(filepath.Join(chartDirectory(), "templates", name))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(chart, "templates", name), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(chart, "Chart.yaml"), []byte("apiVersion: v2\nname: chart-regression-tests\nversion: 0.1.0\n"), 0600); err != nil {
				t.Fatal(err)
			}
			defaults, err := os.ReadFile(filepath.Join(chartDirectory(), "values-template.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			var values map[string]interface{}
			if err := yaml.Unmarshal(defaults, &values); err != nil {
				t.Fatal(err)
			}
			metrics := values["AzureMonitorMetrics"].(map[string]interface{})
			metrics["IncludeDependentCharts"] = false
			ksm := metrics["KubeStateMetrics"].(map[string]interface{})
			expected := map[string][]string{}
			for _, collector := range ksm["Collectors"].([]interface{}) {
				expected["Collectors"] = append(expected["Collectors"], collector.(string))
			}
			for key, value := range tt.values {
				ksm[key] = value
				expected[key] = value
			}
			data, err := yaml.Marshal(values)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(chart, "values.yaml"), data, 0600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("helm", "template", "ama-metrics", chart, "--show-only", "templates/ama-metrics-ksm-deployment.yaml")
			rendered, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("helm template: %v\n%s", err, rendered)
			}
			var deployment struct {
				Spec struct {
					Template struct {
						Spec struct {
							Containers []struct {
								Name string   `json:"name"`
								Args []string `json:"args"`
							} `json:"containers"`
						} `json:"spec"`
					} `json:"template"`
				} `json:"spec"`
			}
			if err := yaml.Unmarshal(rendered, &deployment); err != nil {
				t.Fatalf("deployment args must decode as strings: %v\n%s", err, rendered)
			}
			containers := deployment.Spec.Template.Spec.Containers
			if len(containers) != 1 || containers[0].Name != "ama-metrics-ksm" {
				t.Fatalf("unexpected KSM containers: %+v", containers)
			}
			for key, flag := range map[string]string{
				"Collectors":                 "--resources",
				"MetricLabelsAllowList":      "--metric-labels-allowlist",
				"MetricAnnotationsAllowList": "--metric-annotations-allowlist",
				"MetricAllowList":            "--metric-allowlist",
			} {
				if items, ok := expected[key]; ok {
					want := flag + "=" + strings.Join(items, ",")
					if !slices.Contains(containers[0].Args, want) {
						t.Errorf("args %q do not contain unchanged argument %q", containers[0].Args, want)
					}
				} else {
					for _, arg := range containers[0].Args {
						if strings.HasPrefix(arg, flag+"=") {
							t.Errorf("unset option %s produced argument %q", key, arg)
						}
					}
				}
			}
		})
	}
}
