package shared

import (
	"os"
	"path/filepath"
	"testing"
)

// writeFile is a small helper that writes content to dir/name and returns the path.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", p, err)
	}
	return p
}

// TestParseMetricsFiles_MissingOptionalFileIsSkipped is the core regression test for
// partial v2 configmaps. A user may omit an optional section (e.g.
// prometheus-collector-settings); Kubernetes then does not project a file for it.
// ParseMetricsFiles must skip the missing file and still return every section that
// WAS provided, instead of discarding the whole configuration.
func TestParseMetricsFiles_MissingOptionalFileIsSkipped(t *testing.T) {
	dir := t.TempDir()

	clusterMetrics := `default-targets-metrics-keep-list: |-
  kubestate = "kube_pod_labels|kube_persistentvolumeclaim_info|kube_persistentvolume_capacity_bytes"
minimal-ingestion-profile: |-
  enabled = false`
	present := writeFile(t, dir, "cluster-metrics", clusterMetrics)
	missing := filepath.Join(dir, "prometheus-collector-settings") // intentionally not created

	sections, err := ParseMetricsFiles([]string{present, missing})
	if err != nil {
		t.Fatalf("expected no error for a missing optional file, got: %v", err)
	}

	keepList, ok := sections["default-targets-metrics-keep-list"]
	if !ok {
		t.Fatalf("default-targets-metrics-keep-list section was dropped; sections=%v", sections)
	}
	got := keepList["kubestate"]
	want := "kube_pod_labels|kube_persistentvolumeclaim_info|kube_persistentvolume_capacity_bytes"
	if got != want {
		t.Fatalf("kubestate keep-list = %q, want %q", got, want)
	}

	if mip := sections["minimal-ingestion-profile"]["enabled"]; mip != "false" {
		t.Fatalf("minimal-ingestion-profile enabled = %q, want \"false\"", mip)
	}
}

// TestParseMetricsFiles_ControlplanePartialConfigmap mirrors the control-plane (CCP)
// consumer scenario: controlplane-metrics is provided with kube-scheduler enabled while
// prometheus-collector-settings is omitted. The controlplane scrape-enabled section must
// survive so the controlplane-kube-scheduler target can be created downstream.
func TestParseMetricsFiles_ControlplanePartialConfigmap(t *testing.T) {
	dir := t.TempDir()

	controlplaneMetrics := `default-targets-scrape-enabled: |-
  apiserver = true
  kube-scheduler = true
  etcd = true`
	present := writeFile(t, dir, "controlplane-metrics", controlplaneMetrics)
	missing := filepath.Join(dir, "prometheus-collector-settings") // intentionally not created

	sections, err := ParseMetricsFiles([]string{present, missing})
	if err != nil {
		t.Fatalf("expected no error for a missing optional file, got: %v", err)
	}

	scrape, ok := sections["default-targets-scrape-enabled"]
	if !ok {
		t.Fatalf("default-targets-scrape-enabled section was dropped; sections=%v", sections)
	}
	if scrape["kube-scheduler"] != "true" {
		t.Fatalf("kube-scheduler = %q, want \"true\"", scrape["kube-scheduler"])
	}
}

// TestParseMetricsFiles_AllFilesPresent verifies normal parsing of every provided file.
func TestParseMetricsFiles_AllFilesPresent(t *testing.T) {
	dir := t.TempDir()

	cm := writeFile(t, dir, "cluster-metrics", `default-targets-scrape-enabled: |-
  kubestate = true`)
	pcs := writeFile(t, dir, "prometheus-collector-settings", `cluster_alias = "myclus"`)

	sections, err := ParseMetricsFiles([]string{cm, pcs})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v := sections["default-targets-scrape-enabled"]["kubestate"]; v != "true" {
		t.Fatalf("kubestate = %q, want \"true\"", v)
	}
	// Top-level keys (no section header) bucket into prometheus-collector-settings.
	if v := sections["prometheus-collector-settings"]["cluster_alias"]; v != "myclus" {
		t.Fatalf("cluster_alias = %q, want \"myclus\"", v)
	}
}

// TestParseMetricsFiles_AllMissingReturnsEmpty verifies that when no files exist the
// function returns a non-nil, empty map and no error (downstream parsers then default).
func TestParseMetricsFiles_AllMissingReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "cluster-metrics")
	b := filepath.Join(dir, "prometheus-collector-settings")

	sections, err := ParseMetricsFiles([]string{a, b})
	if err != nil {
		t.Fatalf("expected no error when all files are missing, got: %v", err)
	}
	if sections == nil {
		t.Fatalf("expected non-nil map, got nil")
	}
	if len(sections) != 0 {
		t.Fatalf("expected empty map, got: %v", sections)
	}
}
