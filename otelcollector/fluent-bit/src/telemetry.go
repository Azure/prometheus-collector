package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/microsoft/ApplicationInsights-Go/appinsights"
	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
	"github.com/fluent/fluent-bit-go/output"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// CommonProperties indicates the dimensions that are sent with every event/metric
	CommonProperties map[string]string
	// TelemetryClient is the client used to send the telemetry
	TelemetryClient appinsights.TelemetryClient
	metricsReceivedTotal float64 = 0
	metricsSentTotal float64 = 0
	bytesSentTotal float64 = 0
	TimeseriesVolumeTicker *time.Ticker
	TimeseriesVolumeMutex = &sync.Mutex{}
	timeseriesReceivedRate = 0.0
	timeseriesSentRate = 0.0
	bytesSentRate = 0.0
)

var (
	timeseriesReceivedMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "timeseries_received_per_minute",
			Help: "Number of timeseries to be sent to storage",
		},
		[]string{"computer", "release", "controller_type"},
	)
	timeseriesSentMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "timeseries_sent_per_minute",
			Help: "Number of timeseries sent to storage",
		},
		[]string{"computer", "release", "controller_type"},
	)
	bytesSentMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bytes_sent_per_minute",
			Help: "Number of bytes of timeseries sent to storage",
		},
		[]string{"computer", "release", "controller_type"},
	)
)

const (
	clusterTypeACS                                    = "ACS"
	clusterTypeAKS                                    = "AKS"
	envAKSResourceID                                  = "AKS_RESOURCE_ID"
	envACSResourceName                                = "ACS_RESOURCE_NAME"
	envAgentVersion                                   = "AGENT_VERSION"
	envControllerType                                 = "CONTROLLER_TYPE"
	envNodeIP                                         = "NODE_IP"
	envMode                                           = "MODE"
	envCluster                                        = "customResourceId"
	envAppInsightsAuth                                = "APPLICATIONINSIGHTS_AUTH"
	envAppInsightsEndpoint                            = "APPLICATIONINSIGHTS_ENDPOINT"
	envComputerName                                   = "NODE_NAME"
	envDefaultMetricAccountName                       = "AZMON_DEFAULT_METRIC_ACCOUNT_NAME"
	envPodName                                        = "POD_NAME"
	envTelemetryOffSwitch                             = "DISABLE_TELEMETRY"
	envNamespace                                      = "POD_NAMESPACE"
	envHelmReleaseName                                = "HELM_RELEASE_NAME"
	fluentbitOtelCollectorLogsTag                     = "prometheus.log.otelcollector"
	fluentbitProcessedCountTag                        = "prometheus.log.processedcount"
	fluentbitDiagnosticHeartbeatTag                   = "prometheus.log.diagnosticheartbeat"
	fluentbitEventsProcessedLastPeriodTag             = "prometheus.log.eventsprocessedlastperiod"
	fluentbitInfiniteMetricTag                        = "prometheus.log.infinitemetric"
	fluentbitContainerLogsTag                         = "prometheus.log.prometheuscollectorcontainer"
)

// SendException  send an event to the configured app insights instance
func SendException(err interface{}) {
	if TelemetryClient != nil {
		TelemetryClient.TrackException(err)
	}
}

// InitializeTelemetryClient sets up the telemetry client to send telemetry to the App Insights instance
func InitializeTelemetryClient(agentVersion string) (int, error) {
	encodedIkey := os.Getenv(envAppInsightsAuth)
	if encodedIkey == "" {
		Log("Environment Variable Missing \n")
		return -1, errors.New("Missing Environment Variable")
	}

	decIkey, err := base64.StdEncoding.DecodeString(encodedIkey)
	if err != nil {
		Log("Decoding Error %s", err.Error())
		return -1, err
	}

	appInsightsEndpoint := os.Getenv(envAppInsightsEndpoint)
	telemetryClientConfig := appinsights.NewTelemetryConfiguration(string(decIkey))
	// endpoint override required only for sovereign clouds
	if appInsightsEndpoint != "" {
		Log("Overriding the default AppInsights EndpointUrl with %s", appInsightsEndpoint)
		telemetryClientConfig.EndpointUrl = appInsightsEndpoint
	}
	TelemetryClient = appinsights.NewTelemetryClientFromConfig(telemetryClientConfig)

	telemetryOffSwitch := os.Getenv(envTelemetryOffSwitch)
	if strings.Compare(strings.ToLower(telemetryOffSwitch), "true") == 0 {
		Log("Appinsights telemetry is disabled \n")
		TelemetryClient.SetIsEnabled(false)
	}

	CommonProperties = make(map[string]string)
	CommonProperties["cluster"] = os.Getenv(envCluster)
	CommonProperties["computer"] = os.Getenv(envComputerName)
	CommonProperties["nodeip"] = os.Getenv(envNodeIP)
	CommonProperties["mode"] = os.Getenv(envMode)
	CommonProperties["controllertype"] = os.Getenv(envControllerType)
	CommonProperties["agentversion"] = os.Getenv(envAgentVersion)
	CommonProperties["namespace"] = os.Getenv(envNamespace)
	CommonProperties["defaultmetricaccountname"] = os.Getenv(envDefaultMetricAccountName)
  CommonProperties["podname"] = os.Getenv(envPodName)
	CommonProperties["helmreleasename"] = os.Getenv(envHelmReleaseName)

	aksResourceID := os.Getenv(envAKSResourceID)
	// if the aks resource id is not defined, it is most likely an ACS Cluster
	//todo
	//fix all the casing issues below for property names and also revist these telemetry before productizing as AKS addon
	if aksResourceID == "" && os.Getenv(envACSResourceName) != "" {
		CommonProperties["ACSResourceName"] = os.Getenv(envACSResourceName)
		CommonProperties["ClusterType"] = clusterTypeACS

		CommonProperties["SubscriptionID"] = ""
		CommonProperties["ResourceGroupName"] = ""
		CommonProperties["ClusterName"] = ""
		CommonProperties["Region"] = ""
		CommonProperties["AKS_RESOURCE_ID"] = ""

	} else if aksResourceID != "" {
		CommonProperties["ACSResourceName"] = ""
		CommonProperties["AKS_RESOURCE_ID"] = aksResourceID
		splitStrings := strings.Split(aksResourceID, "/")
		if len(splitStrings) >=9 {
			CommonProperties["SubscriptionID"] = splitStrings[2]
			CommonProperties["ResourceGroupName"] = splitStrings[4]
			CommonProperties["ClusterName"] = splitStrings[8]
		}
		CommonProperties["ClusterType"] = clusterTypeAKS

		region := os.Getenv("AKS_REGION")
		CommonProperties["Region"] = region
	}

	TelemetryClient.Context().CommonProperties = CommonProperties
	return 0, nil
}

func PushLogErrorsToAppInsightsTraces(records []map[interface{}]interface{}, severityLevel contracts.SeverityLevel, tag string) int {
	var logLines []string
	for _, record := range records {
		var logEntry = ""

		// Logs have different parsed formats depending on if they're from otelcollector or metricsextension
		if tag == fluentbitOtelCollectorLogsTag {
			logEntry = fmt.Sprintf("%s %s", ToString(record["caller"]), ToString(record["msg"]))
		} else if tag == fluentbitContainerLogsTag {
			logEntry = ToString(record["log"])
		}
		logLines = append(logLines, logEntry)
	}

	traceEntry := strings.Join(logLines, "\n")
	traceTelemetryItem := appinsights.NewTraceTelemetry(traceEntry, severityLevel)
	traceTelemetryItem.Properties["tag"] = tag
	TelemetryClient.Track(traceTelemetryItem)
	return output.FLB_OK
}

// Get the account name, metrics/bytes processed count, and metrics/bytes sent count from metrics extension log line
// that was filtered by fluent-bit
func PushProcessedCountToAppInsightsMetrics(records []map[interface{}]interface{}) int {
	for _, record := range records {
		var logEntry = ToString(record["message"])
		var metricScrapeInfoRegex = regexp.MustCompile(`\s*([^\s]+)\s*([^\s]+)\s*([^\s]+).*ProcessedCount: ([\d]+).*ProcessedBytes: ([\d]+).*SentToPublicationCount: ([\d]+).*SentToPublicationBytes: ([\d]+).*`)
		groupMatches := metricScrapeInfoRegex.FindStringSubmatch(logEntry)

		if len(groupMatches) > 7 {
			metricsProcessedCount, err := strconv.ParseFloat(groupMatches[4], 64)
			if err == nil {
				metric := appinsights.NewMetricTelemetry("meMetricsProcessedCount", metricsProcessedCount)
				metric.Properties["metricsAccountName"] = groupMatches[3]
				metric.Properties["bytesProcessedCount"] = groupMatches[5]
				metric.Properties["metricsSentToPubCount"] = groupMatches[6]
				metric.Properties["bytesSentToPubCount"] = groupMatches[7]
				TelemetryClient.Track(metric)
			}
			metricsSentToPubCount, err := strconv.ParseFloat(groupMatches[6], 64)
			if err == nil {
				Log("about to lock for metrics sent")
				TimeseriesVolumeMutex.Lock()
				metricsSentTotal += metricsSentToPubCount
				TimeseriesVolumeMutex.Unlock()
				Log("unlocked for metrics sent")
			}
			bytesSentToPubCount, err := strconv.ParseFloat(groupMatches[7], 64)
			if err == nil {
				Log("About to lock for bytes sent")
				TimeseriesVolumeMutex.Lock()
				bytesSentTotal += bytesSentToPubCount
				TimeseriesVolumeMutex.Unlock()
				Log("Unlocked for bytes sent")
			}
		}
	}

	return output.FLB_OK
}

func PushMetricsDroppedCountToAppInsightsMetrics(records []map[interface{}]interface{}) int {
	for _, record := range records {
		var logEntry = ToString(record["message"])
		var metricScrapeInfoRegex = regexp.MustCompile(`.*CurrentRawDataQueueSize: (\d+).*EtwEventsDropped: (\d+).*AggregatedMetricsDropped: (\d+).*`)
		groupMatches := metricScrapeInfoRegex.FindStringSubmatch(logEntry)

		if len(groupMatches) > 3 {
			metricsDroppedCount, err := strconv.ParseFloat(groupMatches[2], 64)
			if err == nil {
				metric := appinsights.NewMetricTelemetry("meMetricsDroppedCount", metricsDroppedCount)
				metric.Properties["currentQueueSize"] = groupMatches[1]
				metric.Properties["aggregatedMetricsDropped"] = groupMatches[3]
				TelemetryClient.Track(metric)
			}
		}
	}

	return output.FLB_OK
}

func PushReceivedMetricsCountToAppInsightsMetrics(records []map[interface{}]interface{}) int {
	for _, record := range records {
		var logEntry = ToString(record["message"])
		var metricScrapeInfoRegex = regexp.MustCompile(`.*EventsProcessedLastPeriod: (\d+).*`)
		groupMatches := metricScrapeInfoRegex.FindStringSubmatch(logEntry)

		if len(groupMatches) > 1 {
		  metricsReceivedCount, err := strconv.ParseFloat(groupMatches[1], 64)
			if err == nil {
				Log("About to lock for received total")
				TimeseriesVolumeMutex.Lock()
				metricsReceivedTotal += metricsReceivedCount
				TimeseriesVolumeMutex.Unlock()
				Log("Unlocking received total")
				metric := appinsights.NewMetricTelemetry("meMetricsReceivedCount", metricsReceivedCount)
				TelemetryClient.Track(metric)
			}
		}
	}

	return output.FLB_OK
}

func PushInfiniteMetricLogToAppInsightsEvents(records []map[interface{}]interface{}) int {
	for _, record := range records {
		var logEntry = ToString(record["message"])
		var metricScrapeInfoRegex = regexp.MustCompile(`.*Metric: "(\w+)".*DimsCount: (\d+).*EstimatedSizeInBytes: (\d+).*Account: "(\w+)".*`)
		groupMatches := metricScrapeInfoRegex.FindStringSubmatch(logEntry)

		if len(groupMatches) > 4 {
			event := appinsights.NewEventTelemetry("meInfiniteMetricDropped")
			event.Properties["metric"] = groupMatches[1]
			event.Properties["dimsCount"] = groupMatches[2]
			event.Properties["estimatedBytes"] = groupMatches[3]
			event.Properties["mdmAccount"] = groupMatches[4]
			TelemetryClient.Track(event)
		}
	}

	return output.FLB_OK
}

func PublishTimeseriesVolume() {
	Log("In go routine for server")
	r := prometheus.NewRegistry()
	r.MustRegister(timeseriesReceivedMetric)
	r.MustRegister(timeseriesSentMetric)
	r.MustRegister(bytesSentMetric)
	handler := promhttp.HandlerFor(r, promhttp.HandlerOpts{})
	http.Handle("/metrics", handler)

	go func() {
		Log("In go func for ticker")
		telemetryPushInterval := 60
		TimeseriesVolumeTicker = time.NewTicker(time.Second * time.Duration(telemetryPushInterval))
		start := time.Now()
		
		for ; true; <-TimeseriesVolumeTicker.C {
			elapsed := time.Since(start)
			Log("About to lock, calculating rates")
		
			TimeseriesVolumeMutex.Lock()

			Log("Have locked, calculating rates")
			timePassedInMinutes := (float64(elapsed) / float64(time.Second)) / 60.0
			Log("time passed in seconds: %f", float64(elapsed) / float64(time.Second))
			Log("time passed in minutes: %f", timePassedInMinutes)
			timeseriesReceivedRate = math.Round(metricsReceivedTotal / timePassedInMinutes)
			timeseriesSentRate = math.Round(metricsSentTotal / timePassedInMinutes)
			bytesSentRate = math.Round(bytesSentTotal / timePassedInMinutes)

			timeseriesReceivedMetric.With(prometheus.Labels{"computer":CommonProperties["computer"], "release":CommonProperties["helmreleasename"], "controller_type":CommonProperties["controllertype"]}).Set(timeseriesReceivedRate)
			timeseriesSentMetric.With(prometheus.Labels{"computer":CommonProperties["computer"], "release":CommonProperties["helmreleasename"], "controller_type":CommonProperties["controllertype"]}).Set(timeseriesSentRate)
			bytesSentMetric.With(prometheus.Labels{"computer":CommonProperties["computer"], "release":CommonProperties["helmreleasename"], "controller_type":CommonProperties["controllertype"]}).Set(bytesSentRate)
		
			metricsReceivedTotal = 0.0
			metricsSentTotal = 0.0
			bytesSentTotal = 0.0
			TimeseriesVolumeMutex.Unlock()
			Log("Have unlocked, calculated rates")
		
			start = time.Now()
			Log("About to wait a minute")
		}
	}()

	Log("About to listen and serve")
	http.ListenAndServe(":2234", nil)
}

func metricInfoHandler(responseWriter http.ResponseWriter, request *http.Request) {
	Log("Handling request, about to lock")
	TimeseriesVolumeMutex.Lock()
	Log("Have locked, writing to response writer")
	fmt.Fprintf(responseWriter, "# HELP timeseriesReceivedTotal The total number of timeseries received by MetricsExtension\n# TYPE timeseriesReceivedTotal counter\ntimeseriesReceivedTotal{computer=\"%s\",cluster=\"%s\"} %f\n\n# HELP timeseriesSentTotal The total number of timeseries sent by MetricsExtension\n# TYPE timeseriesSentTotal counter\ntimeseriesSentTotal{computer=\"%s\",cluster=\"%s\"} %f\n\n# HELP bytesSentTotal The total number of bytest sent by MetricsExtension\n# TYPE bytesSentTotal counter\nbytesSentTotal{computer=\"%s\",cluster=\"%s\"} %f\n",
	CommonProperties["computer"], CommonProperties["cluster"], timeseriesReceivedRate, CommonProperties["computer"], CommonProperties["cluster"], timeseriesSentRate, CommonProperties["computer"], CommonProperties["cluster"], bytesSentRate)
	TimeseriesVolumeMutex.Unlock()
	Log("have unlocked")
}