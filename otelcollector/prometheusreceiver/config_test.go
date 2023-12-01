// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	promConfig "github.com/prometheus/common/config"
	promModel "github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	r0 := cfg.(*Config)
	assert.Equal(t, r0, factory.CreateDefaultConfig())

	sub, err = cm.Sub(component.NewIDWithName(metadata.Type, "customname").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	r1 := cfg.(*Config)
	assert.Equal(t, r1.PrometheusConfig.ScrapeConfigs[0].JobName, "demo")
	assert.Equal(t, time.Duration(r1.PrometheusConfig.ScrapeConfigs[0].ScrapeInterval), 5*time.Second)
	assert.Equal(t, r1.UseStartTimeMetric, true)
	assert.Equal(t, r1.TrimMetricSuffixes, true)
	assert.Equal(t, r1.EnableProtobufNegotiation, true)
	assert.Equal(t, r1.StartTimeMetricRegex, "^(.+_)*process_start_time_seconds$")
	assert.True(t, r1.ReportExtraScrapeMetrics)

	assert.Equal(t, "http://my-targetallocator-service", r1.TargetAllocator.Endpoint)
	assert.Equal(t, 30*time.Second, r1.TargetAllocator.Interval)
	assert.Equal(t, "collector-1", r1.TargetAllocator.CollectorID)
	assert.Equal(t, promModel.Duration(60*time.Second), r1.TargetAllocator.HTTPSDConfig.RefreshInterval)
	assert.Equal(t, "prometheus", r1.TargetAllocator.HTTPSDConfig.HTTPClientConfig.BasicAuth.Username)
	assert.Equal(t, promConfig.Secret("changeme"), r1.TargetAllocator.HTTPSDConfig.HTTPClientConfig.BasicAuth.Password)
}

func TestLoadTargetAllocatorConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config_target_allocator.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	r0 := cfg.(*Config)
	assert.NotNil(t, r0.PrometheusConfig)
	assert.Equal(t, "http://localhost:8080", r0.TargetAllocator.Endpoint)
	assert.Equal(t, 30*time.Second, r0.TargetAllocator.Interval)
	assert.Equal(t, "collector-1", r0.TargetAllocator.CollectorID)

	sub, err = cm.Sub(component.NewIDWithName(metadata.Type, "withScrape").String())
	require.NoError(t, err)
	cfg = factory.CreateDefaultConfig()
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	r1 := cfg.(*Config)
	assert.NotNil(t, r0.PrometheusConfig)
	assert.Equal(t, "http://localhost:8080", r0.TargetAllocator.Endpoint)
	assert.Equal(t, 30*time.Second, r0.TargetAllocator.Interval)
	assert.Equal(t, "collector-1", r0.TargetAllocator.CollectorID)

	assert.Equal(t, 1, len(r1.PrometheusConfig.ScrapeConfigs))
	assert.Equal(t, "demo", r1.PrometheusConfig.ScrapeConfigs[0].JobName)
	assert.Equal(t, promModel.Duration(5*time.Second), r1.PrometheusConfig.ScrapeConfigs[0].ScrapeInterval)

	sub, err = cm.Sub(component.NewIDWithName(metadata.Type, "withOnlyScrape").String())
	require.NoError(t, err)
	cfg = factory.CreateDefaultConfig()
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	r2 := cfg.(*Config)
	assert.Equal(t, 1, len(r2.PrometheusConfig.ScrapeConfigs))
	assert.Equal(t, "demo", r2.PrometheusConfig.ScrapeConfigs[0].JobName)
	assert.Equal(t, promModel.Duration(5*time.Second), r2.PrometheusConfig.ScrapeConfigs[0].ScrapeInterval)
}

func TestLoadConfigFailsOnUnknownSection(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "invalid-config-section.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.Error(t, component.UnmarshalConfig(sub, cfg))
}

// As one of the config parameters is consuming prometheus
// configuration as a subkey, ensure that invalid configuration
// within the subkey will also raise an error.
func TestLoadConfigFailsOnUnknownPrometheusSection(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "invalid-config-prometheus-section.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.Error(t, component.UnmarshalConfig(sub, cfg))
}

// Renaming emits a warning
func TestConfigWarningsOnRenameDisallowed(t *testing.T) {
	// Construct the config that should emit a warning
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "warning-config-prometheus-relabel.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))
	// Use a fake logger
	creationSet := receivertest.NewNopCreateSettings()
	observedZapCore, observedLogs := observer.New(zap.WarnLevel)
	creationSet.Logger = zap.New(observedZapCore)
	_, err = createMetricsReceiver(context.Background(), creationSet, cfg, nil)
	require.NoError(t, err)
	// We should have received a warning
	assert.Equal(t, 1, observedLogs.Len())
}

func TestRejectUnsupportedPrometheusFeatures(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "invalid-config-prometheus-unsupported-features.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	err = component.ValidateConfig(cfg)
	require.NotNil(t, err, "Expected a non-nil error")

	wantErrMsg := `unsupported features:
        alert_config.alertmanagers
        alert_config.relabel_configs
        remote_read
        remote_write
        rule_files`

	gotErrMsg := strings.ReplaceAll(err.Error(), "\t", strings.Repeat(" ", 8))
	require.Equal(t, wantErrMsg, gotErrMsg)

}

func TestNonExistentAuthCredentialsFile(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "invalid-config-prometheus-non-existent-auth-credentials-file.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	err = component.ValidateConfig(cfg)
	require.NotNil(t, err, "Expected a non-nil error")

	wantErrMsg := `error checking authorization credentials file "/nonexistentauthcredentialsfile"`

	gotErrMsg := err.Error()
	require.True(t, strings.HasPrefix(gotErrMsg, wantErrMsg))
}

func TestTLSConfigNonExistentCertFile(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "invalid-config-prometheus-non-existent-cert-file.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	err = component.ValidateConfig(cfg)
	require.NotNil(t, err, "Expected a non-nil error")

	wantErrMsg := `error checking client cert file "/nonexistentcertfile"`

	gotErrMsg := err.Error()
	require.True(t, strings.HasPrefix(gotErrMsg, wantErrMsg))
}

func TestTLSConfigNonExistentKeyFile(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "invalid-config-prometheus-non-existent-key-file.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	err = component.ValidateConfig(cfg)
	require.NotNil(t, err, "Expected a non-nil error")

	wantErrMsg := `error checking client key file "/nonexistentkeyfile"`

	gotErrMsg := err.Error()
	require.True(t, strings.HasPrefix(gotErrMsg, wantErrMsg))
}

func TestTLSConfigCertFileWithoutKeyFile(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "invalid-config-prometheus-cert-file-without-key-file.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)

	err = component.UnmarshalConfig(sub, cfg)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "exactly one of key or key_file must be configured when a client certificate is configured")
	}
}

func TestTLSConfigKeyFileWithoutCertFile(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "invalid-config-prometheus-key-file-without-cert-file.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	err = component.UnmarshalConfig(sub, cfg)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "exactly one of cert or cert_file must be configured when a client key is configured")
	}
}

func TestKubernetesSDConfigWithoutKeyFile(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "invalid-config-prometheus-kubernetes-sd-config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)

	err = component.UnmarshalConfig(sub, cfg)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "exactly one of key or key_file must be configured when a client certificate is configured")
	}
}

func TestFileSDConfigJsonNilTargetGroup(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "invalid-config-prometheus-file-sd-config-json.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	err = component.ValidateConfig(cfg)
	require.NoError(t, err)
}

func TestFileSDConfigYamlNilTargetGroup(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "invalid-config-prometheus-file-sd-config-yaml.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	err = component.ValidateConfig(cfg)
	require.NoError(t, err)
}

func TestFileSDConfigWithoutSDFile(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "non-existent-prometheus-sd-file-config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	err = component.ValidateConfig(cfg)
	require.NoError(t, err)
}
