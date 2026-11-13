package testkube

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"
)

func TestConfigPipelineUsesSharedWorkflowAndSummarySteps(t *testing.T) {
	type workflowCase struct {
		scenario, displayName, summaryDisplayName string
	}
	expected := map[string]workflowCase{
		"testkube-config-test-no-configmaps-crs.yaml":               {"no-configmaps", "Run tests for no configmaps", ""},
		"testkube-config-test-all-targets-disabled-crs.yaml":        {"all-targets-disabled", "Run tests for all targets disabled", ""},
		"testkube-config-test-default-targets-on-crs.yaml":          {"default-targets-on", "Run tests for all default targets in settings cfg map enabled", ""},
		"testkube-config-test-all-rs-targets-enabled-crs.yaml":      {"all-rs-targets-enabled", "Run tests for all rs targets in settings cfg map enabled", ""},
		"testkube-config-test-all-ds-targets-enabled-crs.yaml":      {"all-ds-targets-enabled", "Run tests for all ds targets in settings cfg map enabled", ""},
		"testkube-config-test-all-targets-enabled-crs.yaml":         {"all-targets-enabled", "Run tests for all targets in settings cfg map enabled", ""},
		"testkube-config-test-only-custom-configmap-crs.yaml":       {"only-custom-configmap", "Run tests for custom configmap with all actions enabled", ""},
		"testkube-config-test-global-settings-crs.yaml":             {"global-settings", "Run tests for global ext labels enabled", ""},
		"testkube-config-test-custom-node-configmap-crs.yaml":       {"custom-node-configmap", "Run tests for custom node configmap enabled", ""},
		"testkube-config-test-basic-auth-smon-crs.yaml":             {"basic-auth-smon", "Run tests for basic auth ServiceMonitor", ""},
		"testkube-config-test-basic-auth-secret-update-crs.yaml":    {"basic-auth-secret-update", "Run tests for basic auth secret update", "Create TestKube Results Summary and cleanup basic-auth resources"},
		"testkube-config-test-basic-auth-smon-v2-crs.yaml":          {"basic-auth-smon-v2", "Run tests for basic auth ServiceMonitor (v2 configmap)", ""},
		"testkube-config-test-basic-auth-secret-update-v2-crs.yaml": {"basic-auth-secret-update-v2", "Run tests for basic auth secret update (v2 configmap)", "Create TestKube Results Summary and cleanup basic-auth v2 resources"},
		"testkube-config-test-settings-error-crs.yaml":              {"settings-error", "Run tests for errorprone settings configmap", ""},
		"testkube-config-test-custom-configmap-error-crs.yaml":      {"custom-configmap-error", "Run tests for errorprone custom configmap", ""},
		"testkube-config-test-global-ext-labels-error-crs.yaml":     {"global-ext-labels-error", "Run tests for errorprone global ext labels", ""},
		"testkube-config-test-controlplane-istio-crs.yaml":          {"controlplane-istio", "Run tests for controlplane-istio with custom interval and regex", "Create TestKube Results Summary and cleanup controlplane-istio env"},
		"testkube-config-test-default-targets-on-v2-crs.yaml":       {"default-targets-on-v2", "Run tests for all default targets in settings cfg map enabled (v2)", ""},
		"testkube-config-test-mip-false-crs.yaml":                   {"mip-false", "Run tests for minimal ingestion profile false", ""},
		"testkube-config-test-deny-fs-access-crs.yaml":              {"deny-fs-access", "Run tests for deny_fs_access_through_sms", ""},
		"testkube-config-test-partial-no-pcs-v1-crs.yaml":           {"partial-no-pcs-v1", "Run tests for partial v1 configmap (no prometheus-collector-settings)", "Create TestKube Results Summary (partial v1)"},
		"testkube-config-test-partial-no-pcs-v2-crs.yaml":           {"partial-no-pcs-v2", "Run tests for partial v2 configmap (no prometheus-collector-settings)", "Create TestKube Results Summary and restore default settings configmap"},
	}

	_, source, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(source), "..", "..", "..")
	pipelineData, err := os.ReadFile(filepath.Join(root, ".pipelines", "azure-pipeline-config-tests.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var pipeline struct {
		Stages []struct {
			Jobs []struct {
				Strategy struct {
					RunOnce struct {
						Deploy struct {
							Steps []struct {
								Template   string `json:"template"`
								Parameters struct {
									WorkflowFile        string `json:"workflowFile"`
									Scenario            string `json:"scenario"`
									DisplayName         string `json:"displayName"`
									SummaryDisplayName  string `json:"summaryDisplayName"`
									AMWQueryEndpoint    string `json:"AMW_QUERY_ENDPOINT"`
									AzureClientID       string `json:"AZURE_CLIENT_ID"`
									Branch              string `json:"branch"`
									SetupScript         string `json:"setupScript"`
									BeforeSummaryScript string `json:"beforeSummaryScript"`
									AfterSummaryScript  string `json:"afterSummaryScript"`
								} `json:"parameters"`
							} `json:"steps"`
						} `json:"deploy"`
					} `json:"runOnce"`
				} `json:"strategy"`
			} `json:"jobs"`
		} `json:"stages"`
	}
	if err := yaml.Unmarshal(pipelineData, &pipeline); err != nil {
		t.Fatal(err)
	}
	actual := make(map[string]struct {
		workflowCase
		setup, beforeSummary, afterSummary string
	})
	for _, stage := range pipeline.Stages {
		for _, job := range stage.Jobs {
			for _, step := range job.Strategy.RunOnce.Deploy.Steps {
				if filepath.Base(step.Template) != "azure-template-config-test-case.yml" {
					continue
				}
				p := step.Parameters
				if p.AMWQueryEndpoint != "${{ parameters.AMW_QUERY_ENDPOINT }}" ||
					p.AzureClientID != "${{ parameters.AZURE_CLIENT_ID }}" ||
					p.Branch != "${{ parameters.branch }}" {
					t.Errorf("%s does not forward the pipeline's endpoint, client ID and branch", p.WorkflowFile)
				}
				if _, exists := actual[p.WorkflowFile]; exists {
					t.Errorf("duplicate ConfigTests pipeline mapping for %s", p.WorkflowFile)
				}
				actual[p.WorkflowFile] = struct {
					workflowCase
					setup, beforeSummary, afterSummary string
				}{
					workflowCase:  workflowCase{p.Scenario, p.DisplayName, p.SummaryDisplayName},
					setup:         p.SetupScript,
					beforeSummary: p.BeforeSummaryScript,
					afterSummary:  p.AfterSummaryScript,
				}
			}
		}
	}
	if len(actual) != len(expected) {
		t.Fatalf("found %d shared ConfigTests pipeline cases; want %d", len(actual), len(expected))
	}
	for file, want := range expected {
		if got, ok := actual[file]; !ok || got.workflowCase != want {
			t.Errorf("%s maps to %+v; want %+v", file, got.workflowCase, want)
		}
	}

	templateData, err := os.ReadFile(filepath.Join(root, ".pipelines", "azure-template-config-test-case.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var template struct {
		Parameters []struct {
			Name string `json:"name"`
		} `json:"parameters"`
		Steps []struct {
			Bash            string `json:"bash"`
			ContinueOnError bool   `json:"continueOnError"`
			Condition       string `json:"condition"`
		} `json:"steps"`
	}
	if err := yaml.Unmarshal(templateData, &template); err != nil {
		t.Fatal(err)
	}
	parameters := make(map[string]bool)
	for _, parameter := range template.Parameters {
		parameters[parameter.Name] = true
	}
	for _, name := range []string{"AMW_QUERY_ENDPOINT", "AZURE_CLIENT_ID", "branch"} {
		if !parameters[name] {
			t.Errorf("shared pipeline template does not declare parameter %q", name)
		}
	}
	if len(template.Steps) != 2 {
		t.Fatalf("shared pipeline template has %d steps; want run and summary", len(template.Steps))
	}
	run := template.Steps[0]
	setup := strings.Index(run.Bash, "${{ parameters.setupScript }}")
	invoke := strings.Index(run.Bash, "./testkube/run-testkube-workflow.sh")
	if !run.ContinueOnError || setup < 0 || invoke < 0 || setup > invoke {
		t.Fatalf("shared run step changed setup order or failure continuation: %+v", run)
	}
	summary := template.Steps[1]
	before := strings.Index(summary.Bash, "${{ parameters.beforeSummaryScript }}")
	write := strings.Index(summary.Bash, "TESTKUBE_RESULTS_OTEL=")
	remove := strings.Index(summary.Bash, "rm -f")
	after := strings.Index(summary.Bash, "${{ parameters.afterSummaryScript }}")
	if summary.Condition != "always()" || before < 0 || write < 0 || remove < 0 || after < 0 || !(before < write && write < remove && remove < after) {
		t.Fatalf("shared summary step changed lifecycle order or always condition: %+v", summary)
	}
	if !strings.Contains(actual["testkube-config-test-controlplane-istio-crs.yaml"].beforeSummary, "kubectl rollout status") ||
		!strings.Contains(actual["testkube-config-test-basic-auth-secret-update-crs.yaml"].afterSummary, "kubectl delete namespace basic-auth-test") ||
		!strings.Contains(actual["testkube-config-test-partial-no-pcs-v2-crs.yaml"].afterSummary, "ama-metrics-settings-configmap.yaml") {
		t.Fatal("scenario-specific cleanup no longer surrounds result generation in its original order")
	}
}
