package shared

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// otelColMetricsURL is the internal metrics endpoint exposed by the otelcollector process
	otelColMetricsURL = "http://127.0.0.1:8888/metrics"

	// otelColScrapeInterval is how often we scrape the otelcollector metrics
	otelColScrapeInterval = 15 * time.Second

	// otelColStartupDelay is how long to wait before first scrape to let otelcollector start
	otelColStartupDelay = 30 * time.Second

	// Metric names from otelcollector's internal Prometheus endpoint
	otelColReceivedMetric   = "otelcol_receiver_accepted_metric_points"
	otelColSentMetric       = "otelcol_exporter_sent_metric_points"
	otelColSendFailedMetric = "otelcol_exporter_send_failed_metric_points"
)

var (
	// OtelColDiagMutex protects the diagnostic metric values
	OtelColDiagMutex sync.Mutex

	// OtelColReceivedRate is the per-minute rate of metric points accepted by otelcollector's receiver
	OtelColReceivedRate float64

	// OtelColSentRate is the per-minute rate of metric points sent by otelcollector's exporter (to ME)
	OtelColSentRate float64

	// OtelColSendFailedTotal is the cumulative count of metric points that failed to export from otelcol to ME
	OtelColSendFailedTotal float64
)

// ScrapeOtelCollectorHealthMetrics periodically scrapes the otelcollector's
// internal metrics endpoint to compute diagnostic rates for CCP mode.
//
// These metrics help diagnose failures between otelcollector and ME:
//   - otelcol_receiver_accepted_metric_points: what otelcol received from scrape targets
//   - otelcol_exporter_sent_metric_points: what otelcol handed to ME
//   - otelcol_exporter_send_failed_metric_points: what otelcol failed to hand to ME
//
// The primary health metrics (timeseries_received/sent_per_minute) come from ME log parsing
// via TailMELogs, which gives true end-to-end confirmation that ME accepted and published data.
// These otelcol metrics are supplementary diagnostics exposed as separate gauges on port 2234.
func ScrapeOtelCollectorHealthMetrics() {
	log.Printf("Waiting %v for otelcollector to start before scraping diagnostic metrics", otelColStartupDelay)
	time.Sleep(otelColStartupDelay)

	var prevReceived float64
	var prevSent float64
	var prevSendFailed float64
	var initialized bool

	ticker := time.NewTicker(otelColScrapeInterval)
	defer ticker.Stop()

	log.Printf("Starting otelcollector diagnostic metrics scraper (interval=%v, url=%s)", otelColScrapeInterval, otelColMetricsURL)

	for range ticker.C {
		metrics, err := scrapeOtelColMetrics(otelColMetricsURL)
		if err != nil {
			log.Printf("Failed to scrape otelcollector metrics from %s: %v", otelColMetricsURL, err)
			continue
		}

		currentReceived := metrics[otelColReceivedMetric]
		currentSent := metrics[otelColSentMetric]
		currentSendFailed := metrics[otelColSendFailedMetric]

		if initialized {
			deltaReceived := currentReceived - prevReceived
			deltaSent := currentSent - prevSent
			deltaSendFailed := currentSendFailed - prevSendFailed

			// Handle counter resets (e.g. otelcollector restart)
			if deltaReceived < 0 {
				deltaReceived = currentReceived
			}
			if deltaSent < 0 {
				deltaSent = currentSent
			}
			if deltaSendFailed < 0 {
				deltaSendFailed = currentSendFailed
			}

			// Compute per-minute rates from 15s interval deltas
			minuteScale := 60.0 / otelColScrapeInterval.Seconds()

			OtelColDiagMutex.Lock()
			OtelColReceivedRate = deltaReceived * minuteScale
			OtelColSentRate = deltaSent * minuteScale
			OtelColSendFailedTotal += deltaSendFailed
			OtelColDiagMutex.Unlock()
		}

		prevReceived = currentReceived
		prevSent = currentSent
		prevSendFailed = currentSendFailed
		initialized = true
	}
}

// scrapeOtelColMetrics fetches Prometheus text-format metrics from the given URL
// and returns a map of metric name -> summed value (summed across all label combinations).
func scrapeOtelColMetrics(url string) (map[string]float64, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP GET failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	result := make(map[string]float64)
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 || line[0] == '#' {
			continue
		}

		name, value, ok := parsePrometheusLine(line)
		if !ok {
			continue
		}

		// Sum across all label combinations for each metric name
		result[name] += value
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	return result, nil
}

// parsePrometheusLine parses a single Prometheus text-format line like:
//
//	metric_name{label="value"} 123.45
//	metric_name 123.45
//
// Returns the metric name (without labels), the float value, and whether parsing succeeded.
func parsePrometheusLine(line string) (string, float64, bool) {
	// Find the metric name - it's everything before '{' or the first space
	nameEnd := strings.IndexByte(line, '{')
	if nameEnd < 0 {
		nameEnd = strings.IndexByte(line, ' ')
	}
	if nameEnd <= 0 {
		return "", 0, false
	}
	name := line[:nameEnd]

	// Find the value - it's the last whitespace-separated token
	lastSpace := strings.LastIndexByte(line, ' ')
	if lastSpace < 0 || lastSpace >= len(line)-1 {
		return "", 0, false
	}
	valueStr := line[lastSpace+1:]

	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return "", 0, false
	}

	return name, value, true
}
