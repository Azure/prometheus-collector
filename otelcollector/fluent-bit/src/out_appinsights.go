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
)

//export FLBPluginRegister
func FLBPluginRegister(ctx unsafe.Pointer) int {
	return output.FLBPluginRegister(ctx, "appinsights", "AppInsights GO!")
}

//export FLBPluginInit
// (fluentbit will call this)
// ctx (context) pointer to fluentbit context (state/ c code)
func FLBPluginInit(ctx unsafe.Pointer) int {
	Log("Initializing out_appinsights go plugin for fluentbit")
	var agentVersion string
	agentVersion = os.Getenv("AGENT_VERSION")

	InitializePlugin(agentVersion)

	// Run a go routine that hosts Prometheus metrics for the volume of timeseries scraped and sent
	// These numbers are picked up from the ME logs in the fluent-bit pipeline
	if strings.ToLower(os.Getenv(envPrometheusCollectorHealth)) == "true" {
		go PublishTimeseriesVolume()
	}

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


	// Metrics Extension logs with metrics received, dropped, and processed counts
	switch incomingTag {
	case fluentbitEventsProcessedLastPeriodTag:
		return PushReceivedMetricsCountToAppInsightsMetrics(records)
	case fluentbitProcessedCountTag:
		return PushProcessedCountToAppInsightsMetrics(records)
	case fluentbitDiagnosticHeartbeatTag:
		return PushMetricsDroppedCountToAppInsightsMetrics(records)
	case fluentbitInfiniteMetricTag:
		return PushInfiniteMetricLogToAppInsightsEvents(records)
	case fluentbitExportingFailedTag:
		return PushExportingFailedLogToAppInsightsEvents(records)
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
