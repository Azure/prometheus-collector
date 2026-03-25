package controller

import (
	"context"
	"fmt"

	healthv1alpha1 "prometheus-collector/metricsreport/api/v1alpha1"
)

// evaluateNodePoolHealth checks health of all nodes in a specific AKS node pool.
// Nodes are filtered by the label kubernetes.azure.com/agentpool=<poolName> which
// kube-state-metrics exposes via the "node" label joined with kube_node_labels.
func (r *HealthSignalReconciler) evaluateNodePoolHealth(ctx context.Context, poolName string) (string, string, string) {
	// Find all nodes in the pool by joining kube_node_labels with kube_node_status_condition.
	// kube_node_labels exposes label_kubernetes_azure_com_agentpool.

	// Check NetworkUnavailable for nodes in pool
	netQuery := fmt.Sprintf(
		`count(kube_node_status_condition{condition="NetworkUnavailable",status="true"} == 1 and on(node) kube_node_labels{label_kubernetes_azure_com_agentpool="%s"})`,
		poolName,
	)
	netResult, err := r.queryPrometheus(ctx, netQuery)
	if err != nil {
		return healthv1alpha1.ConditionOngoing, "PrometheusQueryFailed", fmt.Sprintf("Failed to query Prometheus for pool NetworkUnavailable: %v", err)
	}
	for _, r := range netResult {
		if r.Value != "0" && r.Value != "" {
			return healthv1alpha1.ConditionUnhealthy, "NetworkUnavailable", fmt.Sprintf("Pool %s: some nodes have NetworkUnavailable (count: %s)", poolName, r.Value)
		}
	}

	// Check NotReady for nodes in pool
	readyQuery := fmt.Sprintf(
		`count(kube_node_status_condition{condition="Ready",status="true"} == 0 and on(node) kube_node_labels{label_kubernetes_azure_com_agentpool="%s"})`,
		poolName,
	)
	readyResult, err := r.queryPrometheus(ctx, readyQuery)
	if err != nil {
		return healthv1alpha1.ConditionOngoing, "PrometheusQueryFailed", fmt.Sprintf("Failed to query Prometheus for pool Ready: %v", err)
	}

	if len(readyResult) == 0 && len(netResult) == 0 {
		return healthv1alpha1.ConditionOngoing, "NoData", fmt.Sprintf("No health data available for pool %s", poolName)
	}

	for _, r := range readyResult {
		if r.Value != "0" && r.Value != "" {
			return healthv1alpha1.ConditionUnhealthy, "NodesNotReady", fmt.Sprintf("Pool %s: some nodes are not Ready (count: %s)", poolName, r.Value)
		}
	}

	return healthv1alpha1.ConditionHealthy, "PoolHealthy", fmt.Sprintf("All nodes in pool %s are Ready and network is available", poolName)
}
