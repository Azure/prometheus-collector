package main

import (
	"github.com/fluent/fluent-bit-go/output"
	"github.com/microsoft/ApplicationInsights-Go/appinsights"
)
import (
	"C"
	"os"
	"strings"
	"unsafe"
)

//export FLBPluginRegister
func FLBPluginRegister(ctx unsafe.Pointer) int {
	return output.FLBPluginRegister(ctx, "appinsights", "AppInsights GO!")
}

// (fluentbit will call this)
// ctx (context) pointer to fluentbit context (state/ c code)
//
//export FLBPluginInit
func FLBPluginInit(ctx unsafe.Pointer) int {

	// This will not load the plugin instance. FLBPluginFlush won't be called.
	if os.Getenv("TELEMETRY_DISABLED") == "true" {
		Log("Telemetry disabled. Not initializing telemetry plugin.")
		return output.FLB_ERROR
	}

	Log("Initializing out_appinsights go plugin for fluentbit")
	var agentVersion string
	agentVersion = os.Getenv("AGENT_VERSION")

	InitializePlugin(agentVersion)

	// Run a go routine that hosts Prometheus metrics for the health of the agent
	// Volume numbers are picked up from the ME logs in the fluent-bit pipeline
	// Other metrics are from environment variables and otelcollector logs
	if strings.ToLower(os.Getenv(envPrometheusCollectorHealth)) == "true" {
		go ExposePrometheusCollectorHealthMetrics()
	}
	if strings.ToLower(os.Getenv(envControllerType)) == "replicaset" {
		go SendCoreCountToAppInsightsMetrics()
	}

	if strings.ToLower(os.Getenv(envControllerType)) == "daemonset" {
		go SendContainersCpuMemoryToAppInsightsMetrics()
	}

	// Collect, aggregate, and send CPU and Memory usage telemetry for the processes below
	processAggregations := InitProcessAggregations([]string{"otelcollector", "MetricsExtension", "fluent-bit", "mdsd", "telegraf"})
	processAggregations.Run()

	go PushMEProcessedAndReceivedCountToAppInsightsMetrics()

	return output.FLB_OK
}

//export FLBPluginFlush
func FLBPluginFlush(data unsafe.Pointer, length C.int, tag *C.char) int {
	var ret int
	var record map[interface{}]interface{}
	var records []map[interface{}]interface{}

	// Create Fluent Bit decoder
	dec := NewDecoder(data, int(length))

	// Iterate Records
	for {
		// Extract Record
		ret, _, record = GetRecord(dec)
		if ret != 0 {
			break
		}
		records = append(records, record)
	}

	incomingTag := strings.ToLower(C.GoString(tag))

	// Metrics Extension logs with metrics received, dropped, and processed counts
	switch incomingTag {
	case fluentbitEventsProcessedLastPeriodTag:
		return UpdateMEReceivedMetricsCount(records)
	case fluentbitProcessedCountTag:
		return UpdateMEMetricsProcessedCount(records)
	case fluentbitDiagnosticHeartbeatTag:
		return PushMetricsDroppedCountToAppInsightsMetrics(records)
	case fluentbitInfiniteMetricTag:
		return PushInfiniteMetricLogToAppInsightsEvents(records)
	case fluentbitExportingFailedTag:
		return RecordExportingFailed(records)
	case "prometheus.metrics.otelcollector", "prometheus.metrics.prometheus", "prometheus.metrics.targetallocator":
		return SendPrometheusMetricsToAppInsights(records)
	default:
		// Error messages from metrics extension and otelcollector
		return PushLogErrorsToAppInsightsTraces(records, appinsights.Information, incomingTag)
	}
}

// FLBPluginExit exits the plugin
func FLBPluginExit() int {
	return output.FLB_OK
}

func main() {
}
