package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// test prometheusCcpConfigMerger method
func TestMergeYAML_WithMultipleJobsEnabled_ThenMergedConfigIsComplete(t *testing.T) {
	// Arrange
	mergedDefaultConfigs := make(map[interface{}]interface{})
	config1 := map[interface{}]interface{}{
		"scrape_configs": map[interface{}]interface{}{
			"job_name":         "controlplane-cluster-autoscaler",
			"scrape_interval":  "30s",
			"follow_redirects": "false",
			"tls_config": map[interface{}]interface{}{
				"ca_file":              "/etc/kubernetes/secrets/ca.pem",
				"cert_file":            "/etc/kubernetes/secrets/client.pem",
				"key_file":             "/etc/kubernetes/secrets/client-key.pem",
				"insecure_skip_verify": "true",
			},
			"relabel_configs": [2]map[interface{}]interface{}{
				{
					"source_labels": [2]string{"__meta_kubernetes_pod_label_app", "__meta_kubernetes_pod_container_name"},
					"action":        "keep",
					"regex":         "cluster-autoscaler;cluster-autoscaler",
				},
				{
					"source_labels": [1]string{"__meta_kubernetes_pod_annotation_aks_prometheus_io_path"},
					"action":        "replace",
					"target_label":  "__metrics_path__",
					"regex":         "(.+)",
				},
			},
			"metric_relabel_configs": [1]map[interface{}]interface{}{
				{
					"source_labels": [1]string{"__name__"},
					"action":        "drop",
					"regex":         "(go_.*|process_(cpu|max|resident|virtual|open)_.*)",
				},
			},
		},
	}
	config2 := map[interface{}]interface{}{
		"scrape_configs": map[interface{}]interface{}{
			"job_name":         "controlplane-apiserver",
			"scrape_interval":  "30s",
			"follow_redirects": "false",
			"tls_config": map[interface{}]interface{}{
				"ca_file":              "/etc/kubernetes/secrets/etcd-client-ca.crt",
				"cert_file":            "/etc/kubernetes/secrets/etcd-client.crt",
				"key_file":             "/etc/kubernetes/secrets/etcd-client.key",
				"insecure_skip_verify": "true",
			},
			"relabel_configs": [3]map[interface{}]interface{}{
				{
					"source_labels": [2]string{"__meta_kubernetes_pod_label_app", "__meta_kubernetes_pod_container_port_number"},
					"action":        "keep",
					"regex":         "etcd;2379",
				},
				{
					"source_labels": [1]string{"__meta_kubernetes_pod_name"},
					"action":        "replace",
					"target_label":  "instance",
					"regex":         "(.+)",
				},
				{
					"source_labels": [1]string{"__meta_kubernetes_pod_name"},
					"action":        "drop",
					"regex":         "(etcd2-.*)",
				},
			},
			"metric_relabel_configs": [1]map[interface{}]interface{}{
				{
					"source_labels": [1]string{"__name__"},
					"action":        "drop",
					"regex":         "(go_.*|process_(cpu|max|resident|virtual|open)_.*)",
				},
			},
		},
	}

	// Act
	mergedDefaultConfigs = deepMerge(mergedDefaultConfigs, config1)
	mergedDefaultConfigs = deepMerge(mergedDefaultConfigs, config2)

	// Assert
	require.Equal(t, 1, len(mergedDefaultConfigs), "Exactly one root element")
	require.NotNil(t, mergedDefaultConfigs["scrape_configs"], "scrape_configs should not be nil")
	mergedConfigStr := fmt.Sprintf("%v", mergedDefaultConfigs["scrape_configs"])
	require.Equal(t, 2, strings.Count(mergedConfigStr, "job_name"), "Exactly two job_name")

	// TODO: add more assertions to check that the merged config is complete
}
