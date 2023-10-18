package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fluent/fluent-bit-go/output"
	"github.com/microsoft/ApplicationInsights-Go/appinsights"
	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
	yaml "gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type meMetricsProcessedCount struct {
	DimBytesProcessedCount   float64
	DimBytesSentToPubCount   float64
	DimMetricsSentToPubCount float64
	Value                    float64
}

type meMetricsReceivedCount struct {
	Value float64
}

var (
	// CommonProperties indicates the dimensions that are sent with every event/metric
	CommonProperties map[string]string
	// TelemetryClient is the client used to send the telemetry
	TelemetryClient appinsights.TelemetryClient
	// Invalid Prometheus config validation environment variable used for telemetry
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
	// Pod Annotation metrics keep list regex
	PodannotationKeepListRegex string
	// Kappie Basic metrics keep list regex
	KappieBasicKeepListRegex string
	// Kubelet scrape interval
	KubeletScrapeInterval string
	// CoreDNS scrape interval
	CoreDNSScrapeInterval string
	// CAdvisor scrape interval
	CAdvisorScrapeInterval string
	// KubeProxy scrape interval
	KubeProxyScrapeInterval string
	// API Server scrape interval
	ApiServerScrapeInterval string
	// KubeState scrape interval
	KubeStateScrapeInterval string
	// Node Exporter scrape interval
	NodeExporterScrapeInterval string
	// Windows Exporter scrape interval
	WinExporterScrapeInterval string
	// Windows KubeProxy scrape interval
	WinKubeProxyScrapeInterval string
	// PrometheusCollector Health scrape interval
	PromHealthScrapeInterval string
	// Pod Annotation scrape interval
	PodAnnotationScrapeInterval string
	// Kappie Basic scrape interval
	KappieBasicScrapeInterval string
	// meMetricsProcessedCount map, which holds references to metrics per metric account
	meMetricsProcessedCountMap = make(map[string]*meMetricsProcessedCount)
	// meMetricsProcessedCountMapMutex -- used for reading & writing locks on meMetricsProcessedCountMap
	meMetricsProcessedCountMapMutex = &sync.Mutex{}
	// meMetricsReceivedCount map, which holds references to metrics per metric account
	meMetricsReceivedCountMap = make(map[string]*meMetricsReceivedCount)
	// meMetricsReceivedCountMapMutex -- used for reading & writing locks on meMetricsReceivedCountMap
	meMetricsReceivedCountMapMutex = &sync.Mutex{}
)

const (
	coresAttachedTelemetryIntervalSeconds = 600
	ksmAttachedTelemetryIntervalSeconds   = 600
	meMetricsTelemetryIntervalSeconds     = 300
	coresAttachedTelemetryName            = "ClusterCoreCapacity"
	linuxCpuCapacityTelemetryName         = "LiCapacity"
	linuxNodeCountTelemetryName           = "LiNodeCnt"
	windowsCpuCapacityTelemetryName       = "WiCapacity"
	windowsNodeCountTelemetryName         = "WiNodeCnt"
	virtualNodeCountTelemetryName         = "VirtualNodeCnt"
	arm64CpuCapacityTelemetryName         = "ArmCapacity"
	arm64NodeCountTelemetryName           = "ArmNodeCnt"
	marinerNodeCountTelemetryName         = "MarNodeCnt"
	marinerCpuCapacityTelemetryName       = "MarCapacity"
	ksmCpuMemoryTelemetryName             = "ksmUsage"
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
	envPrometheusCollectorHealthCcp       = "AZMON_PROMETHEUS_COLLECTOR_HEALTH_CCP_SCRAPING_ENABLED"
	fluentbitOtelCollectorLogsTag         = "prometheus.log.otelcollector"
	fluentbitProcessedCountTag            = "prometheus.log.processedcount"
	fluentbitDiagnosticHeartbeatTag       = "prometheus.log.diagnosticheartbeat"
	fluentbitEventsProcessedLastPeriodTag = "prometheus.log.eventsprocessedlastperiod"
	fluentbitInfiniteMetricTag            = "prometheus.log.infinitemetric"
	fluentbitContainerLogsTag             = "prometheus.log.prometheuscollectorcontainer"
	fluentbitExportingFailedTag           = "prometheus.log.exportingfailed"
	fluentbitFailedScrapeTag              = "prometheus.log.failedscrape"
	keepListRegexHashFilePath             = "/opt/microsoft/configmapparser/config_def_targets_metrics_keep_list_hash"
	intervalHashFilePath                  = "/opt/microsoft/configmapparser/config_def_targets_scrape_intervals_hash"
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
			PodannotationKeepListRegex = regexHash["POD_ANNOTATION_METRICS_KEEP_LIST_REGEX"]
			KappieBasicKeepListRegex = regexHash["KAPPIEBASIC_METRICS_KEEP_LIST_REGEX"]
		}
	}

	// Reading scrape interval hash file for telemetry
	intervalFileContents, err := ioutil.ReadFile(intervalHashFilePath)
	if err != nil {
		Log("Error while opening interval hash file - %v\n", err)
	} else {
		Log("Successfully read interval hash file contents for telemetry\n")
		var intervalHash map[string]string
		err = yaml.Unmarshal([]byte(intervalFileContents), &intervalHash)
		if err != nil {
			Log("Error while unmarshalling interval hash file - %v\n", err)
		} else {
			KubeletScrapeInterval = intervalHash["KUBELET_SCRAPE_INTERVAL"]
			CoreDNSScrapeInterval = intervalHash["COREDNS_SCRAPE_INTERVAL"]
			CAdvisorScrapeInterval = intervalHash["CADVISOR_SCRAPE_INTERVAL"]
			KubeProxyScrapeInterval = intervalHash["KUBEPROXY_SCRAPE_INTERVAL"]
			ApiServerScrapeInterval = intervalHash["APISERVER_SCRAPE_INTERVAL"]
			KubeStateScrapeInterval = intervalHash["KUBESTATE_SCRAPE_INTERVAL"]
			NodeExporterScrapeInterval = intervalHash["NODEEXPORTER_SCRAPE_INTERVAL"]
			WinExporterScrapeInterval = intervalHash["WINDOWSEXPORTER_SCRAPE_INTERVAL"]
			WinKubeProxyScrapeInterval = intervalHash["WINDOWSKUBEPROXY_SCRAPE_INTERVAL"]
			PromHealthScrapeInterval = intervalHash["PROMETHEUS_COLLECTOR_HEALTH_SCRAPE_INTERVAL"]
			PodAnnotationScrapeInterval = intervalHash["POD_ANNOTATION_SCRAPE_INTERVAL"]
			KappieBasicScrapeInterval = intervalHash["KAPPIEBASIC_SCRAPE_INTERVAL"]
		}
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
		telemetryProperties := map[string]int64{
			windowsCpuCapacityTelemetryName: 0,
			windowsNodeCountTelemetryName:   0,
			virtualNodeCountTelemetryName:   0,
			arm64CpuCapacityTelemetryName:   0,
			arm64NodeCountTelemetryName:     0,
		}

		nodeList, err := client.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			SendException(fmt.Sprintf("Error while getting the nodes list for cores attached telemetry: %v\n", err))
			continue
		}

		// Get core and node count by OS/arch
		for _, node := range nodeList.Items {
			osLabel := ""
			archLabel := ""
			distroLabel := ""
			if node.Labels == nil {
				SendException(fmt.Sprintf("Labels are missing for the node: %s when getting core capacity", node.Name))
			} else if node.Labels["type"] == "virtual-kubelet" {
				// Do not add core capacity total for virtual nodes as this could be extremely large
				// Just count how many virtual nodes exist
				telemetryProperties[virtualNodeCountTelemetryName] += 1
				continue
			} else {
				osLabel = node.Labels["kubernetes.io/os"]
				archLabel = node.Labels["kubernetes.io/arch"]
				distroLabel = node.Labels["kubernetes.azure.com/os-sku"]
			}

			if node.Status.Capacity == nil {
				SendException(fmt.Sprintf("Capacity is missing for the node: %s when getting core capacity", node.Name))
				continue
			}
			cpu := node.Status.Capacity["cpu"]

			if osLabel == "windows" {
				telemetryProperties[windowsCpuCapacityTelemetryName] += cpu.Value()
				telemetryProperties[windowsNodeCountTelemetryName] += 1
			} else {
				telemetryProperties[linuxCpuCapacityTelemetryName] += cpu.Value()
				telemetryProperties[linuxNodeCountTelemetryName] += 1
				if archLabel == "arm64" {
					telemetryProperties[arm64NodeCountTelemetryName] += 1
					telemetryProperties[arm64CpuCapacityTelemetryName] += cpu.Value()
				}
				if strings.ToLower(distroLabel) == "mariner" {
					telemetryProperties[marinerNodeCountTelemetryName] += 1
					telemetryProperties[marinerCpuCapacityTelemetryName] += cpu.Value()
				}
			}
		}
		// Send metric to app insights for node and core capacity
		cpuCapacityTotal := float64(telemetryProperties[linuxCpuCapacityTelemetryName] + telemetryProperties[windowsCpuCapacityTelemetryName])
		metricTelemetryItem := appinsights.NewMetricTelemetry(coresAttachedTelemetryName, cpuCapacityTotal)

		for propertyName, propertyValue := range telemetryProperties {
			if propertyValue != 0 {
				metricTelemetryItem.Properties[propertyName] = fmt.Sprintf("%d", propertyValue)
			}
		}

		TelemetryClient.Track(metricTelemetryItem)
	}
}

// Struct for getting relevant fields from JSON object obtained from cadvisor endpoint
type CadvisorJson struct {
	Pods []struct {
		Containers []struct {
			Name string `json:"name"`
			Cpu  struct {
				UsageNanoCores float64 `json:"usageNanoCores"`
			} `json:"cpu"`
			Memory struct {
				RssBytes float64 `json:"rssBytes"`
			} `json:"memory"`
		} `json:"containers"`
	} `json:"pods"`
}

// Send Cpu and Memory Usage for Kube state metrics to Application Insights periodically
func SendKsmCpuMemoryToAppInsightsMetrics() {

	var p CadvisorJson
	err := json.Unmarshal(retrieveKsmData(), &p)
	if err != nil {
		message := fmt.Sprintf("Unable to retrieve the unmarshalled Json from Cadvisor- %v\n", err)
		Log(message)
		SendException(message)
	}

	ksmTelemetryTicker := time.NewTicker(time.Second * time.Duration(ksmAttachedTelemetryIntervalSeconds))
	for ; true; <-ksmTelemetryTicker.C {
		cpuKsmUsageNanoCoresLinux := float64(0)
		memoryKsmRssBytesLinux := float64(0)

		for podId := 0; podId < len(p.Pods); podId++ {
			for containerId := 0; containerId < len(p.Pods[podId].Containers); containerId++ {
				if strings.TrimSpace(p.Pods[podId].Containers[containerId].Name) == "" {
					message := fmt.Sprintf("Container name is missing")
					Log(message)
					continue
				}
				if strings.TrimSpace(p.Pods[podId].Containers[containerId].Name) == "ama-metrics-ksm" {
					cpuKsmUsageNanoCoresLinux += p.Pods[podId].Containers[containerId].Cpu.UsageNanoCores
					memoryKsmRssBytesLinux += p.Pods[podId].Containers[containerId].Memory.RssBytes
				}
			}
		}
		// Send metric to app insights for Cpu and Memory Usage for Kube state metrics
		metricTelemetryItem := appinsights.NewMetricTelemetry(ksmCpuMemoryTelemetryName, cpuKsmUsageNanoCoresLinux)

		// Abbreviated properties to save telemetry cost
		metricTelemetryItem.Properties["MemKsmRssBytesLinux"] = fmt.Sprintf("%d", memoryKsmRssBytesLinux)

		TelemetryClient.Track(metricTelemetryItem)
	}

}

// Retrieve the JSON payload of Kube state metrics from Cadvisor endpoint
func retrieveKsmData() []byte {
	caCert, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/ca.crt")
	if err != nil {
		message := fmt.Sprintf("Error getting certificate - %v\n", err)
		Log(message)
		SendException(message)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	client := &http.Client{
		Timeout: time.Duration(5) * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:            caCertPool,
				InsecureSkipVerify: true,
			},
		},
	}
	req, err := http.NewRequest("GET", "https://"+CommonProperties["nodeip"]+":10250/stats/summary", nil)
	if err != nil {
		message := fmt.Sprintf("Error creating the http request - %v\n", err)
		Log(message)
		SendException(message)
		return nil
	}
	// Get token data
	tokendata, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		message := fmt.Sprintf("Error accessing the token data - %v\n", err)
		Log(message)
		SendException(message)
		return nil
	}
	// Create bearer token
	bearerToken := "Bearer" + " " + string(tokendata)
	req.Header.Add("Authorization", string(bearerToken))

	resp, err := client.Do(req)
	if err != nil {
		message := fmt.Sprintf("Error getting response from cadvisor- %v\n", err)
		Log(message)
		SendException(message)
		return nil
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		message := fmt.Sprintf("Error reading reponse body - %v\n", err)
		Log(message)
		SendException(message)
		return nil
	}
	return body
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

func UpdateMEMetricsProcessedCount(records []map[interface{}]interface{}) int {
	for _, record := range records {
		var logEntry = ToString(record["message"])
		var metricScrapeInfoRegex = regexp.MustCompile(`\s*([^\s]+)\s*([^\s]+)\s*([^\s]+).*ProcessedCount: ([\d]+).*ProcessedBytes: ([\d]+).*SentToPublicationCount: ([\d]+).*SentToPublicationBytes: ([\d]+).*`)
		groupMatches := metricScrapeInfoRegex.FindStringSubmatch(logEntry)

		if len(groupMatches) > 7 {
			metricsProcessedCount, err := strconv.ParseFloat(groupMatches[4], 64)
			if err == nil {

				metricsAccountName := groupMatches[3]

				bytesProcessedCount, e := strconv.ParseFloat(groupMatches[5], 64)
				if e != nil {
					bytesProcessedCount = 0.0
				}

				metricsSentToPubCount, e := strconv.ParseFloat(groupMatches[6], 64)
				if e != nil {
					metricsSentToPubCount = 0.0
				}
				bytesSentToPubCount, e := strconv.ParseFloat(groupMatches[7], 64)
				if e != nil {
					bytesSentToPubCount = 0.0
				}

				//update map
				meMetricsProcessedCountMapMutex.Lock()

				ref, ok := meMetricsProcessedCountMap[metricsAccountName]

				if ok {
					ref.DimBytesProcessedCount += bytesProcessedCount
					ref.DimBytesSentToPubCount += bytesSentToPubCount
					ref.DimMetricsSentToPubCount += metricsSentToPubCount
					ref.Value += metricsProcessedCount

				} else {
					m := &meMetricsProcessedCount{
						DimBytesProcessedCount:   bytesProcessedCount,
						DimBytesSentToPubCount:   bytesSentToPubCount,
						DimMetricsSentToPubCount: metricsSentToPubCount,
						Value:                    metricsProcessedCount,
					}
					meMetricsProcessedCountMap[metricsAccountName] = m
				}
				meMetricsProcessedCountMapMutex.Unlock()
			}

			if strings.ToLower(os.Getenv(envPrometheusCollectorHealth)) == "true" ||
				strings.ToLower(os.Getenv(envPrometheusCollectorHealthCcp)) == "true" {
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

// Get the account name, metrics/bytes processed count, and metrics/bytes sent count from metrics extension log line
// that was filtered by fluent-bit
func PushMEProcessedAndReceivedCountToAppInsightsMetrics() {

	ticker := time.NewTicker(time.Second * time.Duration(meMetricsTelemetryIntervalSeconds))
	for ; true; <-ticker.C {

		meMetricsProcessedCountMapMutex.Lock()
		for k, v := range meMetricsProcessedCountMap {
			metric := appinsights.NewMetricTelemetry("meMetricsProcessedCount", v.Value)
			metric.Properties["metricsAccountName"] = k
			metric.Properties["bytesProcessedCount"] = fmt.Sprintf("%.2f", v.DimBytesProcessedCount)
			metric.Properties["metricsSentToPubCount"] = fmt.Sprintf("%.2f", v.DimMetricsSentToPubCount)
			metric.Properties["bytesSentToPubCount"] = fmt.Sprintf("%.2f", v.DimBytesSentToPubCount)

			if InvalidCustomPrometheusConfig != "" {
				metric.Properties["InvalidCustomPrometheusConfig"] = InvalidCustomPrometheusConfig
			}
			if DefaultPrometheusConfig != "" {
				metric.Properties["DefaultPrometheusConfig"] = DefaultPrometheusConfig
			}
			if KubeletKeepListRegex != "" {
				metric.Properties["KubeletKeepListRegex"] = KubeletKeepListRegex
			}
			if CoreDNSKeepListRegex != "" {
				metric.Properties["CoreDNSKeepListRegex"] = CoreDNSKeepListRegex
			}
			if CAdvisorKeepListRegex != "" {
				metric.Properties["CAdvisorKeepListRegex"] = CAdvisorKeepListRegex
			}
			if KubeProxyKeepListRegex != "" {
				metric.Properties["KubeProxyKeepListRegex"] = KubeProxyKeepListRegex
			}
			if ApiServerKeepListRegex != "" {
				metric.Properties["ApiServerKeepListRegex"] = ApiServerKeepListRegex
			}
			if KubeStateKeepListRegex != "" {
				metric.Properties["KubeStateKeepListRegex"] = KubeStateKeepListRegex
			}
			if NodeExporterKeepListRegex != "" {
				metric.Properties["NodeExporterKeepListRegex"] = NodeExporterKeepListRegex
			}
			if WinExporterKeepListRegex != "" {
				metric.Properties["WinExporterKeepListRegex"] = WinExporterKeepListRegex
			}
			if WinKubeProxyKeepListRegex != "" {
				metric.Properties["WinKubeProxyKeepListRegex"] = WinKubeProxyKeepListRegex
			}
			if PodannotationKeepListRegex != "" {
				metric.Properties["PodannotationKeepListRegex"] = PodannotationKeepListRegex
			}
			if KappieBasicKeepListRegex != "" {
				metric.Properties["KappieBasicKeepListRegex"] = KappieBasicKeepListRegex
			}
			if KubeletScrapeInterval != "" {
				metric.Properties["KubeletScrapeInterval"] = KubeletScrapeInterval
			}
			if CoreDNSScrapeInterval != "" {
				metric.Properties["CoreDNSScrapeInterval"] = CoreDNSScrapeInterval
			}
			if CAdvisorScrapeInterval != "" {
				metric.Properties["CAdvisorScrapeInterval"] = CAdvisorScrapeInterval
			}
			if KubeProxyScrapeInterval != "" {
				metric.Properties["KubeProxyScrapeInterval"] = KubeProxyScrapeInterval
			}
			if ApiServerScrapeInterval != "" {
				metric.Properties["ApiServerScrapeInterval"] = ApiServerScrapeInterval
			}
			if KubeStateScrapeInterval != "" {
				metric.Properties["KubeStateScrapeInterval"] = KubeStateScrapeInterval
			}
			if NodeExporterScrapeInterval != "" {
				metric.Properties["NodeExporterScrapeInterval"] = NodeExporterScrapeInterval
			}
			if WinExporterScrapeInterval != "" {
				metric.Properties["WinExporterScrapeInterval"] = WinExporterScrapeInterval
			}
			if WinKubeProxyScrapeInterval != "" {
				metric.Properties["WinKubeProxyScrapeInterval"] = WinKubeProxyScrapeInterval
			}
			if PromHealthScrapeInterval != "" {
				metric.Properties["PromHealthScrapeInterval"] = PromHealthScrapeInterval
			}
			if PodAnnotationScrapeInterval != "" {
				metric.Properties["PodAnnotationScrapeInterval"] = PodAnnotationScrapeInterval
			}
			if KappieBasicScrapeInterval != "" {
				metric.Properties["KappieBasicScrapeInterval"] = KappieBasicScrapeInterval
			}

			TelemetryClient.Track(metric)

		}

		meMetricsProcessedCountMap = make(map[string]*meMetricsProcessedCount)

		meMetricsProcessedCountMapMutex.Unlock()

		//send meMetricsReceivedCount

		ref, ok := meMetricsReceivedCountMap["na"]

		if ok {
			meMetricsReceivedCountMapMutex.Lock()

			metric := appinsights.NewMetricTelemetry("meMetricsReceivedCount", ref.Value)
			TelemetryClient.Track(metric)
			meMetricsReceivedCountMap = make(map[string]*meMetricsReceivedCount)

			meMetricsReceivedCountMapMutex.Unlock()
		}

	}
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

func UpdateMEReceivedMetricsCount(records []map[interface{}]interface{}) int {
	for _, record := range records {
		var logEntry = ToString(record["message"])
		var metricScrapeInfoRegex = regexp.MustCompile(`.*EventsProcessedLastPeriod: (\d+).*`)
		groupMatches := metricScrapeInfoRegex.FindStringSubmatch(logEntry)

		if len(groupMatches) > 1 {
			metricsReceivedCount, err := strconv.ParseFloat(groupMatches[1], 64)
			if err == nil {

				//update map
				meMetricsReceivedCountMapMutex.Lock()

				ref, ok := meMetricsReceivedCountMap["na"]

				if ok {
					ref.Value += metricsReceivedCount

				} else {
					m := &meMetricsReceivedCount{
						Value: metricsReceivedCount,
					}
					meMetricsReceivedCountMap["na"] = m
				}
				meMetricsReceivedCountMapMutex.Unlock()

				// Add to the total that PublishTimeseriesVolume() uses
				if strings.ToLower(os.Getenv(envPrometheusCollectorHealth)) == "true" {
					TimeseriesVolumeMutex.Lock()
					TimeseriesReceivedTotal += metricsReceivedCount
					TimeseriesVolumeMutex.Unlock()
				}

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
