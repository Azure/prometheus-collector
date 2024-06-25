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

	go PushMEProcessedAndReceivedCountToAppInsightsMetrics()

	go PushOtelCpuToAppInsightsMetrics()

	go PushMECpuToAppInsightsMetrics()

	// go PushMEMemRssToAppInsightsMetrics()

	// go PushOtelColMemRssToAppInsightsMetrics()

	return output.FLB_OK
}

//export FLBPluginFlush
func FLBPluginFlush(data unsafe.Pointer, length C.int, tag *C.char) int {
	var ret int
	var record map[interface{}]interface{}
	var records []map[interface{}]interface{}

	incomingTag := strings.ToLower(C.GoString(tag))
	Log("Print the incoming tag: %s", incomingTag)
	// Create Fluent Bit decoder
	dec := output.NewDecoder(data, int(length))

	// Iterate Records
	for {
		// Extract Record
		ret, _, record = output.GetRecord(dec)
		if ret != 0 {
			break
		}
		records = append(records, record)
	}

	// Metrics Extension logs with metrics received, dropped, and processed counts
	switch incomingTag {
	case fluentbitEventsProcessedLastPeriodTag:
		Log("Print the entering tag: %s", incomingTag)
		return UpdateMEReceivedMetricsCount(records)
	case fluentbitProcessedCountTag:
		Log("Print the entering tag: %s", incomingTag)
		return UpdateMEMetricsProcessedCount(records)
	case fluentbitDiagnosticHeartbeatTag:
		Log("Print the entering tag: %s", incomingTag)
		return PushMetricsDroppedCountToAppInsightsMetrics(records)
	case fluentbitInfiniteMetricTag:
		Log("Print the entering tag: %s", incomingTag)
		return PushInfiniteMetricLogToAppInsightsEvents(records)
	case fluentbitExportingFailedTag:
		Log("Print the entering tag: %s", incomingTag)
		return RecordExportingFailed(records)
	case otelcolCpuScrapeTag:
		Log("Print the entering tag: %s", incomingTag)
		return UpdateOtelCpuUsages(records)
	case otelcolMemRssScrapeTag:
		Log("Print the entering tag: %s", incomingTag)
		return UpdateOtelColMemRssUsages(records)
	case meMemRssScrapeTag:
		Log("Print the entering tag: %s", incomingTag)
		return UpdateMEMemRssUsages(records)
	case meCpuScrapeTag:
		Log("Print the entering tag: %s", incomingTag)
		return UpdateMECpuUsages(records)
	case promScrapeTag:
		Log("Print the entering tag: %s", incomingTag)
		return PushPromToAppInsightsMetrics(records)
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
