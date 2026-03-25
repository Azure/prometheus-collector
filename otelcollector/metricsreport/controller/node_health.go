package controller

import (
	"context"
	"fmt"
	"strconv"

	healthv1alpha1 "prometheus-collector/metricsreport/api/v1alpha1"
)

// evaluateNodeHealth checks if a specific node is healthy by querying:
//   - kube_node_status_condition for Ready and NetworkUnavailable
//   - kube_pod_status_phase for pod health on the node (>=50% unhealthy → Unhealthy)
//   - kube_poddisruptionbudget_status_pod_disruptions_allowed for restrictive PDBs (any → Unhealthy)
func (r *HealthSignalReconciler) evaluateNodeHealth(ctx context.Context, nodeName string) (string, string, string) {
	// 1. Check Ready condition
	readyQuery := fmt.Sprintf(`kube_node_status_condition{node="%s",condition="Ready",status="true"}`, nodeName)
	readyResult, err := r.queryPrometheus(ctx, readyQuery)
	if err != nil {
		return healthv1alpha1.ConditionOngoing, "PrometheusQueryFailed", fmt.Sprintf("Failed to query Prometheus for Ready: %v", err)
	}

	if len(readyResult) == 0 {
		return healthv1alpha1.ConditionOngoing, "NoData", fmt.Sprintf("No kube_node_status_condition data for node %s", nodeName)
	}

	// 2. Check NetworkUnavailable condition
	netQuery := fmt.Sprintf(`kube_node_status_condition{node="%s",condition="NetworkUnavailable",status="true"}`, nodeName)
	netResult, err := r.queryPrometheus(ctx, netQuery)
	if err != nil {
		return healthv1alpha1.ConditionOngoing, "PrometheusQueryFailed", fmt.Sprintf("Failed to query Prometheus for NetworkUnavailable: %v", err)
	}

	for _, r := range netResult {
		if r.Value == "1" {
			return healthv1alpha1.ConditionUnhealthy, "NetworkUnavailable", fmt.Sprintf("Node %s has NetworkUnavailable=True", nodeName)
		}
	}

	nodeReady := false
	for _, r := range readyResult {
		if r.Value == "1" {
			nodeReady = true
			break
		}
	}
	if !nodeReady {
		return healthv1alpha1.ConditionUnhealthy, "NodeNotReady", fmt.Sprintf("Node %s is not Ready", nodeName)
	}

	// 3. Check for restrictive PDBs — even one means node is not safe to drain
	pdbStatus, pdbReason, pdbMsg := r.evaluatePDBsOnNode(ctx, nodeName)
	if pdbStatus == healthv1alpha1.ConditionUnhealthy {
		return pdbStatus, pdbReason, pdbMsg
	}

	// 4. Check pod health — >=50% unhealthy means node is unhealthy
	podStatus, podReason, podMsg := r.evaluatePodsOnNode(ctx, nodeName)
	if podStatus == healthv1alpha1.ConditionUnhealthy {
		return podStatus, podReason, podMsg
	}

	msg := fmt.Sprintf("Node %s is Ready and network is available", nodeName)
	if podMsg != "" {
		msg += "; " + podMsg
	}
	return healthv1alpha1.ConditionHealthy, "NodeHealthy", msg
}

// evaluatePodsOnNode checks pod health on a specific node.
// If >=50% of pods are in Failed/Unknown state, returns Unhealthy.
func (r *HealthSignalReconciler) evaluatePodsOnNode(ctx context.Context, nodeName string) (string, string, string) {
	// Total pods scheduled on the node
	totalQuery := fmt.Sprintf(`count(kube_pod_info{node="%s"})`, nodeName)
	totalResult, err := r.queryPrometheus(ctx, totalQuery)
	if err != nil {
		return healthv1alpha1.ConditionOngoing, "PrometheusQueryFailed", fmt.Sprintf("pod query error: %v", err)
	}

	// Pods in Failed or Unknown phase on the node
	unhealthyQuery := fmt.Sprintf(
		`count(kube_pod_info{node="%s"} * on(pod,namespace) group_left() (kube_pod_status_phase{phase=~"Failed|Unknown"} == 1))`,
		nodeName,
	)
	unhealthyResult, err := r.queryPrometheus(ctx, unhealthyQuery)
	if err != nil {
		return healthv1alpha1.ConditionOngoing, "PrometheusQueryFailed", fmt.Sprintf("pod query error: %v", err)
	}

	total := parseIntOrZero(totalResult)
	unhealthy := parseIntOrZero(unhealthyResult)

	if total == 0 {
		return healthv1alpha1.ConditionHealthy, "NoPods", fmt.Sprintf("No pods scheduled on node %s", nodeName)
	}

	msg := fmt.Sprintf("pods total: %d, pods unhealthy (Failed/Unknown): %d", total, unhealthy)

	// >=50% unhealthy → node is unhealthy
	if unhealthy*2 >= total {
		return healthv1alpha1.ConditionUnhealthy, "PodsUnhealthy",
			fmt.Sprintf("Node %s: %d/%d pods (>=50%%) are in Failed/Unknown state", nodeName, unhealthy, total)
	}

	return healthv1alpha1.ConditionHealthy, "PodsHealthy", msg
}

// evaluatePDBsOnNode checks for PodDisruptionBudgets that have zero disruptions allowed
// and affect pods running on this node. Even a single restrictive PDB means the node
// cannot be safely drained during an upgrade.
func (r *HealthSignalReconciler) evaluatePDBsOnNode(ctx context.Context, nodeName string) (string, string, string) {
	pdbQuery := fmt.Sprintf(
		`count(kube_poddisruptionbudget_status_pod_disruptions_allowed == 0 and on(namespace) kube_pod_info{node="%s"})`,
		nodeName,
	)
	pdbResult, err := r.queryPrometheus(ctx, pdbQuery)
	if err != nil {
		return healthv1alpha1.ConditionOngoing, "PrometheusQueryFailed", fmt.Sprintf("PDB query error: %v", err)
	}

	restrictive := parseIntOrZero(pdbResult)
	if restrictive > 0 {
		return healthv1alpha1.ConditionUnhealthy, "RestrictivePDB",
			fmt.Sprintf("Node %s: %d PDB(s) with zero disruptions allowed — drain may be blocked", nodeName, restrictive)
	}

	return healthv1alpha1.ConditionHealthy, "NoPDBRestrictions", ""
}

// parseIntOrZero extracts the integer value from a Prometheus query result.
// Returns 0 if the result is empty or unparseable.
func parseIntOrZero(results []promResult) int {
	if len(results) == 0 {
		return 0
	}
	v, err := strconv.Atoi(results[0].Value)
	if err != nil {
		return 0
	}
	return v
}
