package controller

// HealthMetricSample is a single scraped data point relevant to upgrade health.
// Produced by the scrape pipeline interceptor, consumed by the MetricsCache.
type HealthMetricSample struct {
	MetricType HealthMetricType
	Target     string            // node name, pool name, etc.
	Labels     map[string]string // all labels from the data point
	Value      float64
}

// HealthMetricSink receives upgrade-health samples from the scrape pipeline.
// The scrape interceptor (in the receiver module) pushes samples here.
type HealthMetricSink struct {
	cache *MetricsCache
}

// NewHealthMetricSink creates a sink that records samples into the given cache.
func NewHealthMetricSink(cache *MetricsCache) *HealthMetricSink {
	return &HealthMetricSink{cache: cache}
}

// Ingest records a batch of health metric samples into the cache.
func (s *HealthMetricSink) Ingest(samples []HealthMetricSample) {
	for _, sample := range samples {
		s.cache.Record(sample.MetricType, sample.Target, []promResult{
			{
				Metric: sample.Labels,
				Value:  formatFloat(sample.Value),
			},
		})
	}
}
