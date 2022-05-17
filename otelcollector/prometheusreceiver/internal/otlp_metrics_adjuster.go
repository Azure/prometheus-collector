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

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal"

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// Notes on garbage collection (gc):
//
// Job-level gc:
// The Prometheus receiver will likely execute in a long running service whose lifetime may exceed
// the lifetimes of many of the jobs that it is collecting from. In order to keep the JobsMap from
// leaking memory for entries of no-longer existing jobs, the JobsMap needs to remove entries that
// haven't been accessed for a long period of time.
//
// Timeseries-level gc:
// Some jobs that the Prometheus receiver is collecting from may export timeseries based on metrics
// from other jobs (e.g. cAdvisor). In order to keep the timeseriesMap from leaking memory for entries
// of no-longer existing jobs, the timeseriesMap for each job needs to remove entries that haven't
// been accessed for a long period of time.
//
// The gc strategy uses a standard mark-and-sweep approach - each time a timeseriesMap is accessed,
// it is marked. Similarly, each time a timeseriesinfo is accessed, it is also marked.
//
// At the end of each JobsMap.get(), if the last time the JobsMap was gc'd exceeds the 'gcInterval',
// the JobsMap is locked and any timeseriesMaps that are unmarked are removed from the JobsMap
// otherwise the timeseriesMap is gc'd
//
// The gc for the timeseriesMap is straightforward - the map is locked and, for each timeseriesinfo
// in the map, if it has not been marked, it is removed otherwise it is unmarked.
//
// Alternative Strategies
// 1. If the job-level gc doesn't run often enough, or runs too often, a separate go routine can
//    be spawned at JobMap creation time that gc's at periodic intervals. This approach potentially
//    adds more contention and latency to each scrape so the current approach is used. Note that
//    the go routine will need to be cancelled upon Shutdown().
// 2. If the gc of each timeseriesMap during the gc of the JobsMap causes too much contention,
//    the gc of timeseriesMaps can be moved to the end of MetricsAdjuster().AdjustMetricSlice(). This
//    approach requires adding 'lastGC' Time and (potentially) a gcInterval duration to
//    timeseriesMap so the current approach is used instead.

// timeseriesinfo contains the information necessary to adjust from the initial point and to detect
// resets.
type timeseriesinfo struct {
	mark     bool
	initial  *pmetric.Metric
	previous *pmetric.Metric
}

// timeseriesMap maps from a timeseries instance (metric * label values) to the timeseries info for
// the instance.
type timeseriesMap struct {
	sync.RWMutex
	// The mutex is used to protect access to the member fields. It is acquired for the entirety of
	// AdjustMetricSlice() and also acquired by gc().

	mark   bool
	tsiMap map[string]*timeseriesinfo
}

// Get the timeseriesinfo for the timeseries associated with the metric and label values.
func (tsm *timeseriesMap) get(metric *pmetric.Metric, kv pcommon.Map) *timeseriesinfo {
	// This should only be invoked be functions called (directly or indirectly) by AdjustMetricSlice().
	// The lock protecting tsm.tsiMap is acquired there.
	name := metric.Name()
	sig := getTimeseriesSignature(name, kv)
	if metric.DataType() == pmetric.MetricDataTypeHistogram {
		// There are 2 types of Histograms whose aggregation temporality needs distinguishing:
		// * CumulativeHistogram
		// * GaugeHistogram
		aggTemporality := metric.Histogram().AggregationTemporality()
		sig += "," + aggTemporality.String()
	}
	tsi, ok := tsm.tsiMap[sig]
	if !ok {
		tsi = &timeseriesinfo{}
		tsm.tsiMap[sig] = tsi
	}
	tsm.mark = true
	tsi.mark = true
	return tsi
}

// Create a unique timeseries signature consisting of the metric name and label values.
func getTimeseriesSignature(name string, kv pcommon.Map) string {
	labelValues := make([]string, 0, kv.Len())
	kv.Sort().Range(func(_ string, attrValue pcommon.Value) bool {
		value := attrValue.StringVal()
		if value != "" {
			labelValues = append(labelValues, value)
		}
		return true
	})
	return fmt.Sprintf("%s,%s", name, strings.Join(labelValues, ","))
}

// Remove timeseries that have aged out.
func (tsm *timeseriesMap) gc() {
	tsm.Lock()
	defer tsm.Unlock()
	// this shouldn't happen under the current gc() strategy
	if !tsm.mark {
		return
	}
	for ts, tsi := range tsm.tsiMap {
		if !tsi.mark {
			delete(tsm.tsiMap, ts)
		} else {
			tsi.mark = false
		}
	}
	tsm.mark = false
}

func newTimeseriesMap() *timeseriesMap {
	return &timeseriesMap{mark: true, tsiMap: map[string]*timeseriesinfo{}}
}

// JobsMap maps from a job instance to a map of timeseries instances for the job.
type JobsMap struct {
	sync.RWMutex
	// The mutex is used to protect access to the member fields. It is acquired for most of
	// get() and also acquired by gc().

	gcInterval time.Duration
	lastGC     time.Time
	jobsMap    map[string]*timeseriesMap
}

// NewJobsMap creates a new (empty) JobsMap.
func NewJobsMap(gcInterval time.Duration) *JobsMap {
	return &JobsMap{gcInterval: gcInterval, lastGC: time.Now(), jobsMap: make(map[string]*timeseriesMap)}
}

// Remove jobs and timeseries that have aged out.
func (jm *JobsMap) gc() {
	jm.Lock()
	defer jm.Unlock()
	// once the structure is locked, confirm that gc() is still necessary
	if time.Since(jm.lastGC) > jm.gcInterval {
		for sig, tsm := range jm.jobsMap {
			tsm.RLock()
			tsmNotMarked := !tsm.mark
			// take a read lock here, no need to get a full lock as we have a lock on the JobsMap
			tsm.RUnlock()
			if tsmNotMarked {
				delete(jm.jobsMap, sig)
			} else {
				// a full lock will be obtained in here, if required.
				tsm.gc()
			}
		}
		jm.lastGC = time.Now()
	}
}

func (jm *JobsMap) maybeGC() {
	// speculatively check if gc() is necessary, recheck once the structure is locked
	jm.RLock()
	defer jm.RUnlock()
	if time.Since(jm.lastGC) > jm.gcInterval {
		go jm.gc()
	}
}

func (jm *JobsMap) get(job, instance string) *timeseriesMap {
	sig := job + ":" + instance
	// a read locke is taken here as we will not need to modify jobsMap if the target timeseriesMap is available.
	jm.RLock()
	tsm, ok := jm.jobsMap[sig]
	jm.RUnlock()
	defer jm.maybeGC()
	if ok {
		return tsm
	}
	jm.Lock()
	defer jm.Unlock()
	// Now that we've got an exclusive lock, check once more to ensure an entry wasn't created in the interim
	// and then create a new timeseriesMap if required.
	tsm2, ok2 := jm.jobsMap[sig]
	if ok2 {
		return tsm2
	}
	tsm2 = newTimeseriesMap()
	jm.jobsMap[sig] = tsm2
	return tsm2
}

// MetricsAdjuster takes a map from a metric instance to the initial point in the metrics instance
// and provides AdjustMetricSlice, which takes a sequence of metrics and adjust their start times based on
// the initial points.
type MetricsAdjuster struct {
	tsm    *timeseriesMap
	logger *zap.Logger
}

// NewMetricsAdjuster is a constructor for MetricsAdjuster.
func NewMetricsAdjuster(tsm *timeseriesMap, logger *zap.Logger) *MetricsAdjuster {
	return &MetricsAdjuster{
		tsm:    tsm,
		logger: logger,
	}
}

// AdjustMetricSlice takes a sequence of metrics and adjust their start times based on the initial and
// previous points in the timeseriesMap.
// Returns the total number of timeseries that had reset start times.
func (ma *MetricsAdjuster) AdjustMetricSlice(metricL *pmetric.MetricSlice) int {
	resets := 0
	// The lock on the relevant timeseriesMap is held throughout the adjustment process to ensure that
	// nothing else can modify the data used for adjustment.
	ma.tsm.Lock()
	defer ma.tsm.Unlock()
	for i := 0; i < metricL.Len(); i++ {
		metric := metricL.At(i)
		resets += ma.adjustMetric(&metric)
	}
	return resets
}

// AdjustMetrics takes a sequence of metrics and adjust their start times based on the initial and
// previous points in the timeseriesMap.
// Returns the total number of timeseries that had reset start times.
func (ma *MetricsAdjuster) AdjustMetrics(metrics *pmetric.Metrics) int {
	resets := 0
	// The lock on the relevant timeseriesMap is held throughout the adjustment process to ensure that
	// nothing else can modify the data used for adjustment.
	ma.tsm.Lock()
	defer ma.tsm.Unlock()
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			ilm := rm.ScopeMetrics().At(j)
			for k := 0; k < ilm.Metrics().Len(); k++ {
				metric := ilm.Metrics().At(k)
				resets += ma.adjustMetric(&metric)
			}
		}
	}
	return resets
}

// Returns the number of timeseries with reset start times.
func (ma *MetricsAdjuster) adjustMetric(metric *pmetric.Metric) int {
	switch metric.DataType() {
	case pmetric.MetricDataTypeGauge:
		// gauges don't need to be adjusted so no additional processing is necessary
		return 0
	default:
		return ma.adjustMetricPoints(metric)
	}
}

// Returns  the number of timeseries that had reset start times.
func (ma *MetricsAdjuster) adjustMetricPoints(metric *pmetric.Metric) int {
	switch dataType := metric.DataType(); dataType {
	case pmetric.MetricDataTypeGauge:
		return ma.adjustMetricGauge(metric)

	case pmetric.MetricDataTypeHistogram:
		return ma.adjustMetricHistogram(metric)

	case pmetric.MetricDataTypeSummary:
		return ma.adjustMetricSummary(metric)

	case pmetric.MetricDataTypeSum:
		return ma.adjustMetricSum(metric)

	default:
		// this shouldn't happen
		ma.logger.Info("Adjust - skipping unexpected point", zap.String("type", dataType.String()))
		return 0
	}
}

// Returns true if 'current' was adjusted and false if 'current' is an the initial occurrence or a
// reset of the timeseries.
func (ma *MetricsAdjuster) adjustMetricGauge(current *pmetric.Metric) (resets int) {
	currentPoints := current.Gauge().DataPoints()

	for i := 0; i < currentPoints.Len(); i++ {
		currentGauge := currentPoints.At(i)
		tsi := ma.tsm.get(current, currentGauge.Attributes())

		previous := tsi.previous
		if previous == nil {
			// no previous data point with values
			// use the initial data point
			previous = tsi.initial
		}
		tsi.previous = current

		if tsi.initial == nil {
			// initial || reset timeseries.
			tsi.initial = current
			resets++
			continue
		}

		initialPoints := tsi.initial.Gauge().DataPoints()
		previousPoints := previous.Gauge().DataPoints()
		if i >= initialPoints.Len() || i >= previousPoints.Len() {
			ma.logger.Debug("Adjusting Points, all lengths should be equal",
				zap.Int("len(current)", currentPoints.Len()),
				zap.Int("len(initial)", initialPoints.Len()),
				zap.Int("len(previous)", previousPoints.Len()))
			// initial || reset timeseries.
			tsi.initial = current
			resets++
			continue
		}

		currentGauge, previousGauge := currentPoints.At(i), previousPoints.At(i)
		if currentGauge.DoubleVal() < previousGauge.DoubleVal() {
			// reset detected
			tsi.initial = current
			continue
		}
		initialGauge := initialPoints.At(i)
		currentGauge.SetStartTimestamp(initialGauge.StartTimestamp())
		resets++
	}
	return
}

func (ma *MetricsAdjuster) adjustMetricHistogram(current *pmetric.Metric) (resets int) {
	histogram := current.Histogram()
	if histogram.AggregationTemporality() != pmetric.MetricAggregationTemporalityCumulative {
		// Only dealing with CumulativeDistributions.
		return 0
	}

	// Note: Sum of Squared Deviation not currently supported.
	currentPoints := histogram.DataPoints()

	if currentPoints.Len() == 0 {
		return 0
	}

	for i := 0; i < currentPoints.Len(); i++ {
		currentDist := currentPoints.At(i)
		tsi := ma.tsm.get(current, currentDist.Attributes())

		previous := tsi.previous
		if previous == nil {
			// no previous data point with values
			// use the initial data point
			previous = tsi.initial
		}

		if !currentDist.Flags().HasFlag(pmetric.MetricDataPointFlagNoRecordedValue) {
			tsi.previous = current
		}

		if tsi.initial == nil {
			// initial || reset timeseries.
			tsi.initial = current
			resets++
			continue
		}

		initialPoints := tsi.initial.Histogram().DataPoints()
		previousPoints := previous.Histogram().DataPoints()
		if i >= initialPoints.Len() || i >= previousPoints.Len() {
			ma.logger.Debug("Adjusting Points, all lengths should be equal",
				zap.Int("len(current)", currentPoints.Len()),
				zap.Int("len(initial)", initialPoints.Len()),
				zap.Int("len(previous)", previousPoints.Len()))
			// initial || reset timeseries.
			tsi.initial = current
			resets++
			continue
		}
		if currentDist.Flags().HasFlag(pmetric.MetricDataPointFlagNoRecordedValue) {
			currentDist.SetStartTimestamp(initialPoints.At(i).StartTimestamp())
			continue
		}
		previousDist := previousPoints.At(i)
		if currentDist.Count() < previousDist.Count() || currentDist.Sum() < previousDist.Sum() {
			// reset detected
			tsi.initial = current
			resets++
			continue
		}
		initialDist := initialPoints.At(i)
		currentDist.SetStartTimestamp(initialDist.StartTimestamp())
	}
	return
}

func (ma *MetricsAdjuster) adjustMetricSum(current *pmetric.Metric) (resets int) {
	currentPoints := current.Sum().DataPoints()

	for i := 0; i < currentPoints.Len(); i++ {
		currentSum := currentPoints.At(i)
		tsi := ma.tsm.get(current, currentSum.Attributes())

		previous := tsi.previous
		if previous == nil {
			// no previous data point with values
			// use the initial data point
			previous = tsi.initial
		}

		if !currentSum.Flags().HasFlag(pmetric.MetricDataPointFlagNoRecordedValue) {
			tsi.previous = current
		}

		if tsi.initial == nil {
			// initial || reset timeseries.
			tsi.initial = current
			resets++
			continue
		}

		initialPoints := tsi.initial.Sum().DataPoints()
		previousPoints := previous.Sum().DataPoints()
		if i >= initialPoints.Len() || i >= previousPoints.Len() {
			ma.logger.Debug("Adjusting Points, all lengths should be equal",
				zap.Int("len(current)", currentPoints.Len()),
				zap.Int("len(initial)", initialPoints.Len()),
				zap.Int("len(previous)", previousPoints.Len()))
			tsi.initial = current
			resets++
			continue
		}
		if currentSum.Flags().HasFlag(pmetric.MetricDataPointFlagNoRecordedValue) {
			currentSum.SetStartTimestamp(initialPoints.At(i).StartTimestamp())
			continue
		}
		previousSum := previousPoints.At(i)
		if currentSum.DoubleVal() < previousSum.DoubleVal() {
			// reset detected
			tsi.initial = current
			resets++
			continue
		}
		initialSum := initialPoints.At(i)
		currentSum.SetStartTimestamp(initialSum.StartTimestamp())
	}

	return
}

func (ma *MetricsAdjuster) adjustMetricSummary(current *pmetric.Metric) (resets int) {
	currentPoints := current.Summary().DataPoints()

	for i := 0; i < currentPoints.Len(); i++ {
		currentSummary := currentPoints.At(i)
		tsi := ma.tsm.get(current, currentSummary.Attributes())

		previous := tsi.previous
		if previous == nil {
			// no previous data point with values
			// use the initial data point
			previous = tsi.initial
		}

		if !currentSummary.Flags().HasFlag(pmetric.MetricDataPointFlagNoRecordedValue) {
			tsi.previous = current
		}

		if tsi.initial == nil {
			// initial || reset timeseries.
			tsi.initial = current
			resets++
			continue
		}

		initialPoints := tsi.initial.Summary().DataPoints()
		previousPoints := previous.Summary().DataPoints()
		if i >= initialPoints.Len() || i >= previousPoints.Len() {
			ma.logger.Debug("Adjusting Points, all lengths should be equal",
				zap.Int("len(current)", currentPoints.Len()),
				zap.Int("len(initial)", initialPoints.Len()),
				zap.Int("len(previous)", previousPoints.Len()))
			tsi.initial = current
			resets++
			continue
		}
		if currentSummary.Flags().HasFlag(pmetric.MetricDataPointFlagNoRecordedValue) {
			currentSummary.SetStartTimestamp(initialPoints.At(i).StartTimestamp())
			continue
		}
		previousSummary := previousPoints.At(i)
		if (currentSummary.Count() != 0 &&
			previousSummary.Count() != 0 &&
			currentSummary.Count() < previousSummary.Count()) ||
			(currentSummary.Sum() != 0 &&
				previousSummary.Sum() != 0 &&
				currentSummary.Sum() < previousSummary.Sum()) {
			// reset detected
			tsi.initial = current
			resets++
			continue
		}
		initialSummary := initialPoints.At(i)
		currentSummary.SetStartTimestamp(initialSummary.StartTimestamp())
	}

	return
}
