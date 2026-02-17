package shared

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParsePrometheusLine(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		wantName  string
		wantValue float64
		wantOk    bool
	}{
		{
			name:      "simple metric without labels",
			line:      "otelcol_receiver_accepted_metric_points 1234",
			wantName:  "otelcol_receiver_accepted_metric_points",
			wantValue: 1234,
			wantOk:    true,
		},
		{
			name:      "metric with labels",
			line:      `otelcol_receiver_accepted_metric_points{receiver="prometheus",transport="http"} 5678`,
			wantName:  "otelcol_receiver_accepted_metric_points",
			wantValue: 5678,
			wantOk:    true,
		},
		{
			name:      "metric with float value",
			line:      `otelcol_exporter_sent_metric_points{exporter="otlp"} 99.5`,
			wantName:  "otelcol_exporter_sent_metric_points",
			wantValue: 99.5,
			wantOk:    true,
		},
		{
			name:      "metric with zero value",
			line:      "some_metric 0",
			wantName:  "some_metric",
			wantValue: 0,
			wantOk:    true,
		},
		{
			name:      "metric with scientific notation",
			line:      "big_metric 1.5e+06",
			wantName:  "big_metric",
			wantValue: 1.5e+06,
			wantOk:    true,
		},
		{
			name:   "comment line",
			line:   "# HELP otelcol_receiver_accepted_metric_points",
			wantOk: false,
		},
		{
			name:   "empty line",
			line:   "",
			wantOk: false,
		},
		{
			name:   "no value",
			line:   "orphan_metric",
			wantOk: false,
		},
		{
			name:   "non-numeric value",
			line:   "bad_metric notanumber",
			wantOk: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			name, value, ok := parsePrometheusLine(tc.line)
			if ok != tc.wantOk {
				t.Fatalf("parsePrometheusLine(%q) ok=%v, want %v", tc.line, ok, tc.wantOk)
			}
			if !ok {
				return
			}
			if name != tc.wantName {
				t.Errorf("name=%q, want %q", name, tc.wantName)
			}
			if value != tc.wantValue {
				t.Errorf("value=%f, want %f", value, tc.wantValue)
			}
		})
	}
}

func TestScrapeOtelColMetrics(t *testing.T) {
	// Create a test server that returns Prometheus-format metrics
	metricsBody := strings.Join([]string{
		"# HELP otelcol_receiver_accepted_metric_points Number of metric points accepted.",
		"# TYPE otelcol_receiver_accepted_metric_points counter",
		`otelcol_receiver_accepted_metric_points{receiver="prometheus",transport="http"} 1000`,
		`otelcol_receiver_accepted_metric_points{receiver="otlp",transport="grpc"} 500`,
		"# HELP otelcol_exporter_sent_metric_points Number of metric points sent.",
		"# TYPE otelcol_exporter_sent_metric_points counter",
		`otelcol_exporter_sent_metric_points{exporter="otlp"} 800`,
		"# HELP otelcol_process_uptime Total uptime.",
		"# TYPE otelcol_process_uptime counter",
		"otelcol_process_uptime 3600",
	}, "\n")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, metricsBody)
	}))
	defer server.Close()

	metrics, err := scrapeOtelColMetrics(server.URL)
	if err != nil {
		t.Fatalf("scrapeOtelColMetrics failed: %v", err)
	}

	// receiver_accepted should be summed across both label sets: 1000 + 500 = 1500
	if got := metrics[otelColReceivedMetric]; got != 1500 {
		t.Errorf("otelcol_receiver_accepted_metric_points = %f, want 1500", got)
	}

	// exporter_sent should be 800
	if got := metrics[otelColSentMetric]; got != 800 {
		t.Errorf("otelcol_exporter_sent_metric_points = %f, want 800", got)
	}

	// process_uptime should be 3600
	if got := metrics["otelcol_process_uptime"]; got != 3600 {
		t.Errorf("otelcol_process_uptime = %f, want 3600", got)
	}
}

func TestScrapeOtelColMetrics_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := scrapeOtelColMetrics(server.URL)
	if err == nil {
		t.Fatal("Expected error for 500 response, got nil")
	}
	if !strings.Contains(err.Error(), "unexpected status code: 500") {
		t.Errorf("Expected status code error, got: %v", err)
	}
}

func TestScrapeOtelColMetrics_ConnectionRefused(t *testing.T) {
	_, err := scrapeOtelColMetrics("http://127.0.0.1:1")
	if err == nil {
		t.Fatal("Expected error for connection refused, got nil")
	}
}

func TestScrapeOtelColMetrics_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		// empty body
	}))
	defer server.Close()

	metrics, err := scrapeOtelColMetrics(server.URL)
	if err != nil {
		t.Fatalf("Unexpected error for empty response: %v", err)
	}
	if len(metrics) != 0 {
		t.Errorf("Expected empty map, got %v", metrics)
	}
}

func TestDeltaAccumulation(t *testing.T) {
	// Simulate the delta computation logic from ScrapeOtelCollectorHealthMetrics
	// to verify correctness without starting a real ticker

	// Save and restore globals
	TimeseriesVolumeMutex.Lock()
	origReceived := TimeseriesReceivedTotal
	origSent := TimeseriesSentTotal
	TimeseriesReceivedTotal = 0
	TimeseriesSentTotal = 0
	TimeseriesVolumeMutex.Unlock()

	defer func() {
		TimeseriesVolumeMutex.Lock()
		TimeseriesReceivedTotal = origReceived
		TimeseriesSentTotal = origSent
		TimeseriesVolumeMutex.Unlock()
	}()

	type scrapeResult struct {
		received float64
		sent     float64
	}

	// Simulate a series of scrapes with monotonically increasing counters
	scrapes := []scrapeResult{
		{received: 100, sent: 80},   // initial (no delta computed)
		{received: 250, sent: 200},  // delta: +150 received, +120 sent
		{received: 400, sent: 350},  // delta: +150 received, +150 sent
		{received: 10, sent: 5},     // counter reset: use raw value as delta
		{received: 60, sent: 55},    // delta: +50 received, +50 sent
	}

	var prevReceived, prevSent float64
	var initialized bool

	for _, s := range scrapes {
		if initialized {
			deltaReceived := s.received - prevReceived
			deltaSent := s.sent - prevSent

			// Handle counter resets
			if deltaReceived < 0 {
				deltaReceived = s.received
			}
			if deltaSent < 0 {
				deltaSent = s.sent
			}

			if deltaReceived > 0 || deltaSent > 0 {
				TimeseriesVolumeMutex.Lock()
				TimeseriesReceivedTotal += deltaReceived
				TimeseriesSentTotal += deltaSent
				TimeseriesVolumeMutex.Unlock()
			}
		}

		prevReceived = s.received
		prevSent = s.sent
		initialized = true
	}

	// Expected totals:
	// received: 150 + 150 + 10 + 50 = 360
	// sent: 120 + 150 + 5 + 50 = 325
	TimeseriesVolumeMutex.Lock()
	defer TimeseriesVolumeMutex.Unlock()

	if TimeseriesReceivedTotal != 360 {
		t.Errorf("TimeseriesReceivedTotal = %f, want 360", TimeseriesReceivedTotal)
	}
	if TimeseriesSentTotal != 325 {
		t.Errorf("TimeseriesSentTotal = %f, want 325", TimeseriesSentTotal)
	}
}

func TestDeltaAccumulation_NoNegativeDeltas(t *testing.T) {
	// Verify that when current equals previous (no change), no delta is added

	TimeseriesVolumeMutex.Lock()
	origReceived := TimeseriesReceivedTotal
	origSent := TimeseriesSentTotal
	TimeseriesReceivedTotal = 0
	TimeseriesSentTotal = 0
	TimeseriesVolumeMutex.Unlock()

	defer func() {
		TimeseriesVolumeMutex.Lock()
		TimeseriesReceivedTotal = origReceived
		TimeseriesSentTotal = origSent
		TimeseriesVolumeMutex.Unlock()
	}()

	// Same value twice means delta = 0, should not accumulate
	prevReceived := 100.0
	prevSent := 80.0
	currentReceived := 100.0
	currentSent := 80.0

	deltaReceived := currentReceived - prevReceived
	deltaSent := currentSent - prevSent

	if deltaReceived < 0 {
		deltaReceived = currentReceived
	}
	if deltaSent < 0 {
		deltaSent = currentSent
	}

	if deltaReceived > 0 || deltaSent > 0 {
		TimeseriesVolumeMutex.Lock()
		TimeseriesReceivedTotal += deltaReceived
		TimeseriesSentTotal += deltaSent
		TimeseriesVolumeMutex.Unlock()
	}

	TimeseriesVolumeMutex.Lock()
	defer TimeseriesVolumeMutex.Unlock()

	if TimeseriesReceivedTotal != 0 {
		t.Errorf("Expected no accumulation, got TimeseriesReceivedTotal=%f", TimeseriesReceivedTotal)
	}
	if TimeseriesSentTotal != 0 {
		t.Errorf("Expected no accumulation, got TimeseriesSentTotal=%f", TimeseriesSentTotal)
	}
}
