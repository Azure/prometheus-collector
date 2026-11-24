package utils

import (
	"fmt"
	"time"

	"github.com/prometheus/common/model"
)

func FreshMetricQuery(selector string, notBefore time.Time) string {
	query := fmt.Sprintf("(%s) and (timestamp(%s) > time() - 300) and (timestamp(%s) < time() + 30)",
		selector, selector, selector)
	if !notBefore.IsZero() {
		query += fmt.Sprintf(" and (timestamp(%s) >= %.3f)", selector, float64(notBefore.UnixMilli())/1000)
	}
	return query
}

func ValidateMetricSamples(samples model.Vector, metric, job, cluster string, now time.Time, requireUp bool) error {
	if cluster == "" || len(samples) == 0 {
		return fmt.Errorf("expected cluster and nonempty samples for %s/%s", job, metric)
	}
	for _, sample := range samples {
		if sample == nil {
			return fmt.Errorf("nil sample for %s/%s", job, metric)
		}
		for label, expected := range map[model.LabelName]string{"__name__": metric, "job": job, "cluster": cluster} {
			if string(sample.Metric[label]) != expected {
				return fmt.Errorf("%s/%s: label %s=%q, expected %q", job, metric, label, sample.Metric[label], expected)
			}
		}
		if sample.Metric["instance"] == "" {
			return fmt.Errorf("%s/%s: missing instance label", job, metric)
		}
		age := now.Sub(sample.Timestamp.Time())
		if age > 5*time.Minute || age < -30*time.Second {
			return fmt.Errorf("%s/%s: sample timestamp is not fresh (age %s)", job, metric, age)
		}
		if requireUp && sample.Value != 1 {
			return fmt.Errorf("%s: target %s is down (up=%s)", job, sample.Metric["instance"], sample.Value)
		}
	}
	return nil
}
