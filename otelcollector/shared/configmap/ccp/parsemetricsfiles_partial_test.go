package ccpconfigmapsettings

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/prometheus-collector/shared"
	"github.com/stretchr/testify/require"
)

// TestParseControlplaneScrapeSettings_PartialConfigmap is the control-plane (CCP)
// regression test for the v2 partial-configmap bug.
//
// When a v2 configmap enables kube-scheduler under controlplane-metrics but omits the
// prometheus-collector-settings section, the controlplane-kube-scheduler target must
// still be enabled. Previously ParseMetricsFiles failed all-or-nothing on the missing
// prometheus-collector-settings file, returning a nil section map; the scrape-settings
// parser then fell back to its defaults (kube-scheduler = false) and the
// controlplane-kube-scheduler scrape job was never created.
func TestParseControlplaneScrapeSettings_PartialConfigmap(t *testing.T) {
	dir := t.TempDir()

	controlplaneMetrics := `default-targets-scrape-enabled: |-
  apiserver = true
  cluster-autoscaler = false
  node-auto-provisioning = false
  kube-scheduler = true
  kube-controller-manager = true
  etcd = true
  istio = false
default-targets-metrics-keep-list: |-
  kube-scheduler = ""
minimal-ingestion-profile: |-
  enabled = false`

	cpPath := filepath.Join(dir, "controlplane-metrics")
	require.NoError(t, os.WriteFile(cpPath, []byte(controlplaneMetrics), 0o644))

	// prometheus-collector-settings intentionally absent (partial configmap).
	pcsPathMissing := filepath.Join(dir, "prometheus-collector-settings")

	sections, err := shared.ParseMetricsFiles([]string{cpPath, pcsPathMissing})
	require.NoError(t, err, "missing optional file must not fail the whole parse")
	require.Contains(t, sections, "default-targets-scrape-enabled",
		"controlplane scrape-enabled section was dropped")

	loader := &FilesystemConfigLoader{}
	cfg, err := loader.ParseConfigMapForDefaultScrapeSettings(sections, "v2")
	require.NoError(t, err)

	require.Equal(t, "true", cfg["controlplane-kube-scheduler"],
		"kube-scheduler set to true in configmap but not enabled after parse")
	require.Equal(t, "true", cfg["controlplane-kube-controller-manager"])
	require.Equal(t, "true", cfg["controlplane-etcd"])
	require.Equal(t, "true", cfg["controlplane-apiserver"])
}
