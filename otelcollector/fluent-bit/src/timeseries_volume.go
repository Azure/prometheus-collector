package main

import (
	"math"
	"net/http"
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
)

const (
	timeseriesVolumeInterval = 60
	timeseriesVolumePort = ":2234"
)

func PublishTimeseriesVolume() {

	// A new registry prevents go_* and promhttp_* metrics from sending to the endpoint
	r := prometheus.NewRegistry()
	r.MustRegister(timeseriesReceivedMetric)
	r.MustRegister(timeseriesSentMetric)
	r.MustRegister(bytesSentMetric)

	handler := promhttp.HandlerFor(r, promhttp.HandlerOpts{})
	http.Handle("/metrics", handler)

	go func() {
		TimeseriesVolumeTicker = time.NewTicker(time.Second * time.Duration(timeseriesVolumeInterval))
		start := time.Now()
		
		for ; true; <-TimeseriesVolumeTicker.C {
			elapsed := time.Since(start)
		
			timePassedInMinutes := (float64(elapsed) / float64(time.Second)) / float64(timeseriesVolumeInterval)

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
		
			start = time.Now()
		}
	}()

	http.ListenAndServe(timeseriesVolumePort, nil)
}
