package main

import (
	"github.com/microsoft/ApplicationInsights-Go/appinsights"
	"github.com/fluent/fluent-bit-go/output"
)
import (
	"C"
	"os"
	"strings"
	"unsafe"
	"fmt"
)

//export FLBPluginRegister
func FLBPluginRegister(ctx unsafe.Pointer) int {
	return output.FLBPluginRegister(ctx, "appinsights", "AppInsights GO!")
}

//export FLBPluginInit
// (fluentbit will call this)
// ctx (context) pointer to fluentbit context (state/ c code)
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
	go SendCoreCountToAppInsightsMetrics()

	return output.FLB_OK
}

//export FLBPluginFlush
func FLBPluginFlush(data unsafe.Pointer, length C.int, tag *C.char) int {
	var ret int
	var record map[interface{}]interface{}
	var records []map[interface{}]interface{}

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

	incomingTag := strings.ToLower(C.GoString(tag))
  Log(fmt.Sprintf("Incoming tag: %s", incomingTag))

	// Metrics Extension logs with metrics received, dropped, and processed counts
	switch incomingTag {
	case "prometheus.log.prometheus":
		Log("Incoming tag is prometheus.log.prometheus")
		for k, v := range record {
			Log(fmt.Sprintf("\"%s\": %v, ", k, v))
		}
	case fluentbitEventsProcessedLastPeriodTag:
		return PushReceivedMetricsCountToAppInsightsMetrics(records)
	case fluentbitProcessedCountTag:
		return PushProcessedCountToAppInsightsMetrics(records)
	case fluentbitDiagnosticHeartbeatTag:
		return PushMetricsDroppedCountToAppInsightsMetrics(records)
	case fluentbitInfiniteMetricTag:
		return PushInfiniteMetricLogToAppInsightsEvents(records)
	case fluentbitExportingFailedTag:
		return RecordExportingFailed(records)
	default:
		// Error messages from metrics extension and otelcollector
		return PushLogErrorsToAppInsightsTraces(records, appinsights.Information, incomingTag)
	}

	return output.FLB_OK
}

// FLBPluginExit exits the plugin
func FLBPluginExit() int {
	return output.FLB_OK
}

func main() {
}
