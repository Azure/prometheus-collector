package shared

import (
	"log"
	"math"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// TimeseriesReceivedTotal adds up the number of timeseries received that is logged by ME in a minute
	TimeseriesReceivedTotal float64 = 0

	// TimeseriesSentTotal adds up the number of timeseries sent that is logged by ME in a minute
	TimeseriesSentTotal float64 = 0

	// BytesSentTotal adds up the number of timeseries sent that is logged by ME in a minute
	BytesSentTotal float64 = 0

	// TimeseriesVolumeTicker tracks the minute-long period for adding up the number of timeseries and bytes logged by ME
	TimeseriesVolumeTicker *time.Ticker

	// TimeseriesVolumeMutex handles adding to the timeseries volume totals and setting these values as gauges for Prometheus metrics
	TimeseriesVolumeMutex = &sync.Mutex{}

	// OtelColExportingFailedMutex protects the otelcollector export failure event count
	OtelColExportingFailedMutex = &sync.Mutex{}

	// OtelColExportFailureEventCount tracks the number of times otelcollector logged "Exporting failed"
	OtelColExportFailureEventCount = 0

	// MEDroppedMutex protects MEDroppedCount
	MEDroppedMutex = &sync.Mutex{}

	// MEDroppedCount tracks the number of metric points ME received but couldn't publish
	// (ProcessedCount - SentToPublicationCount from ME log lines)
	MEDroppedCount float64 = 0

	// --- Overall (component-level) metrics ---
	// These represent the full pipeline: input = otelcol receiver, output = ME publication

	// overallReceivedMetric is the pipeline input rate (what otelcol's receiver accepted per minute)
	overallReceivedMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "overall_metrics_received_per_minute",
			Help: "Rate of metric points entering the collection pipeline (per minute)",
		},
		[]string{"computer", "release", "controller_type"},
	)

	// overallSentMetric is the pipeline output rate (what ME published to Azure Monitor per minute)
	overallSentMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "overall_metrics_sent_per_minute",
			Help: "Rate of metric points delivered to Azure Monitor (per minute)",
		},
		[]string{"computer", "release", "controller_type"},
	)

	// overallBytesSentMetric is the pipeline output byte rate
	overallBytesSentMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "overall_bytes_sent_per_minute",
			Help: "Bytes of metric data delivered to Azure Monitor (per minute)",
		},
		[]string{"computer", "release", "controller_type"},
	)

	// overallDroppedMetric is the sum of all stage drops (otelcol + ME)
	overallDroppedMetric = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "overall_metrics_dropped_total",
			Help: "Total metric points dropped across all pipeline stages",
		},
		[]string{"computer", "release", "controller_type"},
	)

	// --- ME (sub-component) metrics ---

	// meReceivedMetric is what ME received (EventsProcessedLastPeriod)
	meReceivedMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "me_metrics_received_per_minute",
			Help: "Rate of metric points received by ME (per minute)",
		},
		[]string{"computer", "release", "controller_type"},
	)

	// meSentMetric is what ME published (SentToPublicationCount) — same as overall output
	meSentMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "me_metrics_sent_per_minute",
			Help: "Rate of metric points published by ME to Azure Monitor (per minute)",
		},
		[]string{"computer", "release", "controller_type"},
	)

	// meDroppedMetric counts metric points ME received but couldn't publish
	meDroppedMetric = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "me_metrics_dropped_total",
			Help: "Total metric points ME received but failed to publish to Azure Monitor",
		},
		[]string{"computer", "release", "controller_type"},
	)

	// invalidSettingsConfigMetric indicates whether the metrics settings configmap is invalid (1) or valid (0)
	invalidSettingsConfigMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "invalid_metrics_settings_config",
			Help: "Whether the ama-metrics-settings-configmap is invalid (1) or valid (0)",
		},
		[]string{"computer", "release", "controller_type", "error"},
	)

	// --- OtelCol (sub-component) metrics ---

	// otelcolReceivedRateMetric is what otelcol's receiver accepted per minute (= overall input)
	otelcolReceivedRateMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "otelcol_metrics_received_per_minute",
			Help: "Rate of metric points accepted by otelcollector receiver (per minute)",
		},
		[]string{"computer", "release", "controller_type"},
	)

	// otelcolSentRateMetric is what otelcol's exporter sent to ME per minute
	otelcolSentRateMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "otelcol_metrics_sent_per_minute",
			Help: "Rate of metric points sent by otelcollector exporter to ME (per minute)",
		},
		[]string{"computer", "release", "controller_type"},
	)

	// otelcolDroppedMetric counts metric points that otelcol failed to send to ME
	otelcolDroppedMetric = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "otelcol_metrics_dropped_total",
			Help: "Total metric points otelcollector failed to export to ME",
		},
		[]string{"computer", "release", "controller_type"},
	)

	// otelcolExportFailuresMetric counts the number of "Exporting failed" log events from otelcol
	otelcolExportFailuresMetric = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "otelcol_export_failures_total",
			Help: "Count of otelcollector export failure log events",
		},
		[]string{"computer", "release", "controller_type"},
	)
)

const (
	prometheusCollectorHealthInterval = 60
	prometheusCollectorHealthPort     = ":2234"
)

// ExposePrometheusCollectorHealthMetrics exposes Prometheus metrics about the health of the agent
// This can be called from both CCP mode (main.go) and non-CCP mode (fluent-bit plugin)
func ExposePrometheusCollectorHealthMetrics() {
	// Get common properties from environment variables
	computer := GetEnv("NODE_NAME", "")
	helmReleaseName := GetEnv("HELM_RELEASE_NAME", "")
	controllerType := GetEnv("CONTROLLER_TYPE", "")

	// A new registry excludes go_* and promhttp_* metrics for the endpoint
	r := prometheus.NewRegistry()
	// Overall (component-level) metrics
	r.MustRegister(overallReceivedMetric)
	r.MustRegister(overallSentMetric)
	r.MustRegister(overallBytesSentMetric)
	r.MustRegister(overallDroppedMetric)
	// ME (sub-component) metrics
	r.MustRegister(meReceivedMetric)
	r.MustRegister(meSentMetric)
	r.MustRegister(meDroppedMetric)
	// OtelCol (sub-component) metrics
	r.MustRegister(otelcolReceivedRateMetric)
	r.MustRegister(otelcolSentRateMetric)
	r.MustRegister(otelcolDroppedMetric)
	r.MustRegister(otelcolExportFailuresMetric)
	// Config validation
	r.MustRegister(invalidSettingsConfigMetric)

	handler := promhttp.HandlerFor(r, promhttp.HandlerOpts{})
	http.Handle("/metrics", handler)

	go func() {
		TimeseriesVolumeTicker = time.NewTicker(time.Second * time.Duration(prometheusCollectorHealthInterval))
		lastTickerStart := time.Now()

		for ; true; <-TimeseriesVolumeTicker.C {
			elapsed := time.Since(lastTickerStart)
			timePassedInMinutes := (float64(elapsed) / float64(time.Second)) / float64(prometheusCollectorHealthInterval)

			TimeseriesVolumeMutex.Lock()
			timeseriesReceivedRate := math.Round(TimeseriesReceivedTotal / timePassedInMinutes)
			timeseriesSentRate := math.Round(TimeseriesSentTotal / timePassedInMinutes)
			bytesSentRate := math.Round(BytesSentTotal / timePassedInMinutes)

			// ME sub-component metrics (ME receives from otelcol, sends to Azure Monitor)
			meReceivedMetric.With(prometheus.Labels{"computer": computer, "release": helmReleaseName, "controller_type": controllerType}).Set(timeseriesReceivedRate)
			meSentMetric.With(prometheus.Labels{"computer": computer, "release": helmReleaseName, "controller_type": controllerType}).Set(timeseriesSentRate)

			// Overall metrics: input = otelcol receiver, output = ME publication
			overallSentMetric.With(prometheus.Labels{"computer": computer, "release": helmReleaseName, "controller_type": controllerType}).Set(timeseriesSentRate)
			overallBytesSentMetric.With(prometheus.Labels{"computer": computer, "release": helmReleaseName, "controller_type": controllerType}).Set(bytesSentRate)

			TimeseriesReceivedTotal = 0.0
			TimeseriesSentTotal = 0.0
			BytesSentTotal = 0.0
			TimeseriesVolumeMutex.Unlock()

			isInvalidSettingsConfig := 0
			settingsConfigErrorString := ""
			if os.Getenv("AZMON_INVALID_METRICS_SETTINGS_CONFIG") == "true" {
				isInvalidSettingsConfig = 1
				settingsConfigErrorString = os.Getenv("INVALID_SETTINGS_CONFIG_ERROR")
			}
			invalidSettingsConfigMetric.With(prometheus.Labels{"computer": computer, "release": helmReleaseName, "controller_type": controllerType, "error": settingsConfigErrorString}).Set(float64(isInvalidSettingsConfig))

			MEDroppedMutex.Lock()
			meDroppedMetric.With(prometheus.Labels{"computer": computer, "release": helmReleaseName, "controller_type": controllerType}).Add(MEDroppedCount)
			overallDroppedMetric.With(prometheus.Labels{"computer": computer, "release": helmReleaseName, "controller_type": controllerType}).Add(MEDroppedCount)
			MEDroppedCount = 0
			MEDroppedMutex.Unlock()

			OtelColExportingFailedMutex.Lock()
			otelcolExportFailuresMetric.With(prometheus.Labels{"computer": computer, "release": helmReleaseName, "controller_type": controllerType}).Add(float64(OtelColExportFailureEventCount))
			OtelColExportFailureEventCount = 0
			OtelColExportingFailedMutex.Unlock()

			// Update otelcol sub-component metrics from otelcol scraper
			OtelColDiagMutex.Lock()
			otelcolReceivedRateMetric.With(prometheus.Labels{"computer": computer, "release": helmReleaseName, "controller_type": controllerType}).Set(OtelColReceivedRate)
			otelcolSentRateMetric.With(prometheus.Labels{"computer": computer, "release": helmReleaseName, "controller_type": controllerType}).Set(OtelColSentRate)
			otelcolDroppedMetric.With(prometheus.Labels{"computer": computer, "release": helmReleaseName, "controller_type": controllerType}).Add(OtelColDroppedCount)
			overallDroppedMetric.With(prometheus.Labels{"computer": computer, "release": helmReleaseName, "controller_type": controllerType}).Add(OtelColDroppedCount)
			// Overall input = otelcol receiver rate
			overallReceivedMetric.With(prometheus.Labels{"computer": computer, "release": helmReleaseName, "controller_type": controllerType}).Set(OtelColReceivedRate)
			OtelColDroppedCount = 0
			OtelColDiagMutex.Unlock()

			lastTickerStart = time.Now()
		}
	}()

	log.Printf("Starting Prometheus Collector Health metrics endpoint on %s\n", prometheusCollectorHealthPort)
	err := http.ListenAndServe(prometheusCollectorHealthPort, nil)
	if err != nil {
		log.Printf("Error for Prometheus Collector Health endpoint: %s\n", err.Error())
	}
}
