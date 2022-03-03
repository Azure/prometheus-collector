// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package internal

import (
	"testing"
	"time"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/textparse"
	"github.com/prometheus/prometheus/scrape"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

type byLookupMetadataCache map[string]scrape.MetricMetadata

func (bmc byLookupMetadataCache) Metadata(familyName string) (scrape.MetricMetadata, bool) {
	lookup, ok := bmc[familyName]
	return lookup, ok
}

func (bmc byLookupMetadataCache) SharedLabels() labels.Labels {
	return nil
}

var mc = byLookupMetadataCache{
	"counter": scrape.MetricMetadata{
		Metric: "cr",
		Type:   textparse.MetricTypeCounter,
		Help:   "This is some help for a counter",
		Unit:   "By",
	},
	"gauge": scrape.MetricMetadata{
		Metric: "ge",
		Type:   textparse.MetricTypeGauge,
		Help:   "This is some help for a gauge",
		Unit:   "1",
	},
	"gaugehistogram": scrape.MetricMetadata{
		Metric: "gh",
		Type:   textparse.MetricTypeGaugeHistogram,
		Help:   "This is some help for a gauge histogram",
		Unit:   "?",
	},
	"histogram": scrape.MetricMetadata{
		Metric: "hg",
		Type:   textparse.MetricTypeHistogram,
		Help:   "This is some help for a histogram",
		Unit:   "ms",
	},
	"summary": scrape.MetricMetadata{
		Metric: "s",
		Type:   textparse.MetricTypeSummary,
		Help:   "This is some help for a summary",
		Unit:   "ms",
	},
	"unknown": scrape.MetricMetadata{
		Metric: "u",
		Type:   textparse.MetricTypeUnknown,
		Help:   "This is some help for an unknown metric",
		Unit:   "?",
	},
}

func TestMetricGroupData_toDistributionUnitTest(t *testing.T) {
	type scrape struct {
		at     int64
		value  float64
		metric string
	}
	tests := []struct {
		name                string
		metricName          string
		labels              labels.Labels
		scrapes             []*scrape
		want                func() pdata.HistogramDataPoint
		intervalStartTimeMs int64
	}{
		{
			name:                "histogram with startTimestamp of 11",
			metricName:          "histogram",
			intervalStartTimeMs: 11,
			labels:              labels.Labels{{Name: "a", Value: "A"}, {Name: "le", Value: "0.75"}, {Name: "b", Value: "B"}},
			scrapes: []*scrape{
				{at: 11, value: 10, metric: "histogram_count"},
				{at: 11, value: 1004.78, metric: "histogram_sum"},
				{at: 13, value: 33.7, metric: "value"},
			},
			want: func() pdata.HistogramDataPoint {
				point := pdata.NewHistogramDataPoint()
				point.SetCount(10)
				point.SetSum(1004.78)
				point.SetTimestamp(pdata.Timestamp(11 * time.Millisecond)) // the time in milliseconds -> nanoseconds.
				point.SetBucketCounts([]uint64{33})
				point.SetExplicitBounds([]float64{})
				point.SetStartTimestamp(pdata.Timestamp(11 * time.Millisecond)) // the time in milliseconds -> nanoseconds.
				attributes := point.Attributes()
				attributes.InsertString("a", "A")
				attributes.InsertString("b", "B")
				return point
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			mp := newMetricFamilyPdata(tt.metricName, mc, zap.NewNop())
			for _, tv := range tt.scrapes {
				require.NoError(t, mp.Add(tv.metric, tt.labels.Copy(), tv.at, tv.value))
			}

			require.Equal(t, 1, len(mp.groups), "Expecting exactly 1 groupKey")
			groupKey := mp.getGroupKey(tt.labels.Copy())
			require.NotNil(t, mp.groups[groupKey], "Expecting the groupKey to have a value given key:: "+groupKey)

			sl := pdata.NewMetricSlice()
			mp.ToMetricPdata(&sl)

			require.Equal(t, 1, sl.Len(), "Exactly one metric expected")
			metric := sl.At(0)
			require.Equal(t, mc[tt.metricName].Help, metric.Description(), "Expected help metadata in metric description")
			require.Equal(t, mc[tt.metricName].Unit, metric.Unit(), "Expected unit metadata in metric")

			hdpL := metric.Histogram().DataPoints()
			require.Equal(t, 1, hdpL.Len(), "Exactly one point expected")
			got := hdpL.At(0)
			want := tt.want()
			require.Equal(t, want, got, "Expected the points to be equal")
		})
	}
}

func TestMetricGroupData_toSummaryUnitTest(t *testing.T) {
	type scrape struct {
		at     int64
		value  float64
		metric string
	}

	type labelsScrapes struct {
		labels  labels.Labels
		scrapes []*scrape
	}
	tests := []struct {
		name          string
		labelsScrapes []*labelsScrapes
		want          func() pdata.SummaryDataPoint
	}{
		{
			name: "summary",
			labelsScrapes: []*labelsScrapes{
				{
					labels: labels.Labels{
						{Name: "a", Value: "A"}, {Name: "quantile", Value: "0.0"}, {Name: "b", Value: "B"},
					},
					scrapes: []*scrape{
						{at: 10, value: 10, metric: "summary_count"},
						{at: 10, value: 12, metric: "summary_sum"},
						{at: 10, value: 8, metric: "value"},
					},
				},
				{
					labels: labels.Labels{
						{Name: "a", Value: "A"}, {Name: "quantile", Value: "0.75"}, {Name: "b", Value: "B"},
					},
					scrapes: []*scrape{
						{at: 11, value: 10, metric: "summary_count"},
						{at: 11, value: 1004.78, metric: "summary_sum"},
						{at: 11, value: 33.7, metric: "value"},
					},
				},
				{
					labels: labels.Labels{
						{Name: "a", Value: "A"}, {Name: "quantile", Value: "0.50"}, {Name: "b", Value: "B"},
					},
					scrapes: []*scrape{
						{at: 12, value: 10, metric: "summary_count"},
						{at: 12, value: 13, metric: "summary_sum"},
						{at: 12, value: 27, metric: "value"},
					},
				},
				{
					labels: labels.Labels{
						{Name: "a", Value: "A"}, {Name: "quantile", Value: "0.90"}, {Name: "b", Value: "B"},
					},
					scrapes: []*scrape{
						{at: 13, value: 10, metric: "summary_count"},
						{at: 13, value: 14, metric: "summary_sum"},
						{at: 13, value: 56, metric: "value"},
					},
				},
				{
					labels: labels.Labels{
						{Name: "a", Value: "A"}, {Name: "quantile", Value: "0.99"}, {Name: "b", Value: "B"},
					},
					scrapes: []*scrape{
						{at: 14, value: 10, metric: "summary_count"},
						{at: 14, value: 15, metric: "summary_sum"},
						{at: 14, value: 82, metric: "value"},
					},
				},
			},
			want: func() pdata.SummaryDataPoint {
				point := pdata.NewSummaryDataPoint()
				point.SetCount(10)
				point.SetSum(15)
				qtL := point.QuantileValues()
				qn0 := qtL.AppendEmpty()
				qn0.SetQuantile(0)
				qn0.SetValue(8)
				qn50 := qtL.AppendEmpty()
				qn50.SetQuantile(.5)
				qn50.SetValue(27)
				qn75 := qtL.AppendEmpty()
				qn75.SetQuantile(.75)
				qn75.SetValue(33.7)
				qn90 := qtL.AppendEmpty()
				qn90.SetQuantile(.9)
				qn90.SetValue(56)
				qn99 := qtL.AppendEmpty()
				qn99.SetQuantile(.99)
				qn99.SetValue(82)
				point.SetTimestamp(pdata.Timestamp(14 * time.Millisecond))      // the time in milliseconds -> nanoseconds.
				point.SetStartTimestamp(pdata.Timestamp(14 * time.Millisecond)) // the time in milliseconds -> nanoseconds
				attributes := point.Attributes()
				attributes.InsertString("a", "A")
				attributes.InsertString("b", "B")
				return point
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			mp := newMetricFamilyPdata(tt.name, mc, zap.NewNop())
			for _, lbs := range tt.labelsScrapes {
				for _, scrape := range lbs.scrapes {
					require.NoError(t, mp.Add(scrape.metric, lbs.labels.Copy(), scrape.at, scrape.value))
				}
			}

			require.Equal(t, 1, len(mp.groups), "Expecting exactly 1 groupKey")
			// Get the lone group key.
			groupKey := ""
			for key := range mp.groups {
				groupKey = key
			}
			require.NotNil(t, mp.groups[groupKey], "Expecting the groupKey to have a value given key:: "+groupKey)

			sl := pdata.NewMetricSlice()
			mp.ToMetricPdata(&sl)

			require.Equal(t, 1, sl.Len(), "Exactly one metric expected")
			metric := sl.At(0)
			require.Equal(t, mc[tt.name].Help, metric.Description(), "Expected help metadata in metric description")
			require.Equal(t, mc[tt.name].Unit, metric.Unit(), "Expected unit metadata in metric")

			sdpL := metric.Summary().DataPoints()
			require.Equal(t, 1, sdpL.Len(), "Exactly one point expected")
			got := sdpL.At(0)
			want := tt.want()
			require.Equal(t, want, got, "Expected the points to be equal")
		})
	}
}

func TestMetricGroupData_toNumberDataUnitTest(t *testing.T) {
	type scrape struct {
		at     int64
		value  float64
		metric string
	}
	tests := []struct {
		name                     string
		metricKind               string
		labels                   labels.Labels
		scrapes                  []*scrape
		intervalStartTimestampMs int64
		want                     func() pdata.NumberDataPoint
	}{
		{
			metricKind:               "counter",
			name:                     "counter:: startTimestampMs of 11",
			intervalStartTimestampMs: 11,
			labels:                   labels.Labels{{Name: "a", Value: "A"}, {Name: "b", Value: "B"}},
			scrapes: []*scrape{
				{at: 13, value: 33.7, metric: "value"},
			},
			want: func() pdata.NumberDataPoint {
				point := pdata.NewNumberDataPoint()
				point.SetDoubleVal(33.7)
				point.SetTimestamp(pdata.Timestamp(13 * time.Millisecond))      // the time in milliseconds -> nanoseconds.
				point.SetStartTimestamp(pdata.Timestamp(13 * time.Millisecond)) // the time in milliseconds -> nanoseconds.
				attributes := point.Attributes()
				attributes.InsertString("a", "A")
				attributes.InsertString("b", "B")
				return point
			},
		},
		{
			name:                     "counter:: startTimestampMs of 0",
			metricKind:               "counter",
			intervalStartTimestampMs: 0,
			labels:                   labels.Labels{{Name: "a", Value: "A"}, {Name: "b", Value: "B"}},
			scrapes: []*scrape{
				{at: 28, value: 99.9, metric: "value"},
			},
			want: func() pdata.NumberDataPoint {
				point := pdata.NewNumberDataPoint()
				point.SetDoubleVal(99.9)
				point.SetTimestamp(pdata.Timestamp(28 * time.Millisecond))      // the time in milliseconds -> nanoseconds.
				point.SetStartTimestamp(pdata.Timestamp(28 * time.Millisecond)) // the time in milliseconds -> nanoseconds.
				attributes := point.Attributes()
				attributes.InsertString("a", "A")
				attributes.InsertString("b", "B")
				return point
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			mp := newMetricFamilyPdata(tt.metricKind, mc, zap.NewNop())
			for _, tv := range tt.scrapes {
				require.NoError(t, mp.Add(tv.metric, tt.labels.Copy(), tv.at, tv.value))
			}

			require.Equal(t, 1, len(mp.groups), "Expecting exactly 1 groupKey")
			groupKey := mp.getGroupKey(tt.labels.Copy())
			require.NotNil(t, mp.groups[groupKey], "Expecting the groupKey to have a value given key:: "+groupKey)

			sl := pdata.NewMetricSlice()
			mp.ToMetricPdata(&sl)

			require.Equal(t, 1, sl.Len(), "Exactly one metric expected")
			metric := sl.At(0)
			require.Equal(t, mc[tt.metricKind].Help, metric.Description(), "Expected help metadata in metric description")
			require.Equal(t, mc[tt.metricKind].Unit, metric.Unit(), "Expected unit metadata in metric")

			ndpL := metric.Sum().DataPoints()
			require.Equal(t, 1, ndpL.Len(), "Exactly one point expected")
			got := ndpL.At(0)
			want := tt.want()
			require.Equal(t, want, got, "Expected the points to be equal")
		})
	}
}
