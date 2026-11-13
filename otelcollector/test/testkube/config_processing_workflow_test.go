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
	data, err := os.ReadFile(filepath.Join(root, "testkube-config-test-ginkgo-template-crs.yaml"))
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
	if err := yaml.Unmarshal(data, &template); err != nil {
		t.Fatal(err)
	}
	if template.Kind != "TestWorkflowTemplate" || template.Metadata.Name != "configprocessingginkgo" || template.Metadata.Namespace != "testkube" {
		t.Fatalf("unexpected shared template identity: %+v", template.Metadata)
	}
	for _, name := range []string{"labelFilter", "timeoutFlag", "parallelFlag", "verboseFlag"} {
		if template.Spec.Config[name].Type != "string" || (name != "labelFilter" && template.Spec.Config[name].Default != "") {
			t.Errorf("template config %q must be a string with optional flags defaulting to empty", name)
		}
	}
	if template.Spec.Content.Git.URI != "https://github.com/Azure/prometheus-collector" ||
		template.Spec.Content.Git.Revision != "$BRANCH_NAME" ||
		len(template.Spec.Content.Git.Paths) != 1 || template.Spec.Content.Git.Paths[0] != "otelcollector/test/ginkgo-e2e" {
		t.Fatalf("shared template changed the Ginkgo source: %+v", template.Spec.Content.Git)
	}
	if template.Spec.Container.WorkingDir != "/data/repo/otelcollector/test/ginkgo-e2e" ||
		template.Spec.Container.Image != "cerebro31/ginkgo-test:latest" ||
		len(template.Spec.Steps) != 2 || template.Spec.Steps[0].Name != "Run test" {
		t.Fatalf("shared template changed its execution defaults: %+v", template.Spec)
	}
	command := template.Spec.Steps[0].Shell
	for _, part := range []string{
		"--fail-on-empty {{ config.timeoutFlag }}",
		"--junit-report=/data/reports/junit.xml",
		"--json-report=/data/reports/ginkgo.json",
		"-r {{ config.verboseFlag }} {{ config.parallelFlag }}",
		`--label-filter "{{ config.labelFilter }}"`,
	} {
		if !strings.Contains(command, part) {
			t.Errorf("shared Ginkgo command is missing %q", part)
		}
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
}

func TestConfigProcessingWorkflowMappings(t *testing.T) {
	type workflowCase struct {
		label, timeout, parallel, verbose string
	}
	const common = "(config-processing-common, config-processing-common-with-config-maps)"
	const errorCommon = "(config-processing-common, config-processing-common-with-error-config-maps)"
	workflowFiles := map[string]map[string]workflowCase{
		"testkube-config-test-all-ds-targets-enabled-crs.yaml": {
			"configprocessingcommon": {common, "", "-p", ""},
			"alldstargetsenabled":    {"(config-processing-ds-targets-enabled)", "", "-p", ""},
		},
		"testkube-config-test-all-rs-targets-enabled-crs.yaml": {
			"configprocessingcommon": {common, "", "-p", ""},
			"allrstargetsenabled":    {"(config-processing-rs-targets-enabled)", "", "-p", ""},
		},
		"testkube-config-test-all-targets-disabled-crs.yaml": {
			"configprocessingcommon": {common, "", "-p", ""},
			"alltargetsdisabled":     {"(config-processing-all-targets-disabled)", "", "-p", ""},
		},
		"testkube-config-test-all-targets-enabled-crs.yaml": {
			"configprocessingcommon": {"(config-processing-common, config-processing-common-with-config-maps)", "", "-p", ""},
			"alltargetsenabled":      {"(config-processing-all-targets-enabled)", "", "-p", ""},
		},
		"testkube-config-test-basic-auth-secret-update-crs.yaml": {
			"basicauthsecretupdate": {"(config-processing-basic-auth-secret-update)", "", "-p", ""},
		},
		"testkube-config-test-basic-auth-secret-update-v2-crs.yaml": {
			"basicauthsecretupdatev2": {"(config-processing-basic-auth-secret-update-v2)", "", "-p", "-v"},
		},
		"testkube-config-test-basic-auth-smon-crs.yaml": {
			"basicauthsmon": {"(config-processing-basic-auth-smon)", "--timeout=3h", "-p", "-v"},
		},
		"testkube-config-test-basic-auth-smon-v2-crs.yaml": {
			"basicauthsmonv2": {"(config-processing-basic-auth-smon-v2)", "--timeout=3h", "-p", "-v"},
		},
		"testkube-config-test-controlplane-istio-crs.yaml": {
			"configprocessingcommon":   {common, "", "-p", ""},
			"controlplaneistioenabled": {"(config-processing-controlplane-istio-enabled)", "", "-p", ""},
		},
		"testkube-config-test-custom-configmap-error-crs.yaml": {
			"configprocessingcommon": {errorCommon, "", "-p", ""},
			"customconfigmaperror":   {"(config-processing-custom-config-map-error)", "", "-p", ""},
		},
		"testkube-config-test-custom-node-configmap-crs.yaml": {
			"configprocessingcommon": {common, "", "-p", ""},
			"customnodeconfigmaps":   {"(config-processing-settings-node-config-map)", "", "-p", ""},
		},
		"testkube-config-test-customer-settings-v1-crs.yaml": {
			"customersettingsv1": {"config-processing-customer-settings-v1", "--timeout=3h", "", ""},
		},
		"testkube-config-test-customer-settings-v2-crs.yaml": {
			"customersettingsv2": {"config-processing-customer-settings-v2", "--timeout=3h", "", ""},
		},
		"testkube-config-test-default-targets-on-crs.yaml": {
			"configprocessingcommon": {common, "", "-p", ""},
			"defaultsettingsenabled": {"(config-processing-default-targets-enabled)", "", "-p", ""},
		},
		"testkube-config-test-default-targets-on-v2-crs.yaml": {
			"configprocessingcommonv2": {"(config-processing-common, config-processing-common-with-config-maps-v2)", "", "-p", ""},
			"defaultsettingsenabledv2": {"(config-processing-default-targets-enabled)", "", "-p", ""},
		},
		"testkube-config-test-deny-fs-access-crs.yaml": {
			"denyfsaccess": {"(config-processing-deny-fs-access)", "", "-p", "-v"},
		},
		"testkube-config-test-global-ext-labels-error-crs.yaml": {
			"configprocessingcommon": {errorCommon, "", "-p", ""},
			"extlabelserror":         {"(config-processing-global-settings-error)", "", "-p", ""},
		},
		"testkube-config-test-global-settings-crs.yaml": {
			"configprocessingcommon": {common, "", "-p", ""},
			"globalextlabels":        {"(config-processing-global-settings)", "", "-p", ""},
		},
		"testkube-config-test-mip-false-crs.yaml": {
			"configprocessingmipfalse": {"(config-processing-common, config-processing-mip-false)", "", "-p", ""},
			"mipfalse":                 {"(config-processing-mip-false)", "", "-p", ""},
		},
		"testkube-config-test-no-configmaps-crs.yaml": {
			"configprocessingcommon": {"(config-processing-common, config-processing-common-no-config-maps)", "", "-p", ""},
			"noconfigmaps":           {"(config-processing-no-config-maps)", "", "-p", ""},
		},
		"testkube-config-test-only-custom-configmap-crs.yaml": {
			"configprocessingcommon":    {common, "", "-p", ""},
			"customconfigmapallactions": {"(config-processing-only-config-map-all-actions)", "", "-p", ""},
		},
		"testkube-config-test-partial-no-pcs-v1-crs.yaml": {
			"partialnopcsv1": {"(config-processing-common, config-processing-partial-no-pcs-v1)", "", "-p", ""},
		},
		"testkube-config-test-partial-no-pcs-v2-crs.yaml": {
			"partialnopcsv2": {"(config-processing-common, config-processing-partial-no-pcs-v2)", "", "-p", ""},
		},
		"testkube-config-test-settings-error-crs.yaml": {
			"configprocessingcommon": {common, "", "-p", ""},
			"settingserror":          {"(config-processing-settings-error)", "", "-p", ""},
		},
	}

	_, source, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(source), "config-processing-test-crs")
	files, err := filepath.Glob(filepath.Join(root, "testkube-config-test-*-crs.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	commonEnv := [][2]string{{"AMW_QUERY_ENDPOINT", "$AMW_QUERY_ENDPOINT"}, {"AZURE_CLIENT_ID", "$AZURE_CLIENT_ID"}, {"GOTOOLCHAIN", "auto"}}
	goEnv := [][2]string{{"GOTOOLCHAIN", "auto"}}
	foundFiles := make(map[string]bool)
	count := 0
	for _, path := range files {
		file := filepath.Base(path)
		expected, ok := workflowFiles[file]
		if !ok {
			if strings.Contains(file, "ginkgo-template") {
				continue
			}
			t.Errorf("ConfigProcessing workflow file %s has no mapping", file)
			continue
		}
		foundFiles[file] = true
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		seen := make(map[string]bool)
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
			if workflow.Kind != "TestWorkflow" {
				continue
			}
			count++
			want, ok := expected[workflow.Metadata.Name]
			if !ok {
				t.Errorf("%s has unexpected workflow %q", file, workflow.Metadata.Name)
				continue
			}
			seen[workflow.Metadata.Name] = true
			if workflow.Metadata.Namespace != "testkube" ||
				workflow.Metadata.Labels["executor"] != "ginkgo-executor" || workflow.Metadata.Labels["test-type"] != "ginkgo-test" {
				t.Errorf("%s/%s changed workflow identity labels", file, workflow.Metadata.Name)
			}
			if len(workflow.Spec.Use) != 1 || workflow.Spec.Use[0].Name != "configprocessingginkgo" {
				t.Errorf("%s/%s does not use the shared Ginkgo template", file, workflow.Metadata.Name)
				continue
			}
			config := workflow.Spec.Use[0].Config
			if config["labelFilter"] != want.label || config["timeoutFlag"] != want.timeout ||
				config["parallelFlag"] != want.parallel || config["verboseFlag"] != want.verbose {
				t.Errorf("%s/%s changed Ginkgo filter/options: got %+v", file, workflow.Metadata.Name, config)
			}
			env := commonEnv
			if strings.Contains(file, "customer-settings") {
				env = goEnv
			}
			if len(workflow.Spec.Container.Env) != len(env) {
				t.Errorf("%s/%s changed Ginkgo environment: %+v", file, workflow.Metadata.Name, workflow.Spec.Container.Env)
			} else {
				for i, value := range env {
					if workflow.Spec.Container.Env[i].Name != value[0] || workflow.Spec.Container.Env[i].Value != value[1] {
						t.Errorf("%s/%s changed Ginkgo environment: %+v", file, workflow.Metadata.Name, workflow.Spec.Container.Env)
						break
					}
				}
			}
			if strings.Contains(document, "content:") || strings.Contains(document, "steps:") || strings.Contains(document, "ginkgo run") {
				t.Errorf("%s/%s duplicates shared execution fields", file, workflow.Metadata.Name)
			}
		}
		if len(seen) != len(expected) {
			t.Errorf("%s has %d workflows; want %d", file, len(seen), len(expected))
		}
	}
	if len(foundFiles) != len(workflowFiles) || count == 0 {
		t.Fatalf("found %d mapped workflow files and %d workflows; want %d files", len(foundFiles), count, len(workflowFiles))
	}
}
