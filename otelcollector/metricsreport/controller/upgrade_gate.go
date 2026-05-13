package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// upgradeGateConfigMapName is the name of the ConfigMap that holds
	// customer-defined rules for upgrade readiness.
	upgradeGateConfigMapName = "ama-metrics-upgrade-gate"
	// upgradeGateConfigMapNamespace is the namespace where the gate ConfigMap lives.
	upgradeGateConfigMapNamespace = "kube-system"
	// upgradeGateRulesKey is the ConfigMap data key containing the JSON rules array.
	upgradeGateRulesKey = "rules"
	// upgradeGateRetentionPercentKey is the ConfigMap data key for the cache
	// retention percentage (1-100). The actual retention window is computed as:
	//   window = maxRetention × (percent/100) × (referenceNodes / actualNodes)
	// This keeps memory usage roughly constant regardless of cluster size.
	upgradeGateRetentionPercentKey = "retentionPercent"
	// maxRetention is the longest possible retention window (at 100% on a small cluster).
	maxRetention = 1 * time.Hour
	// referenceNodeCount is the baseline cluster size for retention scaling.
	// A 10-node cluster at 100% gets the full maxRetention.
	referenceNodeCount = 10
	// defaultRetentionPercent is used when the customer doesn't set a value.
	defaultRetentionPercent = 100
	// minRetention prevents the window from shrinking too small.
	minRetention = 1 * time.Minute
)

// UpgradeRule is a single customer-defined rule read from the ConfigMap.
// Each rule specifies a Prometheus query whose scalar result is compared
// against a threshold using the given operator.
//
// Rules can be scoped to a specific level (Node, NodePool, Cluster) so they
// only run for HealthCheckRequests at that scope. If scope is empty or "*",
// the rule runs for all scopes.
//
// Queries support template variables that are substituted at evaluation time:
//   - {{.NodeName}}  — the target node name (Node scope)
//   - {{.PoolName}}  — the target pool name (NodePool scope)
//   - {{.TargetName}} — the raw targetName from the HealthCheckRequest
//
// Example ConfigMap value for the "rules" key:
//
//	[
//	  {
//	    "name": "pool-error-rate",
//	    "scope": "NodePool",
//	    "query": "sum(rate(http_requests_total{code=~\"5..\",node_pool=\"{{.PoolName}}\"}[5m]))",
//	    "operator": "<",
//	    "threshold": 0.05
//	  },
//	  {
//	    "name": "node-cpu-ok",
//	    "scope": "Node",
//	    "query": "1 - avg(rate(node_cpu_seconds_total{mode=\"idle\",node=\"{{.NodeName}}\"}[5m]))",
//	    "operator": "<",
//	    "threshold": 0.9
//	  },
//	  {
//	    "name": "global-error-budget",
//	    "scope": "*",
//	    "query": "sum(rate(http_requests_total{code=~\"5..\"}[5m]))",
//	    "operator": "<",
//	    "threshold": 0.1
//	  }
//	]
type UpgradeRule struct {
	// Name is a short human-readable identifier for the rule.
	Name string `json:"name"`
	// Scope restricts when this rule is evaluated: "Node", "NodePool", "Cluster",
	// or "*" / "" for all scopes.
	Scope string `json:"scope,omitempty"`
	// Query is a PromQL instant query that must return a single scalar value.
	// Supports {{.NodeName}}, {{.PoolName}}, {{.TargetName}} template variables.
	Query string `json:"query"`
	// Operator is the comparison operator: <, >, <=, >=, ==, !=
	Operator string `json:"operator"`
	// Threshold is the value the query result is compared against.
	Threshold float64 `json:"threshold"`
}

// RenderQuery substitutes template variables in the rule's query with actual values.
func (rule UpgradeRule) RenderQuery(scope, targetName string) string {
	q := rule.Query
	q = strings.ReplaceAll(q, "{{.TargetName}}", targetName)

	switch scope {
	case "Node":
		q = strings.ReplaceAll(q, "{{.NodeName}}", targetName)
		q = strings.ReplaceAll(q, "{{.PoolName}}", "")
	case "NodePool":
		q = strings.ReplaceAll(q, "{{.PoolName}}", targetName)
		q = strings.ReplaceAll(q, "{{.NodeName}}", "")
	default:
		q = strings.ReplaceAll(q, "{{.NodeName}}", "")
		q = strings.ReplaceAll(q, "{{.PoolName}}", "")
	}
	return q
}

// MatchesScope returns true if this rule should run for the given HealthCheckRequest scope.
func (rule UpgradeRule) MatchesScope(scope string) bool {
	if rule.Scope == "" || rule.Scope == "*" {
		return true
	}
	return rule.Scope == scope
}

// UpgradeGate reads customer-defined upgrade rules from a ConfigMap.
type UpgradeGate struct {
	client client.Client
}

// NewUpgradeGate creates an UpgradeGate backed by the given Kubernetes client.
func NewUpgradeGate(c client.Client) *UpgradeGate {
	return &UpgradeGate{client: c}
}

// LoadRules reads the ConfigMap and returns the parsed rules.
// If the ConfigMap is missing or has no rules key, it returns an empty slice (no rules = allow).
func (g *UpgradeGate) LoadRules(ctx context.Context) ([]UpgradeRule, error) {
	logger := ctrllog.FromContext(ctx)

	cm := &corev1.ConfigMap{}
	key := types.NamespacedName{
		Namespace: upgradeGateConfigMapNamespace,
		Name:      upgradeGateConfigMapName,
	}

	if err := g.client.Get(ctx, key, cm); err != nil {
		logger.V(1).Info("Upgrade gate ConfigMap not found, no custom rules to evaluate", "error", err)
		return nil, nil
	}

	rulesJSON, ok := cm.Data[upgradeGateRulesKey]
	if !ok || rulesJSON == "" {
		logger.V(1).Info("Upgrade gate ConfigMap has no rules key, no custom rules to evaluate")
		return nil, nil
	}

	var rules []UpgradeRule
	if err := json.Unmarshal([]byte(rulesJSON), &rules); err != nil {
		return nil, fmt.Errorf("parsing upgrade rules from ConfigMap %s/%s: %w",
			upgradeGateConfigMapNamespace, upgradeGateConfigMapName, err)
	}

	logger.Info("Loaded upgrade gate rules", "count", len(rules))
	return rules, nil
}

// LoadRetentionWindow computes the cache retention window based on:
//   - retentionPercent from the ConfigMap (1–100, default 100)
//   - nodeCount: the current number of nodes in the cluster
//
// The formula keeps total cache memory roughly constant regardless of cluster size:
//
//	memoryBudget  = baseBudgetMB × (retentionPercent / 100)
//	snapshotsPerCycle = nodeCount × metricTypesPerNode
//	maxSnapshots  = memoryBudget / estimatedBytesPerSnapshot
//	windowCycles  = maxSnapshots / snapshotsPerCycle
//	retention     = windowCycles × reconcileInterval
//
// Examples at retentionPercent=100 (300MB budget):
//
//	10 nodes   → 1h (clamped)
//	100 nodes  → 1h (clamped)
//	500 nodes  → ~1h (clamped)
//	1000 nodes → ~30min
//	5000 nodes → ~6min
func (g *UpgradeGate) LoadRetentionWindow(ctx context.Context, nodeCount int) time.Duration {
	logger := ctrllog.FromContext(ctx)

	pct := defaultRetentionPercent

	cm := &corev1.ConfigMap{}
	key := types.NamespacedName{
		Namespace: upgradeGateConfigMapNamespace,
		Name:      upgradeGateConfigMapName,
	}
	if err := g.client.Get(ctx, key, cm); err == nil {
		if val, ok := cm.Data[upgradeGateRetentionPercentKey]; ok && val != "" {
			if p, err := strconv.Atoi(val); err == nil {
				if p < 1 {
					p = 1
				} else if p > 100 {
					p = 100
				}
				pct = p
			} else {
				logger.Error(err, "Invalid retentionPercent value, using default", "value", val)
			}
		}
	}

	if nodeCount < 1 {
		nodeCount = 1
	}

	const (
		baseBudgetBytes    = 300 * 1024 * 1024 // 300MB
		bytesPerSnapshot   = 500
		metricTypesPerNode = 10
		reconcileSeconds   = 30
	)

	budget := float64(baseBudgetBytes) * float64(pct) / 100.0
	snapshotsPerCycle := float64(nodeCount * metricTypesPerNode)
	maxSnapshots := budget / float64(bytesPerSnapshot)
	windowCycles := maxSnapshots / snapshotsPerCycle
	window := time.Duration(windowCycles*reconcileSeconds) * time.Second

	// Clamp
	if window < minRetention {
		window = minRetention
	}
	if window > maxRetention {
		window = maxRetention
	}

	logger.V(1).Info("Computed retention window", "nodeCount", nodeCount, "retentionPercent", pct, "window", window)
	return window
}

// CompareValue checks whether value <op> threshold is true.
func CompareValue(value float64, op string, threshold float64) bool {
	switch op {
	case "<":
		return value < threshold
	case ">":
		return value > threshold
	case "<=":
		return value <= threshold
	case ">=":
		return value >= threshold
	case "==":
		return value == threshold
	case "!=":
		return value != threshold
	default:
		return false
	}
}
