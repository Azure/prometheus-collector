package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	shared "github.com/prometheus-collector/shared"
	ccpconfigmapsettings "github.com/prometheus-collector/shared/configmap/ccp"
	configmapsettings "github.com/prometheus-collector/shared/configmap/mp"

	"strconv"
	"strings"
	"time"
)

func main() {

	// Handle SIGTERM
	go handleShutdown()

	controllerType := shared.GetControllerType()
	cluster := shared.GetEnv("CLUSTER", "")
	clusterOverride := shared.GetEnv("CLUSTER_OVERRIDE", "")
	aksRegion := shared.GetEnv("AKSREGION", "")
	ccpMetricsEnabled := shared.GetEnv("CCP_METRICS_ENABLED", "false")
	osType := os.Getenv("OS_TYPE")

	if osType == "windows" {
		shared.SetEnvVariablesForWindows()
	}

	if osType == "linux" {
		outputFile := "/opt/inotifyoutput.txt"

		if ccpMetricsEnabled != "true" { //data-plane

			if err := shared.Inotify(outputFile, "/etc/config/settings"); err != nil {
				log.Fatal(err)
			}

			if err := shared.Inotify(outputFile, "/etc/prometheus/certs"); err != nil {
				log.Fatal(err)
			}
		} else { //control-plane
			if err := shared.InotifyCCP(outputFile, "/etc/config/settings"); err != nil {
				log.Fatal(err)
			}
		}
	} else if osType == "windows" {
		fmt.Println("Starting filesystemwatcher.ps1")
		shared.StartCommand("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", "C:\\opt\\scripts\\filesystemwatcher.ps1")
	}

	if ccpMetricsEnabled != "true" && osType == "linux" {
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

	if ccpMetricsEnabled != "true" {
		if osType == "linux" {
			fmt.Println("Starting MDSD")
			shared.StartMdsdForOverlay()
		} else {
			fmt.Println("Starting MA")
			shared.StartMA()
		}
	} else {
		shared.StartMdsdForUnderlay()
	}

	if osType == "linux" {
		// update this to use color coding
		shared.PrintMdsdVersion()
	}

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
			configmapsettings.SetGlobalSettingsInCollectorConfig()
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

	if controllerType == "replicaset" {
		if os.Getenv("AZMON_OPERATOR_HTTPS_ENABLED") == "true" {
			_ = shared.CollectorTAHttpsCheck(collectorConfig)
		} else {
			_ = shared.RemoveHTTPSSettingsInCollectorConfig(collectorConfig)
		}
		_, err := shared.StartCommandWithOutputFile("/opt/microsoft/otelcollector/otelcollector", []string{"--config", collectorConfig}, "/opt/microsoft/otelcollector/collector-log.txt")
		if err != nil {
			fmt.Printf("Error starting otelcollector: %v\n", err)
		}
		if os.Getenv("AZMON_OPERATOR_HTTPS_ENABLED") == "true" {
			// starting inotify here so that it doesnt restart when it is written the first time
			outputFile := "/opt/inotifyoutput.txt"
			if err = shared.Inotify(outputFile, "/etc/operator-targets/ca/certs"); err != nil {
				fmt.Printf("Error starting inotify for watching targetallocator certs: %v\n", err)
			}
		}
	} else {
		_, err := shared.StartCommandWithOutputFile("/opt/microsoft/otelcollector/otelcollector", []string{"--config", collectorConfig}, "/opt/microsoft/otelcollector/collector-log.txt")
		if err != nil {
			fmt.Printf("Error starting otelcollector: %v\n", err)
		}
	}

	if osType == "linux" {
		shared.LogVersionInfo()
	}

	if ccpMetricsEnabled != "true" {
		shared.StartFluentBit(fluentBitConfigFile)
		// Run the command and capture the output
		if osType == "linux" {
			cmd := exec.Command("fluent-bit", "--version")
			fluentBitVersion, err := cmd.Output()
			if err != nil {
				log.Fatalf("failed to run command: %v", err)
			}
			shared.EchoVar("FLUENT_BIT_VERSION", string(fluentBitVersion))
		} else if osType == "windows" {
			cmd := exec.Command("C:\\opt\\fluent-bit\\bin\\fluent-bit.exe", "--version")
			fluentBitVersion, err := cmd.Output()
			if err != nil {
				log.Fatalf("failed to run command: %v", err)
			}
			shared.EchoVar("FLUENT_BIT_VERSION", string(fluentBitVersion))
		}
	}

	if osType == "linux" {
		// Start inotify to watch for changes
		fmt.Println("Starting inotify for watching mdsd config update")

		// Create an output file for inotify events
		outputFile := "/opt/inotifyoutput-mdsd-config.txt"
		_, err := os.Create(outputFile)
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
			"--event", "create",
			"--event", "delete",
			"--event", "modify",
			"--format", "%e : %T",
			"--timefmt", "+%s",
		)

		// Start the inotify process
		err = inotifyCommand.Start()
		if err != nil {
			log.Fatalf("Error starting inotify process: %v\n", err)
		}
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

// handleShutdown listens for SIGTERM signals and handles cleanup.
func handleShutdown() {
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGTERM)

	// Block until a signal is received
	<-shutdownChan
	fmt.Println("shutting down")
	os.Exit(0) // Exit the application
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	osType := os.Getenv("OS_TYPE")
	status := http.StatusOK
	message := "prometheuscollector is running."
	processToCheck := ""

	tokenConfigFileLocation := "/etc/mdsd.d/config-cache/metricsextension/TokenConfig.json"
	if osType == "windows" {
		tokenConfigFileLocation = "C:\\opt\\genevamonitoringagent\\datadirectory\\mcs\\metricsextension\\TokenConfig.json"
	}

	// Checking if TokenConfig file exists
	if _, err := os.Stat(tokenConfigFileLocation); os.IsNotExist(err) {
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
		processToCheck = "/usr/sbin/MetricsExtension"
		if osType == "windows" {
			processToCheck = "MetricsExtension.Native.exe"
		}
		if !shared.IsProcessRunning(processToCheck) {
			status = http.StatusServiceUnavailable
			message = "Metrics Extension is not running (configuration exists)"
			fmt.Println(message)
			goto response
		}

		processToCheck = "/usr/sbin/mdsd"
		if osType == "windows" {
			processToCheck = "MonAgentLauncher.exe"
		}
		if !shared.IsProcessRunning(processToCheck) {
			status = http.StatusServiceUnavailable
			message = "mdsd not running (configuration exists)"
			fmt.Println(message)
			goto response
		}
	}
	if osType == "linux" {
		if shared.HasConfigChanged("/opt/inotifyoutput-mdsd-config.txt") {
			status = http.StatusServiceUnavailable
			message = "inotifyoutput-mdsd-config.txt has been updated - mdsd config changed"
			fmt.Println(message)
			goto response
		}
		if shared.HasConfigChanged("/opt/inotifyoutput.txt") {
			status = http.StatusServiceUnavailable
			message = "inotifyoutput.txt has been updated - config changed"
			fmt.Println(message)
			goto response
		}
	} else {
		if shared.HasConfigChanged("C:\\opt\\microsoft\\scripts\\filesystemwatcher.txt") {
			status = http.StatusServiceUnavailable
			message = "Config Map Updated or DCR/DCE updated since agent started"
			fmt.Println(message)
			goto response
		}
	}

	processToCheck = "/opt/microsoft/otelcollector/otelcollector"
	if osType == "windows" {
		processToCheck = "otelcollector.exe"
	}
	if !shared.IsProcessRunning(processToCheck) {
		status = http.StatusServiceUnavailable
		message = "OpenTelemetryCollector is not running."
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
