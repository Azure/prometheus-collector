package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"

	shared "github.com/prometheus-collector/shared"
	ccpconfigmapsettings "github.com/prometheus-collector/shared/configmap/ccp"
	configmapsettings "github.com/prometheus-collector/shared/configmap/mp"

	"strconv"
	"strings"
	"time"
)

func main() {
	controllerType := shared.GetControllerType()
	cluster := shared.GetEnv("CLUSTER", "")
	clusterOverride := shared.GetEnv("CLUSTER_OVERRIDE", "")
	aksRegion := shared.GetEnv("AKSREGION", "")
	ccpMetricsEnabled := shared.GetEnv("CCP_METRICS_ENABLED", "false")
	osType := os.Getenv("OS_TYPE")

	outputFile := "/opt/inotifyoutput.txt"
	if err := shared.Inotify(outputFile, "/etc/config/settings", "/etc/prometheus/certs"); err != nil {
		log.Fatal(err)
	}

	if ccpMetricsEnabled != "true" {
		if err := shared.SetupArcEnvironment(); err != nil {
			shared.EchoError(err.Error())
		}
	}
	// Check if MODE environment variable is empty
	mode := shared.GetEnv("MODE", "simple")
	shared.EchoVar("MODE", mode)
	shared.EchoVar("CONTROLLER_TYPE", shared.GetEnv("CONTROLLER_TYPE", ""))
	shared.EchoVar("CLUSTER", cluster)

	customEnvironment := shared.GetEnv("customEnvironment", "")
	if ccpMetricsEnabled != "true" {
		shared.SetupTelemetry(customEnvironment)
		if err := shared.ConfigureEnvironment(); err != nil {
			log.Fatalf("Error configuring environment: %v\n", err)
		}
	}

	if ccpMetricsEnabled == "true" {
		ccpconfigmapsettings.Configmapparserforccp()
	} else {
		configmapsettings.Configmapparser()
	}

	if ccpMetricsEnabled != "true" && osType == "linux" {
		shared.StartCronDaemon()
	}

	var meConfigFile string
	var fluentBitConfigFile string

	meConfigFile, fluentBitConfigFile = shared.DetermineConfigFiles(controllerType, clusterOverride)
	fmt.Println("meConfigFile:", meConfigFile)
	fmt.Println("fluentBitConfigFile:", fluentBitConfigFile)

	shared.WaitForTokenAdapter(ccpMetricsEnabled)

	if ccpMetricsEnabled != "true" {
		shared.SetEnvAndSourceBashrcOrPowershell("ME_CONFIG_FILE", meConfigFile, true)
		shared.SetEnvAndSourceBashrcOrPowershell("customResourceId", cluster, true)
	} else {
		os.Setenv("ME_CONFIG_FILE", meConfigFile)
		os.Setenv("customResourceId", cluster)
	}

	trimmedRegion := strings.ToLower(strings.ReplaceAll(aksRegion, " ", ""))
	if ccpMetricsEnabled != "true" {
		shared.SetEnvAndSourceBashrcOrPowershell("customRegion", trimmedRegion, true)
	} else {
		os.Setenv("customRegion", trimmedRegion)
	}

	fmt.Println("Waiting for 10s for token adapter sidecar to be up and running so that it can start serving IMDS requests")
	time.Sleep(10 * time.Second)

	fmt.Println("Starting MDSD")
	if ccpMetricsEnabled != "true" {
		shared.StartMdsdForOverlay()
	} else {
		shared.StartMdsdForUnderlay()
	}

	// update this to use color coding
	shared.PrintMdsdVersion()

	fmt.Println("Waiting for 30s for MDSD to get the config and put them in place for ME")
	time.Sleep(30 * time.Second)

	fmt.Println("Starting Metrics Extension with config overrides")
	if ccpMetricsEnabled != "true" {
		if _, err := shared.StartMetricsExtensionForOverlay(meConfigFile); err != nil {
			log.Fatalf("Error starting MetricsExtension: %v\n", err)
		}
	} else {
		shared.StartMetricsExtensionWithConfigOverridesForUnderlay(meConfigFile)
	}
	// ME_PID, err := shared.StartMetricsExtensionForOverlay(meConfigFile)
	// if err != nil {
	// 	fmt.Printf("Error starting MetricsExtension: %v\n", err)
	// 	return
	// }
	// fmt.Printf("ME_PID: %d\n", ME_PID)

	// // Modify fluentBitConfigFile using ME_PID
	// err = shared.ModifyConfigFile(fluentBitConfigFile, ME_PID, "${ME_PID}")
	// if err != nil {
	// 	fmt.Printf("Error modifying config file: %v\n", err)
	// }

	// Start otelcollector
	azmonOperatorEnabled := os.Getenv("AZMON_OPERATOR_ENABLED")
	azmonUseDefaultPrometheusConfig := os.Getenv("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG")

	var collectorConfig string

	if controllerType == "replicaset" && azmonOperatorEnabled == "true" {
		fmt.Println("Starting otelcollector in replicaset with Target allocator settings")
		if ccpMetricsEnabled == "true" {
			collectorConfig = "/opt/microsoft/otelcollector/ccp-collector-config-replicaset.yml"
		} else {
			collectorConfig = "/opt/microsoft/otelcollector/collector-config-replicaset.yml"
		}
	} else if azmonUseDefaultPrometheusConfig == "true" {
		fmt.Println("Starting otelcollector with only default scrape configs enabled")
		if ccpMetricsEnabled == "true" {
			collectorConfig = "/opt/microsoft/otelcollector/ccp-collector-config-default.yml"
		} else {
			collectorConfig = "/opt/microsoft/otelcollector/collector-config-default.yml"
		}
	} else {
		collectorConfig = "/opt/microsoft/otelcollector/collector-config.yml"
	}

	fmt.Println("startCommand otelcollector")
	_, err := shared.StartCommandWithOutputFile("/opt/microsoft/otelcollector/otelcollector", []string{"--config", collectorConfig}, "/opt/microsoft/otelcollector/collector-log.txt")
	// OTEL_PID, err := shared.StartCommandWithOutputFile("/opt/microsoft/otelcollector/otelcollector", []string{"--config", collectorConfig}, "/opt/microsoft/otelcollector/collector-log.txt")
	// if err != nil {
	// 	fmt.Printf("Error starting command: %v\n", err)
	// 	return
	// }
	// fmt.Printf("OTEL_PID: %d\n", OTEL_PID)

	// // Modify fluentBitConfigFile using OTEL_PID
	// err = shared.ModifyConfigFile(fluentBitConfigFile, OTEL_PID, "${OTEL_PID}")
	// if err != nil {
	// 	fmt.Printf("Error modifying config file: %v\n", err)
	// }

	shared.LogVersionInfo()

	if ccpMetricsEnabled != "true" {
		shared.StartFluentBit(fluentBitConfigFile)

		// Run the command and capture the output
		cmd := exec.Command("fluent-bit", "--version")
		fluentBitVersion, err := cmd.Output()
		if err != nil {
			log.Fatalf("failed to run command: %v", err)
		}
		shared.EchoVar("FLUENT_BIT_VERSION", string(fluentBitVersion))

		shared.StartTelegraf()

	}

	// Start inotify to watch for changes
	fmt.Println("Starting inotify for watching mdsd config update")

	// Create an output file for inotify events
	outputFile = "/opt/inotifyoutput-mdsd-config.txt"
	_, err = os.Create(outputFile)
	if err != nil {
		log.Fatalf("Error creating output file: %v\n", err)
	}

	// Define the command to start inotify
	inotifyCommand := exec.Command(
		"inotifywait",
		"/etc/mdsd.d/config-cache/metricsextension/TokenConfig.json",
		"--daemon",
		"--outfile", outputFile,
		"--event", "ATTRIB",
		"--format", "%e : %T",
		"--timefmt", "+%s",
	)

	// Start the inotify process
	err = inotifyCommand.Start()
	if err != nil {
		log.Fatalf("Error starting inotify process: %v\n", err)
	}

	// Setting time at which the container started running
	epochTimeNow := time.Now().Unix()
	epochTimeNowReadable := time.Unix(epochTimeNow, 0).Format(time.RFC3339)

	// Writing the epoch time to a file
	file, err := os.Create("/opt/microsoft/liveness/azmon-container-start-time")
	if err != nil {
		fmt.Println("Error creating file:", err)
		return
	}
	defer file.Close()

	_, err = file.WriteString(fmt.Sprintf("%d", epochTimeNow))
	if err != nil {
		fmt.Println("Error writing to file:", err)
		return
	}

	// Printing the environment variable and the readable time
	fmt.Printf("AZMON_CONTAINER_START_TIME=%d\n", epochTimeNow)
	shared.FmtVar("AZMON_CONTAINER_START_TIME_READABLE", epochTimeNowReadable)

	// Expose a health endpoint for liveness probe
	http.HandleFunc("/health", healthHandler)
	http.ListenAndServe(":8080", nil)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	status := http.StatusOK
	message := "prometheuscollector is running."

	// Checking if TokenConfig file exists
	if _, err := os.Stat("/etc/mdsd.d/config-cache/metricsextension/TokenConfig.json"); os.IsNotExist(err) {
		fmt.Println("TokenConfig.json does not exist")
		if _, err := os.Stat("/opt/microsoft/liveness/azmon-container-start-time"); err == nil {
			fmt.Println("azmon-container-start-time file exists, reading start time")
			azmonContainerStartTimeStr, err := os.ReadFile("/opt/microsoft/liveness/azmon-container-start-time")
			if err != nil {
				status = http.StatusServiceUnavailable
				message = "Error reading azmon-container-start-time: " + err.Error()
				fmt.Println(message)
				goto response
			}

			azmonContainerStartTime, err := strconv.Atoi(strings.TrimSpace(string(azmonContainerStartTimeStr)))
			if err != nil {
				status = http.StatusServiceUnavailable
				message = "Error converting azmon-container-start-time to integer: " + err.Error()
				fmt.Println(message)
				goto response
			}

			epochTimeNow := int(time.Now().Unix())
			duration := epochTimeNow - azmonContainerStartTime
			durationInMinutes := duration / 60
			fmt.Printf("Container has been running for %d minutes\n", durationInMinutes)

			if durationInMinutes%5 == 0 {
				fmt.Printf("%s No configuration present for the AKS resource\n", time.Now().Format("2006-01-02T15:04:05"))
			}

			if durationInMinutes > 15 {
				status = http.StatusServiceUnavailable
				message = "No configuration present for the AKS resource"
				fmt.Println(message)
				goto response
			}
		} else {
			fmt.Println("azmon-container-start-time file does not exist")
		}
	} else {
		if !shared.IsProcessRunning("/usr/sbin/MetricsExtension") {
			status = http.StatusServiceUnavailable
			message = "Metrics Extension is not running (configuration exists)"
			fmt.Println(message)
			goto response
		}

		if !shared.IsProcessRunning("/usr/sbin/mdsd") {
			status = http.StatusServiceUnavailable
			message = "mdsd not running (configuration exists)"
			fmt.Println(message)
			goto response
		}
	}

	if shared.HasConfigChanged("/opt/inotifyoutput-mdsd-config.txt") {
		status = http.StatusServiceUnavailable
		message = "inotifyoutput-mdsd-config.txt has been updated - mdsd config changed"
		fmt.Println(message)
		goto response
	}

	if !shared.IsProcessRunning("/opt/microsoft/otelcollector/otelcollector") {
		status = http.StatusServiceUnavailable
		message = "OpenTelemetryCollector is not running."
		fmt.Println(message)
		goto response
	}

	if shared.HasConfigChanged("/opt/inotifyoutput.txt") {
		status = http.StatusServiceUnavailable
		message = "inotifyoutput.txt has been updated - config changed"
		fmt.Println(message)
		goto response
	}

response:
	w.WriteHeader(status)
	fmt.Fprintln(w, message)
	if status != http.StatusOK {
		fmt.Printf("Health check failed: %d, Message: %s\n", status, message)
		shared.WriteTerminationLog(message)
	}
}
