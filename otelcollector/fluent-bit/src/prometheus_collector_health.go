package main

import (
	"math"
	"os"
	"net/http"
	"sync"
	"time"

	"github.com/microsoft/ApplicationInsights-Go/appinsights"
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

	// ExportingFailedMutex handles if the otelcollector has logged that exporting failed
	ExportingFailedMutex = &sync.Mutex{}

	OtelCollectorExportingFailedCount = 0

	// timeseriesReceivedMetric is the Prometheus metric measuring the number of timeseries scraped in a minute
	timeseriesReceivedMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "timeseries_received_per_minute",
			Help: "Number of timeseries to be sent to storage",
		},
		[]string{"computer", "release", "controller_type"},
	)

	// timeseriesSentMetric is the Prometheus metric measuring the number of timeseries scraped in a minute
	timeseriesSentMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "timeseries_sent_per_minute",
			Help: "Number of timeseries sent to storage",
		},
		[]string{"computer", "release", "controller_type"},
	)

	// bytesSentMetric is the Prometheus metric measuring the number of timeseries scraped in a minute
	bytesSentMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bytes_sent_per_minute",
			Help: "Number of bytes of timeseries sent to storage",
		},
		[]string{"computer", "release", "controller_type"},
	)

	// invalidCustomConfigMetric is true if the config provided failed validation and false otherwise
	invalidCustomConfigMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "invalid_custom_prometheus_config",
			Help: "If an invalid custom prometheus config was given or not",
		},
		[]string{"computer", "release", "controller_type", "error"},
	)

	// exportingFailedMetric counts the number of times the otelcollector was unable to export to ME
	exportingFailedMetric = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "exporting_metrics_failed",
			Help: "If exporting metrics failed or not",
		},
		[]string{"computer", "release", "controller_type"},
	)
)

const (
	prometheusCollectorHealthInterval = 60
	prometheusCollectorHealthPort = ":2234"
)


// Expose Prometheus metrics about the health of the agent
func ExposePrometheusCollectorHealthMetrics() {

	// A new registry excludes go_* and promhttp_* metrics for the endpoint
	r := prometheus.NewRegistry()
	r.MustRegister(timeseriesReceivedMetric)
	r.MustRegister(timeseriesSentMetric)
	r.MustRegister(bytesSentMetric)
	r.MustRegister(invalidCustomConfigMetric)
	r.MustRegister(exportingFailedMetric)

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

			timeseriesReceivedMetric.With(prometheus.Labels{"computer":CommonProperties["computer"], "release":CommonProperties["helmreleasename"], "controller_type":CommonProperties["controllertype"]}).Set(timeseriesReceivedRate)
			timeseriesSentMetric.With(prometheus.Labels{"computer":CommonProperties["computer"], "release":CommonProperties["helmreleasename"], "controller_type":CommonProperties["controllertype"]}).Set(timeseriesSentRate)
			bytesSentMetric.With(prometheus.Labels{"computer":CommonProperties["computer"], "release":CommonProperties["helmreleasename"], "controller_type":CommonProperties["controllertype"]}).Set(bytesSentRate)
		
			TimeseriesReceivedTotal = 0.0
			TimeseriesSentTotal = 0.0
			BytesSentTotal = 0.0
			TimeseriesVolumeMutex.Unlock()

			isInvalidCustomConfig := 0
			invalidConfigErrorString := ""
			if os.Getenv("AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG") == "true" {
				isInvalidCustomConfig = 1
				invalidConfigErrorString = os.Getenv("INVALID_CONFIG_FATAL_ERROR")
			}
			invalidCustomConfigMetric.With(prometheus.Labels{"computer":CommonProperties["computer"], "release":CommonProperties["helmreleasename"], "controller_type":CommonProperties["controllertype"], "error":invalidConfigErrorString}).Set(float64(isInvalidCustomConfig))
		
			ExportingFailedMutex.Lock()
			exportingFailedMetric.With(prometheus.Labels{"computer":CommonProperties["computer"], "release":CommonProperties["helmreleasename"], "controller_type":CommonProperties["controllertype"]}).Add(float64(OtelCollectorExportingFailedCount))
			OtelCollectorExportingFailedCount = 0
			ExportingFailedMutex.Unlock()

			lastTickerStart = time.Now()
		}
	}()

	err := http.ListenAndServe(prometheusCollectorHealthPort, nil)
	if err != nil {
		Log("Error for Prometheus Collector Health endpoint: %s", err.Error())
		exception := appinsights.NewExceptionTelemetry(err.Error())
		TelemetryClient.Track(exception)
	}
}
