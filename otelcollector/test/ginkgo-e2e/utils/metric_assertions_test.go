package utils

import (
	"testing"
	"time"

	"github.com/prometheus/common/model"
)

func TestValidateMetricSamples(t *testing.T) {
	now := time.Now()
	sample := func() *model.Sample {
		return &model.Sample{
			Metric: model.Metric{"__name__": "up", "job": "kubelet", "cluster": "test", "instance": "node-a"},
			Value:  1, Timestamp: model.TimeFromUnixNano(now.UnixNano()),
		}
	}
	if err := ValidateMetricSamples(model.Vector{sample()}, "up", "kubelet", "test", now, true); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		mutate func(*model.Sample)
	}{
		{"missing cluster", func(s *model.Sample) { delete(s.Metric, "cluster") }},
		{"wrong cluster", func(s *model.Sample) { s.Metric["cluster"] = "other" }},
		{"missing instance", func(s *model.Sample) { delete(s.Metric, "instance") }},
		{"wrong metric", func(s *model.Sample) { s.Metric["__name__"] = "other" }},
		{"wrong job", func(s *model.Sample) { s.Metric["job"] = "other" }},
		{"stale", func(s *model.Sample) { s.Timestamp = model.TimeFromUnixNano(now.Add(-6 * time.Minute).UnixNano()) }},
		{"future", func(s *model.Sample) { s.Timestamp = model.TimeFromUnixNano(now.Add(time.Minute).UnixNano()) }},
		{"down", func(s *model.Sample) { s.Value = 0 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			bad := sample()
			test.mutate(bad)
			if err := ValidateMetricSamples(model.Vector{sample(), bad}, "up", "kubelet", "test", now, true); err == nil {
				t.Fatal("invalid second sample was accepted")
			}
		})
	}
	for _, samples := range []model.Vector{nil, {nil}} {
		if err := ValidateMetricSamples(samples, "up", "kubelet", "test", now, true); err == nil {
			t.Fatal("missing samples were accepted")
		}
	}
}
