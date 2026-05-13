package controller

import (
	"sync"
	"time"
)

// HealthMetricType identifies the category of upgrade-health metric being cached.
type HealthMetricType string

const (
	MetricNodeReady          HealthMetricType = "node_ready"
	MetricNetworkUnavailable HealthMetricType = "network_unavailable"
	MetricPodHealth          HealthMetricType = "pod_health"
	MetricPodTotal           HealthMetricType = "pod_total"
	MetricPDBRestriction     HealthMetricType = "pdb_restriction"
	MetricClusterNodeCount   HealthMetricType = "cluster_node_count"
	MetricClusterNotReady    HealthMetricType = "cluster_not_ready"
	MetricClusterNetUnavail  HealthMetricType = "cluster_net_unavailable"
	MetricPoolReady          HealthMetricType = "pool_ready"
	MetricPoolNetwork        HealthMetricType = "pool_network"
	MetricCustomRule         HealthMetricType = "custom_rule"
)

// healthMetricKey uniquely identifies a cached metric — type + optional target (node/pool name, rule name).
type healthMetricKey struct {
	Type   HealthMetricType
	Target string // node name, pool name, rule name, or "" for cluster-wide
}

// HealthSnapshot is a single recorded data point for a health metric.
type HealthSnapshot struct {
	Results   []promResult
	Timestamp time.Time
}

// MetricsCache stores upgrade-health metric snapshots in a sliding window.
// Only metrics relevant to upgrade health decisions (node readiness, network,
// pod health, PDB, customer rules) are stored.
type MetricsCache struct {
	mu       sync.RWMutex
	history  map[healthMetricKey][]HealthSnapshot
	window   time.Duration
	dedupTTL time.Duration
}

// NewMetricsCache creates a new MetricsCache.
//   - window: how long historical snapshots are retained (e.g. 1 hour).
//   - dedupTTL: minimum interval between recording new snapshots for the same key.
func NewMetricsCache(window, dedupTTL time.Duration) *MetricsCache {
	return &MetricsCache{
		history:  make(map[healthMetricKey][]HealthSnapshot),
		window:   window,
		dedupTTL: dedupTTL,
	}
}

// Get returns the most recent snapshot for the given health metric if it was
// recorded within the dedupTTL. Returns nil, false if stale or absent.
func (c *MetricsCache) Get(metricType HealthMetricType, target string) ([]promResult, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := healthMetricKey{Type: metricType, Target: target}
	entries := c.history[key]
	if len(entries) == 0 {
		return nil, false
	}

	latest := entries[len(entries)-1]
	if time.Since(latest.Timestamp) > c.dedupTTL {
		return nil, false
	}

	copied := make([]promResult, len(latest.Results))
	copy(copied, latest.Results)
	return copied, true
}

// Record appends a new snapshot for the given health metric and prunes old entries.
func (c *MetricsCache) Record(metricType HealthMetricType, target string, results []promResult) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := healthMetricKey{Type: metricType, Target: target}
	now := time.Now()
	c.history[key] = append(c.history[key], HealthSnapshot{
		Results:   results,
		Timestamp: now,
	})
	c.pruneUnsafe(key, now)
}

// GetHistory returns all snapshots within the retention window for a health metric,
// ordered oldest to newest.
func (c *MetricsCache) GetHistory(metricType HealthMetricType, target string) []HealthSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := healthMetricKey{Type: metricType, Target: target}
	cutoff := time.Now().Add(-c.window)
	entries := c.history[key]

	var result []HealthSnapshot
	for _, e := range entries {
		if e.Timestamp.After(cutoff) {
			cp := HealthSnapshot{
				Results:   make([]promResult, len(e.Results)),
				Timestamp: e.Timestamp,
			}
			copy(cp.Results, e.Results)
			result = append(result, cp)
		}
	}
	return result
}

// Flush removes all entries from the cache.
func (c *MetricsCache) Flush() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.history = make(map[healthMetricKey][]HealthSnapshot)
}

// SetWindow updates the retention window. Existing entries older than the new
// window are pruned on the next EvictExpired or Record call.
func (c *MetricsCache) SetWindow(window time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.window = window
}

// EvictExpired removes snapshots older than the window across all keys.
func (c *MetricsCache) EvictExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key := range c.history {
		c.pruneUnsafe(key, now)
		if len(c.history[key]) == 0 {
			delete(c.history, key)
		}
	}
}

// pruneUnsafe removes entries older than the window. Must be called with mu held.
func (c *MetricsCache) pruneUnsafe(key healthMetricKey, now time.Time) {
	cutoff := now.Add(-c.window)
	entries := c.history[key]

	i := 0
	for i < len(entries) && !entries[i].Timestamp.After(cutoff) {
		i++
	}
	if i > 0 {
		c.history[key] = entries[i:]
	}
}
