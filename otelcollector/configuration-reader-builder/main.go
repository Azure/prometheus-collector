package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os/exec"

	"os"

	"github.com/joho/godotenv"
	yaml "gopkg.in/yaml.v2"
)

type Config struct {
	LabelSelector      map[string]string      `yaml:"label_selector,omitempty"`
	Config             map[string]interface{} `yaml:"config"`
	AllocationStrategy string                 `yaml:"allocation_strategy,omitempty"`
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

// var inotifyTaConfigOutputFile = "/opt/inotifyoutput-ta-config.txt"

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
	targetAllocatorConfig := Config{
		AllocationStrategy: "consistent-hashing",
		LabelSelector: map[string]string{
			"rsName":                         "ama-metrics",
			"kubernetes.azure.com/managedby": "aks",
		},
		Config: promScrapeConfig,
	}

	targetAllocatorConfigYaml, _ := yaml.Marshal(targetAllocatorConfig)
	if err := os.WriteFile(taConfigFilePath, targetAllocatorConfigYaml, 0644); err != nil {
		logFatalError(fmt.Sprintf("config-reader::Unable to write to: %s - %v\n", taConfigFilePath, err))
		os.Exit(1)
	}

	log.Println("Updated file - targetallocator.yaml for the TargetAllocator to pick up new config changes")
	taConfigUpdated = true
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

	if taConfigUpdated {
		status = http.StatusServiceUnavailable
		message += "targetallocator-config changed"
		taLivenessCounter++
	}
	if taLivenessCounter >= 4 {
		// Setting this to false after 4 calls to healthhandler to make sure TA container doesnt keep restarting continuosly
		taConfigUpdated = false
		taLivenessCounter = 0
	}

	w.WriteHeader(status)
	fmt.Fprintln(w, message)
	if status != http.StatusOK {
		fmt.Printf(message)
	}

}

// func taHealthHandler(w http.ResponseWriter, r *http.Request) {
// 	status := http.StatusOK
// 	message := "\ntargetallocator is running."

// 	if hasConfigChanged(inotifyTaConfigOutputFile) {
// 		status = http.StatusServiceUnavailable
// 		message += "\ninotifyoutput-ta-config has been updated - target-allocator-config changed"
// 	}

// 	w.WriteHeader(status)
// 	fmt.Fprintln(w, message)
// 	if status != http.StatusOK {
// 		fmt.Printf(message)
// 	}
// }

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
	}
}

func startCommandAndWait(command string, args ...string) {
	cmd := exec.Command(command, args...)
	// ret := false

	// Set environment variables from os.Environ()
	cmd.Env = append(os.Environ())
	// // Print the environment variables being passed into the cmd
	// fmt.Println("Environment variables being passed into the command:")
	// for _, v := range cmd.Env {
	// 	fmt.Println(v)
	// }
	// Create pipes to capture stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Printf("Error creating stdout pipe: %v\n", err)
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		fmt.Printf("Error creating stderr pipe: %v\n", err)
		return
	}

	// Start the command
	err = cmd.Start()
	if err != nil {
		fmt.Printf("Error starting command: %v\n", err)
		return
	}

	// Create goroutines to capture and print stdout and stderr
	go func() {
		stdoutBytes, _ := ioutil.ReadAll(stdout)
		fmt.Print(string(stdoutBytes))
	}()

	go func() {
		stderrBytes, _ := ioutil.ReadAll(stderr)
		fmt.Print(string(stderrBytes))
	}()

	// Wait for the command to finish
	err = cmd.Wait()
	if err != nil {
		fmt.Printf("Error waiting for command: %v\n", err)
		return
	}
	fmt.Printf("Done command start and wait\n")
}

func main() {
	//configFilePtr := flag.String("config", "", "Config file to read")
	//flag.Parse()
	//otelConfigFilePath := *configFilePtr
	// updateTAConfigFile(otelConfigFilePath)

	//configmap-parser.sh

	_, err := os.Create("/opt/inotifyoutput.txt")
	// inotifywait /etc/config/settings --daemon --recursive --outfile "/opt/inotifyoutput.txt" --event create,delete --format '%e : %T' --timefmt '+%s'
	if err != nil {
		log.Fatalf("Error creating output file: %v\n", err)
	}

	// Define the command to start inotify for config reader's liveness probe
	inotifyCommandCfg := exec.Command(
		"inotifywait",
		"/etc/config/settings",
		"--daemon",
		"--recursive",
		"--outfile", "/opt/inotifyoutput.txt",
		"--event", "create,delete",
		"--format", "%e : %T",
		"--timefmt", "+%s",
	)

	// Start the inotify process
	err = inotifyCommandCfg.Start()
	if err != nil {
		log.Fatalf("Error starting inotify process for config reader's liveness probe: %v\n", err)
	}

	// Define the command to start inotify for config reader's liveness probe
	// taConfigFilePath := "/conf"

	// // Create an output file for inotify events
	// //outputFile := "/opt/inotifyoutput-ta-config.txt"
	// _, err = os.Create(inotifyTaConfigOutputFile)
	// if err != nil {
	// 	log.Fatalf("Error creating output file for TA config: %v\n", err)
	// }

	// // Define the command to start inotify
	// inotifyCommandTA := exec.Command(
	// 	"inotifywait",
	// 	taConfigFilePath,
	// 	"--daemon",
	// 	"--outfile", inotifyTaConfigOutputFile,
	// 	"--event", "ATTRIB",
	// 	"--format", "%e : %T",
	// 	"--timefmt", "+%s",
	// )

	// // Start the inotify process
	// err = inotifyCommandTA.Start()
	// if err != nil {
	// 	log.Fatalf("Error starting inotify process for watching TA config: %v\n", err)
	// }

	// configParserCommand := exec.Command(
	// 	"/bin/sh",
	// 	"/opt/configmap-parser.sh",
	// )

	startCommandAndWait("/bin/sh", "/opt/configmap-parser.sh")

	//stdout, err := configParserCommand.Output()

	// if err != nil {
	// 	fmt.Println(err.Error())
	// 	return
	// }

	// err = configParserCommand.Wait()
	// if err != nil {
	// 	fmt.Printf("Error waiting for shell command: %v\n", err)
	// } else {
	// 	if os.Getenv("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG") == "true" {
	// 		if _, err = os.Stat("/opt/microsoft/otelcollector/collector-config-default.yml"); err == nil {
	// 			updateTAConfigFile("/opt/microsoft/otelcollector/collector-config-default.yml")
	// 		}
	// 	} else if _, err = os.Stat("/opt/microsoft/otelcollector/collector-config.yml"); err == nil {
	// 		updateTAConfigFile("/opt/microsoft/otelcollector/collector-config.yml")
	// 	} else {
	// 		log.Println("No configs found via configmap, not running config reader")
	// 	}
	// }

	err := godotenv.Load("envvars.env")
	if os.Getenv("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG") == "true" {
		if _, err = os.Stat("/opt/microsoft/otelcollector/collector-config-default.yml"); err == nil {
			updateTAConfigFile("/opt/microsoft/otelcollector/collector-config-default.yml")
		}
	} else if _, err = os.Stat("/opt/microsoft/otelcollector/collector-config.yml"); err == nil {
		updateTAConfigFile("/opt/microsoft/otelcollector/collector-config.yml")
	} else {
		log.Println("No configs found via configmap, not running config reader")
	}

	// Print the output
	// fmt.Println(string(stdout))

	// Start the inotify process
	// err = configParserCommand.Start()
	// if err != nil {
	// 	log.Fatalf("Error running configparser: %v\n", err)
	// }

	// Run configreader to update the configmap for TargetAllocator
	// if os.Getenv("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG") == "true" {
	// 	if _, err = os.Stat("/opt/microsoft/otelcollector/collector-config-default.yml"); err == nil {
	// 		updateTAConfigFile("/opt/microsoft/otelcollector/collector-config-default.yml")
	// 	}
	// } else if _, err = os.Stat("/opt/microsoft/otelcollector/collector-config.yml"); err == nil {
	// 	updateTAConfigFile("/opt/microsoft/otelcollector/collector-config.yml")
	// } else {
	// 	log.Println("No configs found via configmap, not running config reader")
	// }

	// updateTAConfigFile("/opt/microsoft/otelcollector/collector-config-default.yml")

	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/health-ta", taHealthHandler)
	http.ListenAndServe(":8081", nil)
}
