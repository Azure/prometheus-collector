package testkube

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"
)

func TestConfigProcessingWorkflowsUseSharedTemplate(t *testing.T) {
	_, source, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(source), "config-processing-test-crs")
	templatePath := filepath.Join(root, "testkube-config-test-ginkgo-template-crs.yaml")
	templateData, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatal(err)
	}
	var template struct {
		Kind     string `json:"kind"`
		Metadata struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"metadata"`
		Spec struct {
			Config map[string]struct {
				Type    string `json:"type"`
				Default string `json:"default"`
			} `json:"config"`
			Content struct {
				Git struct {
					URI      string   `json:"uri"`
					Revision string   `json:"revision"`
					Paths    []string `json:"paths"`
				} `json:"git"`
			} `json:"content"`
			Container struct {
				WorkingDir string `json:"workingDir"`
				Image      string `json:"image"`
			} `json:"container"`
			Steps []struct {
				Name      string `json:"name"`
				Condition string `json:"condition"`
				Shell     string `json:"shell"`
				Artifacts struct {
					WorkingDir string   `json:"workingDir"`
					Paths      []string `json:"paths"`
				} `json:"artifacts"`
			} `json:"steps"`
			Pod struct {
				NodeSelector map[string]string `json:"nodeSelector"`
			} `json:"pod"`
		} `json:"spec"`
	}
	if err := yaml.Unmarshal(templateData, &template); err != nil {
		t.Fatal(err)
	}
	if template.Kind != "TestWorkflowTemplate" || template.Metadata.Name != "configprocessingginkgo" || template.Metadata.Namespace != "testkube" {
		t.Fatalf("unexpected shared template identity: %+v", template.Metadata)
	}
	if template.Spec.Config["labelFilter"].Type != "string" ||
		template.Spec.Config["timeoutFlag"].Type != "string" || template.Spec.Config["timeoutFlag"].Default != "" ||
		template.Spec.Config["parallelFlag"].Type != "string" || template.Spec.Config["parallelFlag"].Default != "" {
		t.Fatalf("shared template config must expose string labelFilter and optional Ginkgo flags: %+v", template.Spec.Config)
	}
	if template.Spec.Content.Git.URI != "https://github.com/Azure/prometheus-collector" ||
		template.Spec.Content.Git.Revision != "$BRANCH_NAME" ||
		len(template.Spec.Content.Git.Paths) != 1 || template.Spec.Content.Git.Paths[0] != "otelcollector/test/ginkgo-e2e" {
		t.Fatalf("shared template changed the Ginkgo source: %+v", template.Spec.Content.Git)
	}
	if template.Spec.Container.WorkingDir != "/data/repo/otelcollector/test/ginkgo-e2e" ||
		template.Spec.Container.Image != "cerebro31/ginkgo-test:latest" {
		t.Fatalf("shared template changed its container defaults: %+v", template.Spec.Container)
	}
	if len(template.Spec.Steps) != 2 {
		t.Fatalf("shared template has %d steps, want test and evidence steps", len(template.Spec.Steps))
	}
	run := template.Spec.Steps[0]
	ginkgoCommand := ""
	for _, line := range strings.Split(run.Shell, "\n") {
		if command := strings.TrimSpace(line); strings.HasPrefix(command, "ginkgo run ") {
			ginkgoCommand = command
			break
		}
	}
	if run.Name != "Run test" ||
		!strings.Contains(ginkgoCommand, "--fail-on-empty {{ config.timeoutFlag }}") ||
		!strings.Contains(ginkgoCommand, `--label-filter "{{ config.labelFilter }}"`) ||
		!strings.Contains(ginkgoCommand, "--junit-report=/data/reports/junit.xml") ||
		!strings.Contains(ginkgoCommand, "--json-report=/data/reports/ginkgo.json") ||
		!strings.Contains(ginkgoCommand, "-r {{ config.parallelFlag }}") {
		t.Fatalf("shared Ginkgo step changed its execution contract: %+v", run)
	}
	evidence := template.Spec.Steps[1]
	if evidence.Name != "Retain Ginkgo evidence" || evidence.Condition != "always" ||
		evidence.Artifacts.WorkingDir != "/data/reports" ||
		len(evidence.Artifacts.Paths) != 1 || evidence.Artifacts.Paths[0] != "**/*" {
		t.Fatalf("shared evidence step no longer retains reports on every outcome: %+v", evidence)
	}
	if template.Spec.Pod.NodeSelector["kubernetes.io/os"] != "linux" || template.Spec.Pod.NodeSelector["kubernetes.io/arch"] != "amd64" {
		t.Fatalf("shared template changed its node selector: %+v", template.Spec.Pod.NodeSelector)
	}

	for _, tt := range []struct {
		file, name, label, timeoutFlag, parallelFlag string
		env                                          [][2]string
	}{
		{"testkube-config-test-customer-settings-v1-crs.yaml", "customersettingsv1", "config-processing-customer-settings-v1", "--timeout=3h", "", [][2]string{{"GOTOOLCHAIN", "auto"}}},
		{"testkube-config-test-customer-settings-v2-crs.yaml", "customersettingsv2", "config-processing-customer-settings-v2", "--timeout=3h", "", [][2]string{{"GOTOOLCHAIN", "auto"}}},
		{"testkube-config-test-all-targets-enabled-crs.yaml", "configprocessingcommon", "(config-processing-common, config-processing-common-with-config-maps)", "", "-p", [][2]string{{"AMW_QUERY_ENDPOINT", "$AMW_QUERY_ENDPOINT"}, {"AZURE_CLIENT_ID", "$AZURE_CLIENT_ID"}, {"GOTOOLCHAIN", "auto"}}},
		{"testkube-config-test-all-targets-enabled-crs.yaml", "alltargetsenabled", "(config-processing-all-targets-enabled)", "", "-p", [][2]string{{"AMW_QUERY_ENDPOINT", "$AMW_QUERY_ENDPOINT"}, {"AZURE_CLIENT_ID", "$AZURE_CLIENT_ID"}, {"GOTOOLCHAIN", "auto"}}},
		{"testkube-config-test-default-targets-on-crs.yaml", "configprocessingcommon", "(config-processing-common, config-processing-common-with-config-maps)", "", "-p", [][2]string{{"AMW_QUERY_ENDPOINT", "$AMW_QUERY_ENDPOINT"}, {"AZURE_CLIENT_ID", "$AZURE_CLIENT_ID"}, {"GOTOOLCHAIN", "auto"}}},
		{"testkube-config-test-default-targets-on-crs.yaml", "defaultsettingsenabled", "(config-processing-default-targets-enabled)", "", "-p", [][2]string{{"AMW_QUERY_ENDPOINT", "$AMW_QUERY_ENDPOINT"}, {"AZURE_CLIENT_ID", "$AZURE_CLIENT_ID"}, {"GOTOOLCHAIN", "auto"}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(root, tt.file))
			if err != nil {
				t.Fatal(err)
			}
			var found bool
			for _, document := range strings.Split(string(data), "---") {
				if strings.TrimSpace(document) == "" {
					continue
				}
				var workflow struct {
					Kind     string `json:"kind"`
					Metadata struct {
						Name      string            `json:"name"`
						Namespace string            `json:"namespace"`
						Labels    map[string]string `json:"labels"`
					} `json:"metadata"`
					Spec struct {
						Use []struct {
							Name   string            `json:"name"`
							Config map[string]string `json:"config"`
						} `json:"use"`
						Container struct {
							Env []struct {
								Name  string `json:"name"`
								Value string `json:"value"`
							} `json:"env"`
						} `json:"container"`
					} `json:"spec"`
				}
				if err := yaml.Unmarshal([]byte(document), &workflow); err != nil {
					t.Fatal(err)
				}
				if workflow.Metadata.Name != tt.name {
					continue
				}
				found = true
				if workflow.Kind != "TestWorkflow" || workflow.Metadata.Namespace != "testkube" ||
					workflow.Metadata.Labels["executor"] != "ginkgo-executor" || workflow.Metadata.Labels["test-type"] != "ginkgo-test" {
					t.Fatalf("unexpected workflow identity or labels: %+v", workflow.Metadata)
				}
				if len(workflow.Spec.Use) != 1 || workflow.Spec.Use[0].Name != "configprocessingginkgo" ||
					workflow.Spec.Use[0].Config["labelFilter"] != tt.label ||
					workflow.Spec.Use[0].Config["timeoutFlag"] != tt.timeoutFlag ||
					workflow.Spec.Use[0].Config["parallelFlag"] != tt.parallelFlag {
					t.Fatalf("workflow %s maps to the wrong shared template case: %+v", tt.name, workflow.Spec.Use)
				}
				if len(workflow.Spec.Container.Env) != len(tt.env) {
					t.Fatalf("workflow %s environment changed: %+v", tt.name, workflow.Spec.Container.Env)
				}
				for i, env := range tt.env {
					if workflow.Spec.Container.Env[i].Name != env[0] || workflow.Spec.Container.Env[i].Value != env[1] {
						t.Fatalf("workflow %s environment changed: %+v", tt.name, workflow.Spec.Container.Env)
					}
				}
				if strings.Contains(document, "ginkgo run") || strings.Contains(document, "content:") || strings.Contains(document, "steps:") {
					t.Fatalf("workflow %s duplicates the shared template body", tt.name)
				}
				break
			}
			if !found {
				t.Fatalf("workflow %s was not found in %s", tt.name, tt.file)
			}
		})
	}
}
