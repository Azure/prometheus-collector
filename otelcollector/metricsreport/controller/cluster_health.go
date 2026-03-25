package controller

import (
	"context"
	"fmt"

	healthv1alpha1 "prometheus-collector/metricsreport/api/v1alpha1"
)

// evaluateClusterHealth checks overall cluster health via Ready and NetworkUnavailable conditions
// across ALL nodes in the cluster. The cluster is unhealthy if >=40% of nodes are unhealthy.
func (r *HealthSignalReconciler) evaluateClusterHealth(ctx context.Context) (string, string, string) {
	// Get total node count
	totalQuery := `count(kube_node_status_condition{condition="Ready",status="true"})`
	totalResult, err := r.queryPrometheus(ctx, totalQuery)
	if err != nil {
		return healthv1alpha1.ConditionOngoing, "PrometheusQueryFailed", fmt.Sprintf("Failed to query Prometheus for total nodes: %v", err)
	}

	totalNodes := parseIntOrZero(totalResult)
	if totalNodes == 0 {
		return healthv1alpha1.ConditionOngoing, "NoData", "No cluster health data available yet"
	}

	// Count nodes with NetworkUnavailable=true
	netQuery := `count(kube_node_status_condition{condition="NetworkUnavailable",status="true"} == 1)`
	netResult, err := r.queryPrometheus(ctx, netQuery)
	if err != nil {
		return healthv1alpha1.ConditionOngoing, "PrometheusQueryFailed", fmt.Sprintf("Failed to query Prometheus for NetworkUnavailable: %v", err)
	}
	netUnavailable := parseIntOrZero(netResult)

	// Count nodes that are NotReady
	notReadyQuery := `count(kube_node_status_condition{condition="Ready",status="true"} == 0)`
	notReadyResult, err := r.queryPrometheus(ctx, notReadyQuery)
	if err != nil {
		return healthv1alpha1.ConditionOngoing, "PrometheusQueryFailed", fmt.Sprintf("Failed to query Prometheus for Ready: %v", err)
	}
	notReady := parseIntOrZero(notReadyResult)

	// Total unhealthy = max(netUnavailable, notReady) since they can overlap
	unhealthy := notReady
	if netUnavailable > unhealthy {
		unhealthy = netUnavailable
	}

	// >=40% unhealthy nodes → cluster is unhealthy
	if unhealthy*100 >= totalNodes*40 {
		reason := "NodesNotReady"
		detail := fmt.Sprintf("not Ready: %d", notReady)
		if netUnavailable > 0 {
			reason = "NetworkUnavailable"
			detail = fmt.Sprintf("NetworkUnavailable: %d, not Ready: %d", netUnavailable, notReady)
		}
		return healthv1alpha1.ConditionUnhealthy, reason,
			fmt.Sprintf("Cluster unhealthy: %d/%d nodes (>=40%%) are unhealthy (%s)", unhealthy, totalNodes, detail)
	}

	msg := fmt.Sprintf("Cluster healthy: %d/%d nodes are Ready and network is available", totalNodes-unhealthy, totalNodes)
	if unhealthy > 0 {
		msg += fmt.Sprintf(" (%d unhealthy, below 40%% threshold)", unhealthy)
	}
	return healthv1alpha1.ConditionHealthy, "ClusterHealthy", msg
}
