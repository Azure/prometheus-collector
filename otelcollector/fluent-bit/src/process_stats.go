package main

import (
	"fmt"
	"maps"
	"math"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/microsoft/ApplicationInsights-Go/appinsights"
	stats "github.com/shirou/gopsutil/v4/process"
)

var replicasetDimensionsNameToEnvVar = map[string]string{
	"cpulimit":                                "CONTAINER_CPU_LIMIT",
	"memlimit":                                "CONTAINER_MEMORY_LIMIT",
	"defaultscrapekubelet":                    "AZMON_PROMETHEUS_KUBELET_SCRAPING_ENABLED",
	"defaultscrapecoreDns":                    "AZMON_PROMETHEUS_COREDNS_SCRAPING_ENABLED",
	"defaultscrapecadvisor":                   "AZMON_PROMETHEUS_CADVISOR_SCRAPING_ENABLED",
	"defaultscrapekubeproxy":                  "AZMON_PROMETHEUS_KUBEPROXY_SCRAPING_ENABLED",
	"defaultscrapeapiserver":                  "AZMON_PROMETHEUS_APISERVER_SCRAPING_ENABLED",
	"defaultscrapekubestate":                  "AZMON_PROMETHEUS_KUBESTATE_SCRAPING_ENABLED",
	"defaultscrapenodeexporter":               "AZMON_PROMETHEUS_NODEEXPORTER_SCRAPING_ENABLED",
	"defaultscrapecollectorhealth":            "AZMON_PROMETHEUS_COLLECTOR_HEALTH_SCRAPING_ENABLED",
	"defaultscrapewindowsexporter":            "AZMON_PROMETHEUS_WINDOWSEXPORTER_SCRAPING_ENABLED",
	"defaultscrapewindowskubeproxy":           "AZMON_PROMETHEUS_WINDOWSKUBEPROXY_SCRAPING_ENABLED",
	"defaultscrapepodannotations":             "AZMON_PROMETHEUS_POD_ANNOTATION_SCRAPING_ENABLED",
	"podannotationns":                         "AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX",
	"defaultscrapekappiebasic":                "AZMON_PROMETHEUS_KAPPIEBASIC_SCRAPING_ENABLED",
	"defaultscrapenetworkobservabilityRetina": "AZMON_PROMETHEUS_NETWORKOBSERVABILITYRETINA_SCRAPING_ENABLED",
	"defaultscrapenetworkobservabilityHubble": "AZMON_PROMETHEUS_NETWORKOBSERVABILITYHUBBLE_SCRAPING_ENABLED",
	"defaultscrapenetworkobservabilityCilium": "AZMON_PROMETHEUS_NETWORKOBSERVABILITYCILIUM_SCRAPING_ENABLED",
	"nodeexportertargetport":                  "NODE_EXPORTER_TARGETPORT",
	"nodeexportername":                        "NODE_EXPORTER_NAME",
	"kubestatename":                           "KUBE_STATE_NAME",
	"kubestateversion":                        "KUBE_STATE_VERSION",
	"nodeexporterversion":                     "NODE_EXPORTER_VERSION",
	"akvauth":                                 "AKVAUTH",
	"debugmodeenabled":                        "DEBUG_MODE_ENABLED",
	"kubestatemetriclabelsallowlist":          "KUBE_STATE_METRIC_LABELS_ALLOWLIST",
	"kubestatemetricannotationsallowlist":     "KUBE_STATE_METRIC_ANNOTATIONS_ALLOWLIST",
	"httpproxyenabled":                        "HTTP_PROXY_ENABLED",
	"tadapterh":                               "tokenadapterHealthyAfterSecs",
	"tadapterf":                               "tokenadapterUnhealthyAfterSecs",
	"setGlobalSettings":                       "AZMON_SET_GLOBAL_SETTINGS",
	"globalSettingsConfigured":                "AZMON_GLOBAL_SETTINGS_CONFIGURED",
	"calias":                                  "AZMON_CLUSTER_ALIAS",
	"clabel":                                  "AZMON_CLUSTER_LABEL",
	"mip":                                     "MINIMAL_INGESTION_PROFILE",
	"operatormodel":                           "AZMON_OPERATOR_ENABLED",
	"operatormodelcfgmapsetting":              "AZMON_OPERATOR_ENABLED_CFG_MAP_SETTING",
	"operatormodelchartsetting":               "AZMON_OPERATOR_ENABLED_CHART_SETTING",
	"collectorHpaEnabled":                     "AZMON_COLLECTOR_HPA_ENABLED",
}

var daemonsetDimensionsNameToEnvVar = map[string]string{
	"cpulimit":         "CONTAINER_CPU_LIMIT",
	"memlimit":         "CONTAINER_MEMORY_LIMIT",
	"debugmodeenabled": "DEBUG_MODE_ENABLED",
	"tadapterh":        "tokenadapterHealthyAfterSecs",
	"tadapterf":        "tokenadapterUnhealthyAfterSecs",
}

type Process struct {
	processName         string
	processPID          int32
	cpuValues           sort.Float64Slice
	memValues           sort.Float64Slice
	process             *stats.Process
	telemetryDimensions map[string]string
}

type ProcessAggregations struct {
	processMap map[string]*Process
	mu         sync.Mutex
}

func InitProcessAggregations(processName []string, os string) *ProcessAggregations {
	fmt.Println("Starting process aggregations")

	processAggregationsMap := make(map[string]*Process)
	for _, processName := range processName {
		pids, err := findPIDFromExe(processName, os)
		if err != nil || len(pids) == 0 {
			fmt.Printf("Error getting PID for process %s: %s\n", processName, err.Error())
			continue
		}

		process, err := stats.NewProcess(pids[0])
		if err != nil {
			fmt.Printf("Error tracking process %s\n", processName)
			continue
		}

		p := Process{
			processName:         processName,
			processPID:          pids[0],
			process:             process,
			telemetryDimensions: getExtraDimensions(processName), // Set dimensions from env vars once
		}

		processAggregationsMap[processName] = &p
	}

	return &ProcessAggregations{
		processMap: processAggregationsMap,
	}
}

func (pa *ProcessAggregations) Run() {
	go pa.CollectStats()
	go pa.SendToAppInsights()
}

func (pa *ProcessAggregations) CollectStats() {
	ticker := time.NewTicker(time.Second * time.Duration(5))
	for ; true; <-ticker.C {
		pa.mu.Lock()

		for _, p := range pa.processMap {

			// 0 means to use the delta with the previous CPU seconds reading
			cpu, err := p.process.Percent(0)
			if err == nil {
				p.cpuValues = append(p.cpuValues, cpu)
				p.cpuValues.Sort()
			}

			mem, err := p.process.MemoryInfo()
			if err == nil {
				p.memValues = append(p.memValues, float64(mem.RSS))
				p.memValues.Sort()
			}
		}

		Log("Collected process stats")

		pa.mu.Unlock()
	}
}

func (pa *ProcessAggregations) SendToAppInsights() {
	ticker := time.NewTicker(time.Second * time.Duration(300))
	for ; true; <-ticker.C {
		pa.mu.Lock()

		// For each process, send 50th and 95th percentile CPU and Memory usage
		for processName, p := range pa.processMap {
			for _, percentile := range []int{50, 95} {

				if len(p.cpuValues) > 0 {
					cpuMetric := createProcessMetric(processName, "cpu_usage", percentile, p.cpuValues)

					// Add telemetry dimensions to the metric properties
					maps.Copy(cpuMetric.Properties, p.telemetryDimensions)

					TelemetryClient.Track(cpuMetric)
				}

				if len(p.memValues) > 0 {
					memMetric := createProcessMetric(processName, "memory_rss", percentile, p.memValues)
					TelemetryClient.Track(memMetric)
				}
			}

			Log(fmt.Sprintf("Sent telemetry for process %s", processName))

			// Clear values for next aggregation period
			p.cpuValues = sort.Float64Slice{}
			p.memValues = sort.Float64Slice{}
		}

		pa.mu.Unlock()
	}
}

func getExtraDimensions(processName string) map[string]string {
	extraDimensions := make(map[string]string)

	if processName == "otelcollector" {
		var dimensionNamesToEnvVar map[string]string

		controllerType := os.Getenv(envControllerType)
		if controllerType == "ReplicaSet" {
			dimensionNamesToEnvVar = replicasetDimensionsNameToEnvVar
		} else if controllerType == "DaemonSet" {
			dimensionNamesToEnvVar = daemonsetDimensionsNameToEnvVar
		}

		for dimensionName, envVarName := range dimensionNamesToEnvVar {
			envVarValue := os.Getenv(envVarName)
			if envVarValue != "" {
				extraDimensions[dimensionName] = envVarValue
			}
		}
	}

	return extraDimensions
}

func createProcessMetric(processName string, metricName string, percentile int, values sort.Float64Slice) *appinsights.MetricTelemetry {
	return appinsights.NewMetricTelemetry(
		fmt.Sprintf("%s_%s_0%d", strings.ToLower(processName), metricName, percentile),
		float64(values[int(math.Round(float64(len(values)-1)*float64(percentile)/100.0))]),
	)
}
