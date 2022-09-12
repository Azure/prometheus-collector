package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"context"

	yaml "gopkg.in/yaml.v2"
	"golang.org/x/exp/maps"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"github.com/fluent/fluent-bit-go/output"
	"github.com/microsoft/ApplicationInsights-Go/appinsights"
	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
)

var (
	// CommonProperties indicates the dimensions that are sent with every event/metric
	CommonProperties map[string]string
	// TelemetryClient is the client used to send the telemetry
	TelemetryClient appinsights.TelemetryClient
	// Invalid Prometheus config validation environemnt variable used for telemetry
	InvalidCustomPrometheusConfig string
	// Default Collector config
	DefaultPrometheusConfig string
	// Kubelet metrics keep list regex
	KubeletKeepListRegex string
	// CoreDNS metrics keep list regex
	CoreDNSKeepListRegex string
	// CAdvisor metrics keep list regex
	CAdvisorKeepListRegex string
	// KubeProxy metrics keep list regex
	KubeProxyKeepListRegex string
	// API Server metrics keep list regex
	ApiServerKeepListRegex string
	// KubeState metrics keep list regex
	KubeStateKeepListRegex string
	// Node Exporter metrics keep list regex
	NodeExporterKeepListRegex string
	// Windows Exporter metrics keep list regex
	WinExporterKeepListRegex string
	// Windows KubeProxy metrics keep list regex
	WinKubeProxyKeepListRegex string
	// ExtraProperties
	ExtraProperties map[string]string
)

const (
	coresAttachedTelemetryIntervalSeconds = 600
	coresAttachedTelemetryName            = "ClusterCoreCapacity"
	envAgentVersion                       = "AGENT_VERSION"
	envControllerType                     = "CONTROLLER_TYPE"
	envNodeIP                             = "NODE_IP"
	envMode                               = "MODE"
	envCluster                            = "customResourceId" //this will contain full resourceid for MAC , ir-resprective of cluster_alias set or not
	// explicitly defining below for clarity, but not send thru our telemetry for brieviety
	//envCustomResourceId					  = "customResourceId"
	//envClusterAlias						  = "AZMON_CLUSTER_ALIAS"
	//envClusterLabel						  = "AZMON_CLUSTER_LABEL"
	envAppInsightsAuth                    = "APPLICATIONINSIGHTS_AUTH"
	envAppInsightsEndpoint                = "APPLICATIONINSIGHTS_ENDPOINT"
	envComputerName                       = "NODE_NAME"
	envDefaultMetricAccountName           = "AZMON_DEFAULT_METRIC_ACCOUNT_NAME"
	envPodName                            = "POD_NAME"
	envTelemetryOffSwitch                 = "DISABLE_TELEMETRY"
	envNamespace                          = "POD_NAMESPACE"
	envHelmReleaseName                    = "HELM_RELEASE_NAME"
	envPrometheusCollectorHealth          = "AZMON_PROMETHEUS_COLLECTOR_HEALTH_SCRAPING_ENABLED"
	fluentbitOtelCollectorLogsTag         = "prometheus.log.otelcollector"
	fluentbitProcessedCountTag            = "prometheus.log.processedcount"
	fluentbitDiagnosticHeartbeatTag       = "prometheus.log.diagnosticheartbeat"
	fluentbitEventsProcessedLastPeriodTag = "prometheus.log.eventsprocessedlastperiod"
	fluentbitInfiniteMetricTag            = "prometheus.log.infinitemetric"
	fluentbitContainerLogsTag             = "prometheus.log.prometheuscollectorcontainer"
	fluentbitExportingFailedTag           = "prometheus.log.exportingfailed"
	fluentbitFailedScrapeTag              = "prometheus.log.failedscrape"
	keepListRegexHashFilePath             = "/opt/microsoft/configmapparser/config_def_targets_metrics_keep_list_hash"
	amcsConfigFilePath                    = "/etc/mdsd.d/config-cache/metricsextension/TokenConfig.json"
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
	CommonProperties["osType"] = os.Getenv("OS_TYPE")

	isMacMode := os.Getenv("MAC")
	if strings.Compare(strings.ToLower(isMacMode), "true") == 0 {
		CommonProperties["macmode"] = isMacMode
		aksResourceID := os.Getenv("CLUSTER")
		CommonProperties["Region"] = os.Getenv("AKSREGION")
		splitStrings := strings.Split(aksResourceID, "/")
		if len(splitStrings) >= 9 {
			CommonProperties["SubscriptionID"] = splitStrings[2]
			CommonProperties["ResourceGroupName"] = splitStrings[4]
			CommonProperties["ClusterName"] = splitStrings[8]
		}
		// Reading AMCS config file for telemetry
		amcsConfigFile, err := os.Open(amcsConfigFilePath)
		if err != nil {
			message := fmt.Sprintf("Error while opening AMCS config file - %v\n", err)
			Log(message)
			SendException(message)
		}
		Log("Successfully read AMCS config file contents for telemetry\n")
		defer amcsConfigFile.Close()

		amcsConfigFileContents, err := ioutil.ReadAll(amcsConfigFile)
		if err != nil {
			message := fmt.Sprintf("Error while reading AMCS config file contents - %v\n", err)
			Log(message)
			SendException(message)
		}

		var amcsConfig map[string]interface{}

		err = json.Unmarshal([]byte(amcsConfigFileContents), &amcsConfig)
		if err != nil {
			message := fmt.Sprintf("Error while unmarshaling AMCS config file contents - %v\n", err)
			Log(message)
			SendException(message)
		}

		// iterate through keys and parse dcr name
		for key, _ := range amcsConfig {
			Log("Parsing %v for extracting DCR:", key)
			splitKey := strings.Split(key, "/")
			// Expecting a key in this format to extract out DCR Id -
			// https://<dce>.eastus2euap-1.metrics.ingest.monitor.azure.com/api/v1/dataCollectionRules/<dcrid>/streams/Microsoft-PrometheusMetrics
			if len(splitKey) == 9 {
				dcrId := CommonProperties["DCRId"]
				if dcrId == "" {
					CommonProperties["DCRId"] = splitKey[6]
				} else {
					dcrIdArray := dcrId + ";" + splitKey[6]
					CommonProperties["DCRId"] = dcrIdArray
				}
			} else {
				message := fmt.Sprintf("AMCS token config json key contract has changed, unable to get DCR ID. Logging the entire key as DCRId")
				Log(message)
				SendException(message)
				dcrId := CommonProperties["DCRId"]
				if dcrId == "" {
					CommonProperties["DCRId"] = key
				} else {
					dcrIdArray := dcrId + ";" + key
					CommonProperties["DCRId"] = dcrIdArray
				}
			}
		}
	}

	TelemetryClient.Context().CommonProperties = CommonProperties

	InvalidCustomPrometheusConfig = os.Getenv("AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG")
	DefaultPrometheusConfig = os.Getenv("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG")

	// Reading regex hash file for telemetry
	regexFileContents, err := ioutil.ReadFile(keepListRegexHashFilePath)
	if err != nil {
		Log("Error while opening regex hash file - %v\n", err)
	} else {
		Log("Successfully read regex hash file contents for telemetry\n")
		var regexHash map[string]string
		err = yaml.Unmarshal([]byte(regexFileContents), &regexHash)
		if err != nil {
			Log("Error while unmarshalling regex hash file - %v\n", err)
		} else {
			KubeletKeepListRegex = regexHash["KUBELET_METRICS_KEEP_LIST_REGEX"]
			CoreDNSKeepListRegex = regexHash["COREDNS_METRICS_KEEP_LIST_REGEX"]
			CAdvisorKeepListRegex = regexHash["CADVISOR_METRICS_KEEP_LIST_REGEX"]
			KubeProxyKeepListRegex = regexHash["KUBEPROXY_METRICS_KEEP_LIST_REGEX"]
			ApiServerKeepListRegex = regexHash["APISERVER_METRICS_KEEP_LIST_REGEX"]
			KubeStateKeepListRegex = regexHash["KUBESTATE_METRICS_KEEP_LIST_REGEX"]
			NodeExporterKeepListRegex = regexHash["NODEEXPORTER_METRICS_KEEP_LIST_REGEX"]
			WinExporterKeepListRegex = regexHash["WINDOWSEXPORTER_METRICS_KEEP_LIST_REGEX"]
			WinKubeProxyKeepListRegex = regexHash["WINDOWSKUBEPROXY_METRICS_KEEP_LIST_REGEX"]
		}
	}


	ExtraProperties = make(map[string]string)
	ExtraProperties["cpulimit"] = os.Getenv("CONTAINER_CPU_LIMIT")
	ExtraProperties["memlimit"] = os.Getenv("CONTAINER_MEMORY_LIMIT")
	ExtraProperties["defaultscrapekubelet"] = os.Getenv("AZMON_PROMETHEUS_KUBELET_SCRAPING_ENABLED")
	ExtraProperties["defaultscrapecoreDns"] = os.Getenv("AZMON_PROMETHEUS_COREDNS_SCRAPING_ENABLED")
	ExtraProperties["defaultscrapecadvisor"] = os.Getenv("AZMON_PROMETHEUS_CADVISOR_SCRAPING_ENABLED")
	ExtraProperties["defaultscrapekubeproxy"] = os.Getenv("AZMON_PROMETHEUS_KUBEPROXY_SCRAPING_ENABLED")
	ExtraProperties["defaultscrapeapiserver"] = os.Getenv("AZMON_PROMETHEUS_APISERVER_SCRAPING_ENABLED")
	ExtraProperties["defaultscrapekubestate"] = os.Getenv("AZMON_PROMETHEUS_KUBESTATE_SCRAPING_ENABLED")
	ExtraProperties["defaultscrapenodeexporter"] = os.Getenv("AZMON_PROMETHEUS_NODEEXPORTER_SCRAPING_ENABLED")
	ExtraProperties["defaultscrapecollectorhealth"] = os.Getenv("AZMON_PROMETHEUS_COLLECTOR_HEALTH_SCRAPING_ENABLED")
	ExtraProperties["defaultscrapewindowsexporter"] = os.Getenv("AZMON_PROMETHEUS_WINDOWSEXPORTER_SCRAPING_ENABLED")
	ExtraProperties["defaultscrapewindowskubeproxy"] = os.Getenv("AZMON_PROMETHEUS_WINDOWSKUBEPROXY_SCRAPING_ENABLED")
	ExtraProperties["nodeexportertargetport"] = os.Getenv("NODE_EXPORTER_TARGETPORT")
	ExtraProperties["nodeexportername"] = os.Getenv("NODE_EXPORTER_NAME")
	ExtraProperties["kubestatename"] = os.Getenv("KUBE_STATE_NAME")
	ExtraProperties["kubestateversion"] = os.Getenv("KUBE_STATE_VERSION")
	ExtraProperties["nodeexporterversion"] = os.Getenv("NODE_EXPORTER_VERSION")
	ExtraProperties["akvauth"] = os.Getenv("$AKVAUTH")
	ExtraProperties["debugmodeenabled"] = os.Getenv("$DEBUG_MODE_ENABLED")
	ExtraProperties["kubestatemetriclabelsallowlist"] = os.Getenv("$KUBE_STATE_METRIC_LABELS_ALLOWLIST")
	ExtraProperties["kubestatemetricannotationsallowlist"] = os.Getenv("$KUBE_STATE_METRIC_ANNOTATIONS_ALLOWLIST")
	if InvalidCustomPrometheusConfig != "" {
		ExtraProperties["InvalidCustomPrometheusConfig"] = InvalidCustomPrometheusConfig
	}
	if DefaultPrometheusConfig != "" {
		ExtraProperties["DefaultPrometheusConfig"] = DefaultPrometheusConfig
	}
	if KubeletKeepListRegex != "" {
		ExtraProperties["KubeletKeepListRegex"] = KubeletKeepListRegex
	}
	if CoreDNSKeepListRegex != "" {
		ExtraProperties["CoreDNSKeepListRegex"] = CoreDNSKeepListRegex
	}
	if CAdvisorKeepListRegex != "" {
		ExtraProperties["CAdvisorKeepListRegex"] = CAdvisorKeepListRegex
	}
	if KubeProxyKeepListRegex != "" {
		ExtraProperties["KubeProxyKeepListRegex"] = KubeProxyKeepListRegex
	}
	if ApiServerKeepListRegex != "" {
		ExtraProperties["ApiServerKeepListRegex"] = ApiServerKeepListRegex
	}
	if KubeStateKeepListRegex != "" {
		ExtraProperties["KubeStateKeepListRegex"] = KubeStateKeepListRegex
	}
	if NodeExporterKeepListRegex != "" {
		ExtraProperties["NodeExporterKeepListRegex"] = NodeExporterKeepListRegex
	}
	if WinExporterKeepListRegex != "" {
		ExtraProperties["WinExporterKeepListRegex"] = WinExporterKeepListRegex
	}
	if WinKubeProxyKeepListRegex != "" {
		ExtraProperties["WinKubeProxyKeepListRegex"] = WinKubeProxyKeepListRegex
	}

	return 0, nil
}

// Send count of cores/nodes attached to Application Insights periodically
func SendCoreCountToAppInsightsMetrics() {
	config, err := rest.InClusterConfig()
	if err != nil {
		SendException(fmt.Sprintf("Error while getting the credentials for the golang client for cores attached telemetry: %v\n", err))
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		SendException(fmt.Sprintf("Error while creating the golang client for cores attached telemetry: %v\n", err))
	}

	coreCountTelemetryTicker := time.NewTicker(time.Second * time.Duration(coresAttachedTelemetryIntervalSeconds))
	for ; true; <-coreCountTelemetryTicker.C {
		cpuCapacityTotalLinux := int64(0)
		cpuCapacityTotalWindows := int64(0)
		linuxNodeCount := 0
		windowsNodeCount := 0

		nodeList, err := client.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			SendException(fmt.Sprintf("Error while getting the nodes list for cores attached telemetry: %v\n", err))
			continue
		}

		// Get core and node count by OS
		for _, node := range nodeList.Items {
			osLabel := ""
			if node.Labels == nil {
				SendException(fmt.Sprintf("Labels are missing for the node: %s when getting core capacity", node.Name))
			} else {
				osLabel = node.Labels["kubernetes.io/os"]
			}

			if node.Status.Capacity == nil {
				SendException(fmt.Sprintf("Capacity is missing for the node: %s when getting core capacity", node.Name))
				continue
			}
			cpu := node.Status.Capacity["cpu"]

			if osLabel == "windows" {
				cpuCapacityTotalWindows += cpu.Value()
				windowsNodeCount += 1
			} else {
				cpuCapacityTotalLinux += cpu.Value()
				linuxNodeCount += 1
			}
		}

		// Send metric to app insights for node and core capacity
		cpuCapacityTotal := float64(cpuCapacityTotalLinux + cpuCapacityTotalWindows)
		metricTelemetryItem := appinsights.NewMetricTelemetry(coresAttachedTelemetryName, cpuCapacityTotal)

		// Abbreviated properties to save telemetry cost
		metricTelemetryItem.Properties["LiCapacity"] = fmt.Sprintf("%d", cpuCapacityTotalLinux)
		metricTelemetryItem.Properties["LiNodeCnt"] = fmt.Sprintf("%d", linuxNodeCount)
		metricTelemetryItem.Properties["WiCapacity"] = fmt.Sprintf("%d", cpuCapacityTotalWindows)
		metricTelemetryItem.Properties["WiNodeCnt"] = fmt.Sprintf("%d", windowsNodeCount)

		TelemetryClient.Track(metricTelemetryItem)
	}
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

func PushProcessMemoryToAppInsightsMetrics(records []map[interface{}]interface{}, tag string) int {
	for _, record := range records {
		Log(fmt.Sprintf("mem record: %v", record))
		var memoryUsage, _ = ToFloat(record["maxRSS"])
		metric := appinsights.NewMetricTelemetry("procMemUsage", memoryUsage)
		if strings.Contains(tag, "metricsextension") {
			metric.Properties["procName"] = "metricsextension"
		} else if strings.Contains(tag, "otelcollector") {
			metric.Properties["procName"] = "otelcollector"
		}
		TelemetryClient.Track(metric)
	}
	return output.FLB_OK
}

func PushProcessCpuToAppInsightsMetrics(records []map[interface{}]interface{}, tag string) int {
	for _, record := range records {
		Log(fmt.Sprintf("cpu record: %v", record))
		var cpuUsage, _ = ToFloat(record["maxCpu"])
		metric := appinsights.NewMetricTelemetry("procCpuUsage", cpuUsage)
		if strings.Contains(tag, "metricsextension") {
			metric.Properties["procName"] = "metricsextension"
		} else if strings.Contains(tag, "otelcollector") {
			metric.Properties["procName"] = "otelcollector"
			maps.Copy(metric.Properties, ExtraProperties)
		}
		TelemetryClient.Track(metric)
	}
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

			if strings.ToLower(os.Getenv(envPrometheusCollectorHealth)) == "true" {
				// Add to the total that PublishTimeseriesVolume() uses
				metricsSentToPubCount, err := strconv.ParseFloat(groupMatches[6], 64)
				if err == nil {
					TimeseriesVolumeMutex.Lock()
					TimeseriesSentTotal += metricsSentToPubCount
					TimeseriesVolumeMutex.Unlock()
				}

				// Add to the total that PublishTimeseriesVolume() uses
				bytesSentToPubCount, err := strconv.ParseFloat(groupMatches[7], 64)
				if err == nil {
					TimeseriesVolumeMutex.Lock()
					BytesSentTotal += bytesSentToPubCount
					TimeseriesVolumeMutex.Unlock()
				}
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

				// Add to the total that PublishTimeseriesVolume() uses
				if strings.ToLower(os.Getenv(envPrometheusCollectorHealth)) == "true" {
					TimeseriesVolumeMutex.Lock()
					TimeseriesReceivedTotal += metricsReceivedCount
					TimeseriesVolumeMutex.Unlock()
				}

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

func RecordExportingFailed(records []map[interface{}]interface{}) int {
	if strings.ToLower(os.Getenv(envPrometheusCollectorHealth)) == "true" {
		ExportingFailedMutex.Lock()
		OtelCollectorExportingFailedCount += 1
		ExportingFailedMutex.Unlock()
	}
	return output.FLB_OK
}
