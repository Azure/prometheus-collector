package main

import (
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"os"

	configmapsettings "github.com/prometheus-collector/shared/configmap/mp"
	"github.com/prometheus/common/model"
	yaml "gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PrometheusCRConfig struct {
	Enabled                         bool                  `yaml:"enabled,omitempty"`
	AllowNamespaces                 []string              `yaml:"allow_namespaces,omitempty"`
	DenyNamespaces                  []string              `yaml:"deny_namespaces,omitempty"`
	PodMonitorSelector              *metav1.LabelSelector `yaml:"pod_monitor_selector,omitempty"`
	PodMonitorNamespaceSelector     *metav1.LabelSelector `yaml:"pod_monitor_namespace_selector,omitempty"`
	ServiceMonitorSelector          *metav1.LabelSelector `yaml:"service_monitor_selector,omitempty"`
	ServiceMonitorNamespaceSelector *metav1.LabelSelector `yaml:"service_monitor_namespace_selector,omitempty"`
	ScrapeConfigSelector            *metav1.LabelSelector `yaml:"scrape_config_selector,omitempty"`
	ScrapeConfigNamespaceSelector   *metav1.LabelSelector `yaml:"scrape_config_namespace_selector,omitempty"`
	ProbeSelector                   *metav1.LabelSelector `yaml:"probe_selector,omitempty"`
	ProbeNamespaceSelector          *metav1.LabelSelector `yaml:"probe_namespace_selector,omitempty"`
	ScrapeInterval                  model.Duration        `yaml:"scrape_interval,omitempty"`
}

type Config struct {
	CollectorSelector  *metav1.LabelSelector  `yaml:"collector_selector,omitempty"`
	Config             map[string]interface{} `yaml:"config"`
	AllocationStrategy string                 `yaml:"allocation_strategy,omitempty"`
	PrometheusCR       PrometheusCRConfig     `yaml:"prometheus_cr,omitempty"`
	FilterStrategy     string                 `yaml:"filter_strategy,omitempty"`
}

type OtelConfig struct {
	Exporters  interface{} `yaml:"exporters"`
	Processors interface{} `yaml:"processors"`
	Extensions interface{} `yaml:"extensions"`
	Receivers  struct {
		Prometheus struct {
			Config          map[string]interface{} `yaml:"config"`
			TargetAllocator interface{}            `yaml:"target_allocator"`
		} `yaml:"prometheus"`
	} `yaml:"receivers"`
	Service struct {
		Extensions interface{} `yaml:"extensions"`
		Pipelines  struct {
			Metrics struct {
				Exporters  interface{} `yaml:"exporters"`
				Processors interface{} `yaml:"processors"`
				Receivers  interface{} `yaml:"receivers"`
			} `yaml:"metrics"`
			MetricsTelemetry struct {
				Exporters  interface{} `yaml:"exporters,omitempty"`
				Processors interface{} `yaml:"processors,omitempty"`
				Receivers  interface{} `yaml:"receivers,omitempty"`
			} `yaml:"metrics/telemetry,omitempty"`
		} `yaml:"pipelines"`
		Telemetry struct {
			Logs struct {
				Level    interface{} `yaml:"level"`
				Encoding interface{} `yaml:"encoding"`
			} `yaml:"logs"`
		} `yaml:"telemetry"`
	} `yaml:"service"`
}

var RESET = "\033[0m"
var RED = "\033[31m"

var taConfigFilePath = "/ta-configuration/targetallocator.yaml"
var taConfigUpdated = false
var taLivenessCounter = 0
var taLivenessStartTime = time.Time{}

func logFatalError(message string) {
	// Always log the full message
	log.Fatalf("%s%s%s", RED, message, RESET)
}

func updateTAConfigFile(configFilePath string) {
	defaultsMergedConfigFileContents, err := os.ReadFile(configFilePath)
	if err != nil {
		logFatalError(fmt.Sprintf("config-reader::Unable to read file contents from: %s - %v\n", configFilePath, err))
		os.Exit(1)
	}
	var promScrapeConfig map[string]interface{}
	var otelConfig OtelConfig
	err = yaml.Unmarshal([]byte(defaultsMergedConfigFileContents), &otelConfig)
	if err != nil {
		logFatalError(fmt.Sprintf("config-reader::Unable to unmarshal merged otel configuration from: %s - %v\n", configFilePath, err))
		os.Exit(1)
	}

	promScrapeConfig = otelConfig.Receivers.Prometheus.Config
	// Removing $$ added for regex and replacement in relabel_config and metric_relabel_config added by promconfigvalidator.
	// The $$ are required by the validator's otel get method, but the TA doesnt do env substitution and hence needs to be removed, else TA crashes.
	scrapeConfigs := promScrapeConfig["scrape_configs"]
	if scrapeConfigs != nil {
		var sc = scrapeConfigs.([]interface{})
		for _, scrapeConfig := range sc {
			scrapeConfig := scrapeConfig.(map[interface{}]interface{})
			if scrapeConfig["relabel_configs"] != nil {
				relabelConfigs := scrapeConfig["relabel_configs"].([]interface{})
				for _, relabelConfig := range relabelConfigs {
					relabelConfig := relabelConfig.(map[interface{}]interface{})
					//replace $$ with $ for regex field
					if relabelConfig["regex"] != nil {
						// Adding this check here since regex can be boolean and the conversion will fail
						if _, isString := relabelConfig["regex"].(string); isString {
							regexString := relabelConfig["regex"].(string)
							modifiedRegexString := strings.ReplaceAll(regexString, "$$", "$")
							relabelConfig["regex"] = modifiedRegexString
						}
					}
					//replace $$ with $ for replacement field
					if relabelConfig["replacement"] != nil {
						replacement := relabelConfig["replacement"].(string)
						modifiedReplacementString := strings.ReplaceAll(replacement, "$$", "$")
						relabelConfig["replacement"] = modifiedReplacementString
					}
				}
			}

			if scrapeConfig["metric_relabel_configs"] != nil {
				metricRelabelConfigs := scrapeConfig["metric_relabel_configs"].([]interface{})
				for _, metricRelabelConfig := range metricRelabelConfigs {
					metricRelabelConfig := metricRelabelConfig.(map[interface{}]interface{})
					//replace $$ with $ for regex field
					if metricRelabelConfig["regex"] != nil {
						// Adding this check here since regex can be boolean and the conversion will fail
						if _, isString := metricRelabelConfig["regex"].(string); isString {
							regexString := metricRelabelConfig["regex"].(string)
							modifiedRegexString := strings.ReplaceAll(regexString, "$$", "$")
							metricRelabelConfig["regex"] = modifiedRegexString
						}
					}

					//replace $$ with $ for replacement field
					if metricRelabelConfig["replacement"] != nil {
						replacement := metricRelabelConfig["replacement"].(string)
						modifiedReplacementString := strings.ReplaceAll(replacement, "$$", "$")
						metricRelabelConfig["replacement"] = modifiedReplacementString
					}
				}
			}
		}
	}

	targetAllocatorConfig := Config{
		AllocationStrategy: "consistent-hashing",
		FilterStrategy:     "relabel-config",
		CollectorSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"rsName":                         "ama-metrics",
				"kubernetes.azure.com/managedby": "aks",
			},
		},
		Config: promScrapeConfig,
		PrometheusCR: PrometheusCRConfig{
			ServiceMonitorSelector: &metav1.LabelSelector{},
			PodMonitorSelector:     &metav1.LabelSelector{},
		},
	}

	targetAllocatorConfigYaml, _ := yaml.Marshal(targetAllocatorConfig)
	if err := os.WriteFile(taConfigFilePath, targetAllocatorConfigYaml, 0644); err != nil {
		logFatalError(fmt.Sprintf("config-reader::Unable to write to: %s - %v\n", taConfigFilePath, err))
		os.Exit(1)
	}

	log.Println("Updated file - targetallocator.yaml for the TargetAllocator to pick up new config changes")
	taConfigUpdated = true
	taLivenessStartTime = time.Now()
}

func hasConfigChanged(filePath string) bool {
	if _, err := os.Stat(filePath); err == nil {
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			fmt.Println("Error getting file info:", err)
			os.Exit(1)
		}

		return fileInfo.Size() > 0
	}
	return false
}

func taHealthHandler(w http.ResponseWriter, r *http.Request) {
	status := http.StatusOK
	message := "\ntargetallocator is running."

	client := &http.Client{Timeout: time.Duration(2) * time.Second}

	req, err := http.NewRequest("GET", "http://localhost:8080/metrics", nil)
	if err == nil {
		resp, _ := client.Do(req)
		if resp != nil && resp.StatusCode == http.StatusOK {
			if taConfigUpdated {
				if !taLivenessStartTime.IsZero() {
					duration := time.Since(taLivenessStartTime)
					// Serve the response of ServiceUnavailable for 60s and then reset
					if duration.Seconds() < 60 {
						status = http.StatusServiceUnavailable
						message += "targetallocator-config changed"
					} else {
						taConfigUpdated = false
						taLivenessStartTime = time.Time{}
					}
				}
			}

			if status != http.StatusOK {
				fmt.Printf(message)
			}
			w.WriteHeader(status)
			fmt.Fprintln(w, message)
		}
		if resp != nil && resp.Body != nil {
			defer resp.Body.Close()
		}
	} else {
		message = "\ncall to get TA metrics failed"
		status = http.StatusServiceUnavailable
		fmt.Printf(message)
		w.WriteHeader(status)
		fmt.Fprintln(w, message)
	}
}

func writeTerminationLog(message string) {
	if err := os.WriteFile("/dev/termination-log", []byte(message), fs.FileMode(0644)); err != nil {
		log.Printf("Error writing to termination log: %v", err)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	status := http.StatusOK
	message := "\nconfig-reader is running."

	if hasConfigChanged("/opt/inotifyoutput.txt") {
		status = http.StatusServiceUnavailable
		message += "\ninotifyoutput.txt has been updated - config-reader-config changed"
	}

	w.WriteHeader(status)
	fmt.Fprintln(w, message)
	if status != http.StatusOK {
		fmt.Printf(message)
		writeTerminationLog(message)
	}
}

func main() {
	_, err := os.Create("/opt/inotifyoutput.txt")
	if err != nil {
		log.Fatalf("Error creating output file: %v\n", err)
		fmt.Println("Error creating inotify output file:", err)
	}

	// Define the command to start inotify for config reader's liveness probe
	inotifyCommandCfg := exec.Command(
		"inotifywait",
		"/etc/config/settings",
		"--daemon",
		"--recursive",
		"--outfile", "/opt/inotifyoutput.txt",
		"--event", "create",
		"--event", "delete",
		"--format", "%e : %T",
		"--timefmt", "+%s",
	)

	// Start the inotify process
	err = inotifyCommandCfg.Start()
	if err != nil {
		log.Fatalf("Error starting inotify process for config reader's liveness probe: %v\n", err)
		fmt.Println("Error starting inotify process:", err)
	}

	configmapsettings.Configmapparser()
	if os.Getenv("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG") == "true" {
		if _, err = os.Stat("/opt/microsoft/otelcollector/collector-config-default.yml"); err == nil {
			updateTAConfigFile("/opt/microsoft/otelcollector/collector-config-default.yml")
		}
	} else if _, err = os.Stat("/opt/microsoft/otelcollector/collector-config.yml"); err == nil {
		updateTAConfigFile("/opt/microsoft/otelcollector/collector-config.yml")
	} else {
		log.Println("No configs found via configmap, not running config reader")
	}

	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/health-ta", taHealthHandler)

	http.ListenAndServe(":8081", nil)

}
