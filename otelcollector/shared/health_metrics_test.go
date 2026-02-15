package shared

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func TestHealthMetricsRegistration(t *testing.T) {
	// Test that all metrics can be registered without panic
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Metric registration panicked: %v", r)
		}
	}()

	// Create a new registry
	registry := prometheus.NewRegistry()
	
	// Register all health metrics
	registry.MustRegister(timeseriesReceivedMetric)
	registry.MustRegister(timeseriesSentMetric)
	registry.MustRegister(bytesSentMetric)
	registry.MustRegister(invalidCustomConfigMetric)
	registry.MustRegister(exportingFailedMetric)

	// If we get here, registration was successful
	t.Log("All health metrics registered successfully")
}

func TestHealthMetricsEndpoint(t *testing.T) {
	// Create a test HTTP server
	registry := prometheus.NewRegistry()
	
	// Create new instances of the metrics for testing
	testTimeseriesReceived := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "timeseries_received_per_minute",
			Help: "Number of timeseries to be sent to storage",
		},
		[]string{"computer", "release", "controller_type"},
	)
	testTimeseriesSent := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "timeseries_sent_per_minute",
			Help: "Number of timeseries sent to storage",
		},
		[]string{"computer", "release", "controller_type"},
	)
	testBytesSent := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bytes_sent_per_minute",
			Help: "Number of bytes of timeseries sent to storage",
		},
		[]string{"computer", "release", "controller_type"},
	)
	testInvalidConfig := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "invalid_custom_prometheus_config",
			Help: "If an invalid custom prometheus config was given or not",
		},
		[]string{"computer", "release", "controller_type", "error"},
	)
	testExportingFailed := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "exporting_metrics_failed",
			Help: "If exporting metrics failed or not",
		},
		[]string{"computer", "release", "controller_type"},
	)
	
	registry.MustRegister(testTimeseriesReceived)
	registry.MustRegister(testTimeseriesSent)
	registry.MustRegister(testBytesSent)
	registry.MustRegister(testInvalidConfig)
	registry.MustRegister(testExportingFailed)

	// Set some test values
	testTimeseriesReceived.With(prometheus.Labels{"computer": "test", "release": "test", "controller_type": "test"}).Set(100)
	testTimeseriesSent.With(prometheus.Labels{"computer": "test", "release": "test", "controller_type": "test"}).Set(100)
	testBytesSent.With(prometheus.Labels{"computer": "test", "release": "test", "controller_type": "test"}).Set(1000)
	testInvalidConfig.With(prometheus.Labels{"computer": "test", "release": "test", "controller_type": "test", "error": ""}).Set(0)
	testExportingFailed.With(prometheus.Labels{"computer": "test", "release": "test", "controller_type": "test"}).Add(0)

	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	
	// Create a test request
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	
	// Serve the request
	handler.ServeHTTP(w, req)
	
	// Check the response
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}
	
	body := w.Body.String()
	
	// Verify all metrics are in the response
	expectedMetrics := []string{
		"timeseries_received_per_minute",
		"timeseries_sent_per_minute",
		"bytes_sent_per_minute",
		"invalid_custom_prometheus_config",
		"exporting_metrics_failed",
	}
	
	for _, metric := range expectedMetrics {
		if !strings.Contains(body, metric) {
			t.Errorf("Expected metric %s not found in response", metric)
		}
	}
}

func TestHealthMetricsLabels(t *testing.T) {
	// Save original environment variables
	origNodeName := os.Getenv("NODE_NAME")
	origHelmRelease := os.Getenv("HELM_RELEASE_NAME")
	origControllerType := os.Getenv("CONTROLLER_TYPE")
	defer func() {
		os.Setenv("NODE_NAME", origNodeName)
		os.Setenv("HELM_RELEASE_NAME", origHelmRelease)
		os.Setenv("CONTROLLER_TYPE", origControllerType)
	}()

	tests := []struct {
		name           string
		nodeName       string
		helmRelease    string
		controllerType string
	}{
		{
			name:           "ReplicaSet with all labels",
			nodeName:       "node-1",
			helmRelease:    "release-1",
			controllerType: "ReplicaSet",
		},
		{
			name:           "DaemonSet with all labels",
			nodeName:       "node-2",
			helmRelease:    "release-2",
			controllerType: "DaemonSet",
		},
		{
			name:           "Empty labels",
			nodeName:       "",
			helmRelease:    "",
			controllerType: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			os.Setenv("NODE_NAME", tc.nodeName)
			os.Setenv("HELM_RELEASE_NAME", tc.helmRelease)
			os.Setenv("CONTROLLER_TYPE", tc.controllerType)

			// Create a new registry for this test
			r := prometheus.NewRegistry()
			
			// Create new metrics with labels
			testMetric := prometheus.NewGaugeVec(
				prometheus.GaugeOpts{
					Name: "test_metric",
					Help: "Test metric",
				},
				[]string{"computer", "release", "controller_type"},
			)
			r.MustRegister(testMetric)

			// Set the metric with the environment variable values
			computer := GetEnv("NODE_NAME", "")
			helmReleaseName := GetEnv("HELM_RELEASE_NAME", "")
			controllerType := GetEnv("CONTROLLER_TYPE", "")

			testMetric.With(prometheus.Labels{
				"computer":        computer,
				"release":         helmReleaseName,
				"controller_type": controllerType,
			}).Set(123.0)

			// Gather metrics
			metricFamilies, err := r.Gather()
			if err != nil {
				t.Fatalf("Failed to gather metrics: %v", err)
			}

			if len(metricFamilies) == 0 {
				t.Fatal("No metrics were gathered")
			}

			// Verify labels are present
			found := false
			for _, mf := range metricFamilies {
				if mf.GetName() == "test_metric" {
					found = true
					for _, m := range mf.GetMetric() {
						labels := make(map[string]string)
						for _, label := range m.GetLabel() {
							labels[label.GetName()] = label.GetValue()
						}
						
						if labels["computer"] != tc.nodeName {
							t.Errorf("Expected computer=%s, got %s", tc.nodeName, labels["computer"])
						}
						if labels["release"] != tc.helmRelease {
							t.Errorf("Expected release=%s, got %s", tc.helmRelease, labels["release"])
						}
						if labels["controller_type"] != tc.controllerType {
							t.Errorf("Expected controller_type=%s, got %s", tc.controllerType, labels["controller_type"])
						}
					}
				}
			}

			if !found {
				t.Error("test_metric not found in gathered metrics")
			}
		})
	}
}

func TestInvalidCustomConfigMetric(t *testing.T) {
	// Save original environment variables
	origInvalidConfig := os.Getenv("AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG")
	origInvalidError := os.Getenv("INVALID_CONFIG_FATAL_ERROR")
	defer func() {
		os.Setenv("AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG", origInvalidConfig)
		os.Setenv("INVALID_CONFIG_FATAL_ERROR", origInvalidError)
	}()

	tests := []struct {
		name              string
		invalidConfig     string
		invalidError      string
		expectedValue     float64
		expectedErrorText string
	}{
		{
			name:              "Valid config",
			invalidConfig:     "false",
			invalidError:      "",
			expectedValue:     0,
			expectedErrorText: "",
		},
		{
			name:              "Invalid config without error message",
			invalidConfig:     "true",
			invalidError:      "",
			expectedValue:     1,
			expectedErrorText: "",
		},
		{
			name:              "Invalid config with error message",
			invalidConfig:     "true",
			invalidError:      "config parse error",
			expectedValue:     1,
			expectedErrorText: "config parse error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			os.Setenv("AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG", tc.invalidConfig)
			os.Setenv("INVALID_CONFIG_FATAL_ERROR", tc.invalidError)

			isInvalidCustomConfig := 0
			invalidConfigErrorString := ""
			if os.Getenv("AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG") == "true" {
				isInvalidCustomConfig = 1
				invalidConfigErrorString = os.Getenv("INVALID_CONFIG_FATAL_ERROR")
			}

			if float64(isInvalidCustomConfig) != tc.expectedValue {
				t.Errorf("Expected value %f, got %d", tc.expectedValue, isInvalidCustomConfig)
			}

			if invalidConfigErrorString != tc.expectedErrorText {
				t.Errorf("Expected error text %q, got %q", tc.expectedErrorText, invalidConfigErrorString)
			}
		})
	}
}

func TestMetricMutexSafety(t *testing.T) {
	// Save original values
	TimeseriesVolumeMutex.Lock()
	origReceived := TimeseriesReceivedTotal
	origSent := TimeseriesSentTotal
	origBytes := BytesSentTotal
	TimeseriesReceivedTotal = 0
	TimeseriesSentTotal = 0
	BytesSentTotal = 0
	TimeseriesVolumeMutex.Unlock()
	
	defer func() {
		TimeseriesVolumeMutex.Lock()
		TimeseriesReceivedTotal = origReceived
		TimeseriesSentTotal = origSent
		BytesSentTotal = origBytes
		TimeseriesVolumeMutex.Unlock()
	}()

	// Test that concurrent access to metrics is safe
	done := make(chan bool)
	iterations := 100

	// Start multiple goroutines that update the metrics
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < iterations; j++ {
				TimeseriesVolumeMutex.Lock()
				TimeseriesReceivedTotal += 1.0
				TimeseriesSentTotal += 2.0
				BytesSentTotal += 3.0
				TimeseriesVolumeMutex.Unlock()
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 5; i++ {
		<-done
	}

	// Verify the totals
	TimeseriesVolumeMutex.Lock()
	expectedReceived := float64(5 * iterations)
	expectedSent := float64(5 * iterations * 2)
	expectedBytes := float64(5 * iterations * 3)

	if TimeseriesReceivedTotal != expectedReceived {
		t.Errorf("Expected TimeseriesReceivedTotal=%f, got %f", expectedReceived, TimeseriesReceivedTotal)
	}
	if TimeseriesSentTotal != expectedSent {
		t.Errorf("Expected TimeseriesSentTotal=%f, got %f", expectedSent, TimeseriesSentTotal)
	}
	if BytesSentTotal != expectedBytes {
		t.Errorf("Expected BytesSentTotal=%f, got %f", expectedBytes, BytesSentTotal)
	}
	TimeseriesVolumeMutex.Unlock()
}

func TestExportingFailedCounter(t *testing.T) {
	// Save original count
	ExportingFailedMutex.Lock()
	originalCount := OtelCollectorExportingFailedCount
	OtelCollectorExportingFailedCount = 0
	ExportingFailedMutex.Unlock()
	
	defer func() {
		ExportingFailedMutex.Lock()
		OtelCollectorExportingFailedCount = originalCount
		ExportingFailedMutex.Unlock()
	}()

	// Increment the counter
	ExportingFailedMutex.Lock()
	OtelCollectorExportingFailedCount += 5
	currentCount := OtelCollectorExportingFailedCount
	ExportingFailedMutex.Unlock()

	if currentCount != 5 {
		t.Errorf("Expected count=5, got %d", currentCount)
	}

	// Reset the counter (simulating the metric update cycle)
	ExportingFailedMutex.Lock()
	OtelCollectorExportingFailedCount = 0
	finalCount := OtelCollectorExportingFailedCount
	ExportingFailedMutex.Unlock()

	if finalCount != 0 {
		t.Errorf("Expected count=0 after reset, got %d", finalCount)
	}
}

func TestMetricsConstantsAreCorrect(t *testing.T) {
	// Verify the constants are as expected
	expectedInterval := 60
	expectedPort := ":2234"

	if prometheusCollectorHealthInterval != expectedInterval {
		t.Errorf("Expected interval=%d, got %d", expectedInterval, prometheusCollectorHealthInterval)
	}

	if prometheusCollectorHealthPort != expectedPort {
		t.Errorf("Expected port=%s, got %s", expectedPort, prometheusCollectorHealthPort)
	}
}
