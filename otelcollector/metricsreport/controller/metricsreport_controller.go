package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	healthv1alpha1 "prometheus-collector/metricsreport/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	healthSignalPrefix = "prom-health-"
	conditionTypeReady = "Ready"
	// How long a node must be NotReady before we report unhealthy.
	notReadyThreshold = 2 * time.Minute
)

// HealthSignalReconciler watches HealthCheckRequests and creates/updates HealthSignals.
type HealthSignalReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	// PrometheusAPIURL is the Prometheus HTTP API base URL.
	// Configured via the PROMETHEUS_API_URL environment variable.
	// Defaults to http://localhost:9092 (the local collector's Prometheus receiver).
	PrometheusAPIURL string
	httpClient       *http.Client
	// Cache stores upgrade-health metrics (node readiness, network, pod health,
	// PDB status, customer rules) with a 1-hour sliding window.
	Cache *MetricsCache
	// UpgradeGate reads customer-defined upgrade rules from a ConfigMap.
	UpgradeGate *UpgradeGate
}

func (r *HealthSignalReconciler) getHTTPClient() *http.Client {
	if r.httpClient == nil {
		r.httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return r.httpClient
}

// Reconcile handles a single HealthCheckRequest event.
func (r *HealthSignalReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrllog.FromContext(ctx)

	// 1. Fetch the HealthCheckRequest
	hcr := &healthv1alpha1.HealthCheckRequest{}
	if err := r.Get(ctx, req.NamespacedName, hcr); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil // deleted, nothing to do
		}
		return ctrl.Result{}, err
	}

	logger.Info("Reconciling HealthCheckRequest", "name", hcr.Name, "scope", hcr.Spec.Scope, "target", hcr.Spec.TargetName)

	// 1b. Sync cache retention window — scales based on cluster size and customer percentage.
	if r.UpgradeGate != nil && r.Cache != nil {
		nodeCount := r.getNodeCount(ctx)
		retention := r.UpgradeGate.LoadRetentionWindow(ctx, nodeCount)
		r.Cache.SetWindow(retention)
	}

	// 1c. Evaluate customer-defined upgrade rules from ConfigMap.
	// Each rule is a PromQL query + operator + threshold. Rules are scoped
	// so only rules matching this request's scope (Node/NodePool/Cluster) run,
	// and template variables ({{.NodeName}}, {{.PoolName}}) are substituted.
	if r.UpgradeGate != nil {
		rules, err := r.UpgradeGate.LoadRules(ctx)
		if err != nil {
			logger.Error(err, "Failed to load upgrade gate rules")
			return r.reconcileWithStatus(ctx, hcr, healthv1alpha1.ConditionOngoing, "RuleLoadError", err.Error())
		}
		if len(rules) > 0 {
			allowed, failMsg := r.evaluateUpgradeRules(ctx, rules, string(hcr.Spec.Scope), hcr.Spec.TargetName)
			if !allowed {
				logger.Info("Upgrade blocked by customer rules", "detail", failMsg)
				return r.reconcileWithStatus(ctx, hcr, healthv1alpha1.ConditionUnhealthy, "UpgradeRuleFailed", failMsg)
			}
			logger.Info("All customer upgrade rules passed")
		}
	}

	// 2. Determine signal type from request scope
	signalType := healthv1alpha1.NodeHealth
	targetRef := corev1.ObjectReference{
		Name: hcr.Spec.TargetName,
	}

	switch hcr.Spec.Scope {
	case healthv1alpha1.HealthCheckRequestScopeNode:
		signalType = healthv1alpha1.NodeHealth
		targetRef.APIVersion = "v1"
		targetRef.Kind = "Node"
	case healthv1alpha1.HealthCheckRequestScopeCluster:
		signalType = healthv1alpha1.ClusterHealth
		// No single K8s object represents the cluster; reference the default namespace as a sentinel.
		targetRef.APIVersion = "v1"
		targetRef.Kind = "Namespace"
		targetRef.Name = "default"
	case healthv1alpha1.HealthCheckRequestScopeNodePool:
		// A node pool spans multiple nodes. Use NodeHealth type and reference the pool
		// by name. Nodes belonging to the pool share the label "kubernetes.azure.com/agentpool=<poolName>".
		signalType = healthv1alpha1.NodeHealth
		targetRef.APIVersion = "v1"
		targetRef.Kind = "Node" // represents the set of nodes in the pool
	}

	// 3. Evaluate health from Prometheus metrics
	conditionStatus, reason, message := r.evaluateHealth(ctx, hcr.Spec.Scope, hcr.Spec.TargetName)

	// 4. Get or create the HealthSignal
	signalName := healthSignalPrefix + hcr.Name
	signal := &healthv1alpha1.HealthSignal{}
	signal.Name = signalName

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, signal, func() error {
		// Set spec
		signal.Spec = healthv1alpha1.HealthSignalSpec{
			Type:      signalType,
			TargetRef: targetRef,
		}

		// Set ownerReference to the HealthCheckRequest (required by AKS Health Signal spec)
		signal.OwnerReferences = []metav1.OwnerReference{
			{
				APIVersion: healthv1alpha1.GroupVersion.String(),
				Kind:       "HealthCheckRequest",
				Name:       hcr.Name,
				UID:        hcr.UID,
			},
		}

		// Copy correlation annotations from the request
		if signal.Annotations == nil {
			signal.Annotations = make(map[string]string)
		}
		if v, ok := hcr.Annotations["kubernetes.azure.com/upgradeCorrelationID"]; ok {
			signal.Annotations["kubernetes.azure.com/upgradeCorrelationID"] = v
		}
		signal.Annotations["health.aks.io/request-name"] = hcr.Name

		return nil
	})
	if err != nil {
		logger.Error(err, "Failed to create or update HealthSignal", "name", signalName)
		return ctrl.Result{}, err
	}
	logger.Info("HealthSignal reconciled", "name", signalName, "operation", op)

	// 5. Update status conditions
	now := metav1.Now()
	condition := metav1.Condition{
		Type:               conditionTypeReady,
		Status:             metav1.ConditionStatus(conditionStatus),
		ObservedGeneration: signal.Generation,
		LastTransitionTime: now,
		Reason:             reason,
		Message:            message,
	}
	meta.SetStatusCondition(&signal.Status.Conditions, condition)

	if err := r.Status().Update(ctx, signal); err != nil {
		logger.Error(err, "Failed to update HealthSignal status", "name", signalName)
		return ctrl.Result{}, err
	}

	// Re-evaluate periodically
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// evaluateHealth queries Prometheus to determine the health of a target.
// Returns (conditionStatus, reason, message).
func (r *HealthSignalReconciler) evaluateHealth(ctx context.Context, scope healthv1alpha1.HealthCheckRequestScope, targetName string) (string, string, string) {
	switch scope {
	case healthv1alpha1.HealthCheckRequestScopeNode:
		return r.evaluateNodeHealth(ctx, targetName)
	case healthv1alpha1.HealthCheckRequestScopeCluster:
		return r.evaluateClusterHealth(ctx)
	case healthv1alpha1.HealthCheckRequestScopeNodePool:
		return r.evaluateNodePoolHealth(ctx, targetName)
	default:
		return healthv1alpha1.ConditionOngoing, "UnknownScope", "Unsupported health check scope"
	}
}

// --- Prometheus query helpers ---

type promQueryResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []interface{}     `json:"value"` // [timestamp, value_string]
		} `json:"result"`
	} `json:"data"`
}

type promResult struct {
	Metric map[string]string
	Value  string
}

func (r *HealthSignalReconciler) queryPrometheus(ctx context.Context, query string) ([]promResult, error) {
	url := fmt.Sprintf("%s/api/v1/query?query=%s", r.PrometheusAPIURL, query)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := r.getHTTPClient().Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("querying prometheus: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	var promResp promQueryResponse
	if err := json.Unmarshal(body, &promResp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	if promResp.Status != "success" {
		return nil, fmt.Errorf("prometheus query failed with status: %s", promResp.Status)
	}

	results := make([]promResult, 0, len(promResp.Data.Result))
	for _, r := range promResp.Data.Result {
		val := ""
		if len(r.Value) >= 2 {
			val = fmt.Sprintf("%v", r.Value[1])
		}
		results = append(results, promResult{
			Metric: r.Metric,
			Value:  val,
		})
	}
	return results, nil
}

// queryHealthMetric queries Prometheus for an upgrade-health metric, using the
// typed cache to dedup and record results. The metricType + target identify
// what health signal this query belongs to.
func (r *HealthSignalReconciler) queryHealthMetric(ctx context.Context, metricType HealthMetricType, target string, query string) ([]promResult, error) {
	if r.Cache != nil {
		if cached, ok := r.Cache.Get(metricType, target); ok {
			return cached, nil
		}
	}

	results, err := r.queryPrometheus(ctx, query)
	if err != nil {
		return nil, err
	}

	if r.Cache != nil {
		r.Cache.Record(metricType, target, results)
	}
	return results, nil
}

// getNodeCount returns the current number of nodes in the cluster by querying
// kube_node_info. Falls back to 1 if the query fails or returns no data.
func (r *HealthSignalReconciler) getNodeCount(ctx context.Context) int {
	results, err := r.queryHealthMetric(ctx, MetricClusterNodeCount, "", `count(kube_node_info)`)
	if err != nil || len(results) == 0 {
		return 1
	}
	n, err := strconv.Atoi(results[0].Value)
	if err != nil || n < 1 {
		return 1
	}
	return n
}

// reconcileWithStatus creates/updates the HealthSignal for a given HealthCheckRequest
// and sets the status condition in a single helper. Used by the upgrade gate short-circuit.
func (r *HealthSignalReconciler) reconcileWithStatus(ctx context.Context, hcr *healthv1alpha1.HealthCheckRequest,
	conditionStatus, reason, message string) (ctrl.Result, error) {

	logger := ctrllog.FromContext(ctx)

	signalName := healthSignalPrefix + hcr.Name
	signal := &healthv1alpha1.HealthSignal{}
	signal.Name = signalName

	signalType := healthv1alpha1.NodeHealth
	targetRef := corev1.ObjectReference{Name: hcr.Spec.TargetName}

	switch hcr.Spec.Scope {
	case healthv1alpha1.HealthCheckRequestScopeNode:
		signalType = healthv1alpha1.NodeHealth
		targetRef.APIVersion = "v1"
		targetRef.Kind = "Node"
	case healthv1alpha1.HealthCheckRequestScopeCluster:
		signalType = healthv1alpha1.ClusterHealth
		targetRef.APIVersion = "v1"
		targetRef.Kind = "Namespace"
		targetRef.Name = "default"
	case healthv1alpha1.HealthCheckRequestScopeNodePool:
		signalType = healthv1alpha1.NodeHealth
		targetRef.APIVersion = "v1"
		targetRef.Kind = "Node"
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, signal, func() error {
		signal.Spec = healthv1alpha1.HealthSignalSpec{
			Type:      signalType,
			TargetRef: targetRef,
		}
		signal.OwnerReferences = []metav1.OwnerReference{
			{
				APIVersion: healthv1alpha1.GroupVersion.String(),
				Kind:       "HealthCheckRequest",
				Name:       hcr.Name,
				UID:        hcr.UID,
			},
		}
		if signal.Annotations == nil {
			signal.Annotations = make(map[string]string)
		}
		if v, ok := hcr.Annotations["kubernetes.azure.com/upgradeCorrelationID"]; ok {
			signal.Annotations["kubernetes.azure.com/upgradeCorrelationID"] = v
		}
		signal.Annotations["health.aks.io/request-name"] = hcr.Name
		return nil
	})
	if err != nil {
		logger.Error(err, "Failed to create or update HealthSignal", "name", signalName)
		return ctrl.Result{}, err
	}

	now := metav1.Now()
	condition := metav1.Condition{
		Type:               conditionTypeReady,
		Status:             metav1.ConditionStatus(conditionStatus),
		ObservedGeneration: signal.Generation,
		LastTransitionTime: now,
		Reason:             reason,
		Message:            message,
	}
	meta.SetStatusCondition(&signal.Status.Conditions, condition)

	if err := r.Status().Update(ctx, signal); err != nil {
		logger.Error(err, "Failed to update HealthSignal status", "name", signalName)
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// evaluateUpgradeRules runs each customer-defined rule that matches the current
// scope. Template variables in the query are replaced with the actual target name.
// The cache key includes the target so node/pool-scoped rules cache independently.
// Returns (allowed, failureMessage).
func (r *HealthSignalReconciler) evaluateUpgradeRules(ctx context.Context, rules []UpgradeRule, scope, targetName string) (bool, string) {
	logger := ctrllog.FromContext(ctx)

	var failures []string
	evaluated := 0
	for _, rule := range rules {
		if !rule.MatchesScope(scope) {
			continue
		}
		evaluated++

		// Render the query with scope-specific variables
		renderedQuery := rule.RenderQuery(scope, targetName)

		// Cache key includes target so per-node/pool rules are cached separately
		cacheTarget := rule.Name + "/" + targetName
		results, err := r.queryHealthMetric(ctx, MetricCustomRule, cacheTarget, renderedQuery)
		if err != nil {
			failures = append(failures, fmt.Sprintf("rule %q: query error: %v", rule.Name, err))
			continue
		}

		if len(results) == 0 {
			logger.V(1).Info("Upgrade rule returned no data, skipping", "rule", rule.Name, "scope", scope, "target", targetName)
			continue
		}

		val, err := strconv.ParseFloat(results[0].Value, 64)
		if err != nil {
			failures = append(failures, fmt.Sprintf("rule %q: cannot parse value %q as float: %v", rule.Name, results[0].Value, err))
			continue
		}

		if !CompareValue(val, rule.Operator, rule.Threshold) {
			failures = append(failures, fmt.Sprintf("rule %q: value %.4f does not satisfy %s %.4f",
				rule.Name, val, rule.Operator, rule.Threshold))
		} else {
			logger.V(1).Info("Upgrade rule passed", "rule", rule.Name, "value", val, "scope", scope, "target", targetName)
		}
	}

	if len(failures) > 0 {
		msg := fmt.Sprintf("%d/%d upgrade rule(s) failed: %s", len(failures), evaluated, joinStrings(failures, "; "))
		return false, msg
	}
	return true, ""
}

// joinStrings joins a string slice with a separator — avoids importing strings package.
func joinStrings(ss []string, sep string) string {
	if len(ss) == 0 {
		return ""
	}
	result := ss[0]
	for _, s := range ss[1:] {
		result += sep + s
	}
	return result
}

// SetupWithManager registers the controller to watch HealthCheckRequests
// and own HealthSignals.
func (r *HealthSignalReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&healthv1alpha1.HealthCheckRequest{}).
		Owns(&healthv1alpha1.HealthSignal{}).
		Complete(r)
}
