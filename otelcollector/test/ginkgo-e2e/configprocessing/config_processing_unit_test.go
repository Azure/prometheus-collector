package configprocessing

import (
	"testing"
	"time"

	"prometheus-collector/otelcollector/test/utils"

	"github.com/prometheus/prometheus/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMonitorAccessAssertions(t *testing.T) {
	t.Setenv("SCENARIO_RUN_ID", "123e4567-e89b-12d3-a456-426614174000")
	if got := utils.MonitorObject("servicemonitors", "default", nil, scenarioRunLabels()).GetLabels()["ama-metrics.azure.com/scenario-run"]; got != "123e4567-e89b-12d3-a456-426614174000" {
		t.Fatalf("generated monitor has unexpected scenario run label: %q", got)
	}
	allowed, denied := []string{"serviceMonitor/default/allowed"}, []string{"podMonitor/default/denied"}
	if err := validateMonitorJobs([]string{allowed[0] + "/0"}, allowed, denied); err != nil {
		t.Fatal(err)
	}

	for _, jobs := range [][]string{nil, {allowed[0] + "/0", denied[0] + "/0"}, {allowed[0] + "-different/0"}} {
		if err := validateMonitorJobs(jobs, allowed, denied); err == nil {
			t.Fatalf("missing control or admitted forbidden monitor must fail: %v", jobs)
		}
	}
}

func TestSettingsTransitionGuards(t *testing.T) {
	now := time.Now()
	pod := corev1.Pod{Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{
		Name: "prometheus-collector", Ready: true,
		State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(now.Add(time.Second))}},
	}}}}
	if err := utils.ValidateAnnotationReload(pod, "\x1b[0;36mAZMON_AGENT_CFG_FILE_VERSION\x1b[0m=regex-empt\n", "regex-empt", now); err != nil {
		t.Fatal(err)
	}
	if err := utils.ValidateAnnotationReload(pod, "AZMON_AGENT_CFG_FILE_VERSION=regex-defa\n", "regex-empt", now); err == nil {
		t.Fatal("the previous equivalent settings must not prove the new phase reloaded")
	}
	if err := utils.ValidateAnnotationReload(pod, "AZMON_AGENT_CFG_FILE_VERSION=regex-empt\n", "regex-empt", now.Add(2*time.Second)); err == nil {
		t.Fatal("a collector started before the settings change must not prove reload")
	}
	pod.Status.ContainerStatuses[0].Ready = false
	if err := utils.ValidateAnnotationReload(pod, "AZMON_AGENT_CFG_FILE_VERSION=regex-empt\n", "regex-empt", now); err == nil {
		t.Fatal("an unready collector must not prove reload")
	}
	valid := utils.TransitionTarget{ScrapePool: "expected/0", Health: "down", LastError: "HTTP401", LastScrape: now}
	if err := utils.ValidateTransitionTargets([]utils.TransitionTarget{valid}, []string{"expected/0"}, "down", now); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []utils.TransitionTarget{
		{ScrapePool: "other/0", Health: "down", LastError: "HTTP401", LastScrape: now},
		{ScrapePool: "expected/0", Health: "up", LastError: "HTTP401", LastScrape: now},
		{ScrapePool: "expected/0", Health: "down", LastError: "connection refused", LastScrape: now},
		{ScrapePool: "expected/0", Health: "down", LastError: "HTTP401", LastScrape: now.Add(-time.Second)},
	} {
		if err := utils.ValidateTransitionTargets([]utils.TransitionTarget{bad}, []string{"expected/0"}, "down", now); err == nil {
			t.Fatalf("wrong identity/health/auth/freshness must fail: %+v", bad)
		}
	}
	if err := utils.ValidateTransitionTargets(nil, []string{"expected/0"}, "up", now); err == nil {
		t.Fatal("empty targets must fail")
	}
	watchJobsConfig, err := utils.ParseScrapeJobs[*config.ScrapeConfig](`
scrape_configs:
  - job_name: serviceMonitor/test/secret/0
    basic_auth:
      username: old
      password: synthetic
`)
	if err != nil {
		t.Fatal(err)
	}
	watchJobs := []string{"serviceMonitor/test/secret/0"}
	if err := validateMonitorTransitionState(watchJobsConfig, watchJobs, "old", false); err != nil {
		t.Fatal(err)
	}
	if err := validateMonitorTransitionState(watchJobsConfig, watchJobs, "new", false); err == nil {
		t.Fatal("stale or unexpected secret update must fail")
	}
	if err := validateMonitorTransitionState(watchJobsConfig, []string{"podMonitor/test/missing/0"}, "old", false); err == nil {
		t.Fatal("missing secret-backed job must fail")
	}
	watchJobsConfig[0].HTTPClientConfig.BasicAuth.Username = "new"
	if err := validateMonitorTransitionState(watchJobsConfig, watchJobs, "new", false); err != nil {
		t.Fatal(err)
	}
	if err := validateMonitorTransitionState(watchJobsConfig, watchJobs, "old", false); err == nil {
		t.Fatal("unexpected secret update must fail")
	}
	for _, test := range []struct{ cluster, mac, want string }{
		{"/subscriptions/s/resourcegroups/g/providers/p/clusters/real", "true", "real"},
		{"Arc-Cluster.Unmodified", "false", "Arc-Cluster.Unmodified"},
		{"raw/Arc-Cluster.Unmodified", "false", "raw/Arc-Cluster.Unmodified"},
	} {
		got, err := utils.FallbackCluster(test.cluster, test.mac)
		if err != nil || got != test.want {
			t.Fatalf("independent cluster identity: got=%q err=%v want=%q", got, err, test.want)
		}
	}
	if _, err := utils.FallbackCluster("", "true"); err == nil {
		t.Fatal("missing independent identity must fail")
	}
	for _, empty := range []string{"", " \n\t", "# no config\n", "---\n", "null\n"} {
		if _, err := utils.ParseScrapeJobs[*config.ScrapeConfig](empty); err == nil {
			t.Fatalf("config %q must not prove annotation job removal", empty)
		}
	}
	emptyJobs, err := utils.ParseScrapeJobs[*config.ScrapeConfig]("scrape_configs: []")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateAnnotationConfig(emptyJobs, ""); err != nil {
		t.Fatal(err)
	}
	if err := validateAnnotationConfig(nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := validateAnnotationConfig(nil, "^ama-settings-included$"); err == nil {
		t.Fatal("missing admitted annotation job must fail")
	}
	if err := validateAnnotationConfig([]*config.ScrapeConfig{{JobName: "kubernetes-pods"}}, ""); err == nil {
		t.Fatal("retained annotation job after removal must fail")
	}
	cfg, err := utils.ParseScrapeJobs[*config.ScrapeConfig](`
scrape_configs:
  - job_name: kubernetes-pods
    relabel_configs:
      - source_labels: [__meta_kubernetes_namespace]
        action: keep
        regex: ^ama-settings-included$
`)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateAnnotationConfig(cfg, "^ama-settings-included$"); err != nil {
		t.Fatal(err)
	}
	if err := validateAnnotationConfig(cfg, "^ama-settings-excluded$"); err == nil {
		t.Fatal("stale annotation namespace selector must fail")
	}
	if err := validateAnnotationConfig(cfg, ""); err == nil {
		t.Fatal("an empty namespace selector does not remove the annotation job")
	}
}
