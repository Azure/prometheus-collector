package main

import (
    "fmt"
	"net/http"
    "os"
    "os/exec"
    "time"
    "io/ioutil"
    "strings"
	"log"
	"strconv"
)

func main(){
	mac := os.Getenv("MAC")
    controllerType := os.Getenv("CONTROLLER_TYPE")
	clusterOverride := os.Getenv("CLUSTER_OVERRIDE")
	cluster := os.Getenv("CLUSTER")
	aksRegion := os.Getenv("AKSREGION")
	ccpMetricsEnabled := os.Getenv("CCP_METRICS_ENABLED")

	outputFile := "/opt/inotifyoutput.txt"
	err := monitorInotify(outputFile)
	if err != nil {
		log.Fatal(err)
	}

	if ccpMetricsEnabled == "true" {
		confgimapparserforccp()
	} else {
		// TODO : Part of Step 1 of ccp merge to main
		// configmapparser()
	}

	var meConfigFile string

	if strings.ToLower(controllerType) == "replicaset" {
		if clusterOverride == "true" {
			meConfigFile = "/usr/sbin/me_internal.config"
		} else {
			meConfigFile = "/usr/sbin/me.config"
		}
	} else {
		println("Failed: controllerType is not 'replicaset' and only replicaset mode is supported for CCP")
		os.Exit(1)
	}
	fmt.Println("meConfigFile:", meConfigFile)

    if mac == "true" {
		// Wait for addon-token-adapter to be healthy
		tokenAdapterWaitSecs := 20
		waitedSecsSoFar := 1

		for {
			if waitedSecsSoFar > tokenAdapterWaitSecs {
				_, err := http.Get("http://localhost:9999/healthz")
				if err != nil {
					fmt.Printf("giving up waiting for token adapter to become healthy after %d secs\n", waitedSecsSoFar)
					// Log telemetry about failure after waiting for waitedSecsSoFar and break
					fmt.Printf("export tokenadapterUnhealthyAfterSecs=%d\n", waitedSecsSoFar)
					break
				}
			} else {
				fmt.Printf("checking health of token adapter after %d secs\n", waitedSecsSoFar)
				resp, err := http.Get("http://localhost:9999/healthz")
				if err == nil && resp.StatusCode == http.StatusOK {
					fmt.Printf("found token adapter to be healthy after %d secs\n", waitedSecsSoFar)
					// Log telemetry about success after waiting for waitedSecsSoFar and break
					fmt.Printf("export tokenadapterHealthyAfterSecs=%d\n", waitedSecsSoFar)
					break
				}
			}

			time.Sleep(1 * time.Second)
			waitedSecsSoFar++
		}
    }

	// Set environment variables
	os.Setenv("ME_CONFIG_FILE", meConfigFile)
	os.Setenv("customResourceId", cluster)

	trimmedRegion := strings.ReplaceAll(aksRegion, " ", "")
	trimmedRegion = strings.ToLower(trimmedRegion)
	os.Setenv("customRegion", trimmedRegion)

	fmt.Println("Waiting for 10s for token adapter sidecar to be up and running so that it can start serving IMDS requests")
	time.Sleep(10 * time.Second)
	
	fmt.Println("Starting MDSD")
	startMdsd()
	
	printMdsdVersion()
	
	fmt.Println("Waiting for 30s for MDSD to get the config and put them in place for ME")
	time.Sleep(30 * time.Second)
	
	fmt.Println("Starting metricsextension with config overrides")
	startMetricsExtensionWithConfigOverrides(meConfigFile)

    // Get ME version
	meVersion, err := readVersionFile("/opt/metricsextversion.txt")
	if err != nil {
		fmt.Printf("Error reading ME version file: %v\n", err)
	} else {
		fmtVar("ME_VERSION", meVersion)
	}

	// Get Golang version
	golangVersion, err := readVersionFile("/opt/goversion.txt")
	if err != nil {
		fmt.Printf("Error reading Golang version file: %v\n", err)
	} else {
		fmtVar("GOLANG_VERSION", golangVersion)
	}

	// Start otelcollector
	azmonOperatorEnabled := os.Getenv("AZMON_OPERATOR_ENABLED")
	azmonUseDefaultPrometheusConfig := os.Getenv("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG")

	var collectorConfig string

	if controllerType == "replicaset" && azmonOperatorEnabled == "true" {
		fmt.Println("Starting otelcollector in replicaset with Target allocator settings")
		collectorConfig = "/opt/microsoft/otelcollector/collector-config-replicaset.yml"
	} else if azmonUseDefaultPrometheusConfig == "true" {
		fmt.Println("Starting otelcollector with only default scrape configs enabled")
		collectorConfig = "/opt/microsoft/otelcollector/collector-config-default.yml"
	} else {
		fmt.Println("Starting otelcollector with collector-config.yml")
		collectorConfig = "/opt/microsoft/otelcollector/collector-config.yml"
	}

	fmt.Println("startCommand otelcollector")
	startCommand("/opt/microsoft/otelcollector/otelcollector", "--config", collectorConfig)

	otelCollectorVersion, err := exec.Command("/opt/microsoft/otelcollector/otelcollector", "--version", "").Output()
	if err != nil {
		fmt.Printf("Error getting otelcollector version: %v\n", err)
	} else {
		fmtVar("OTELCOLLECTOR_VERSION", string(otelCollectorVersion))
	}

	prometheusVersion, err := readVersionFile("/opt/microsoft/otelcollector/PROMETHEUS_VERSION")
	if err != nil {
		fmt.Printf("Error reading Prometheus version file: %v\n", err)
	} else {
		fmtVar("PROMETHEUS_VERSION", prometheusVersion)
	}

	if mac == "true" {
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
    fmt.Printf("AZMON_CONTAINER_START_TIME_READABLE=%s\n", epochTimeNowReadable)

    // Expose a health endpoint for liveness probe
    http.HandleFunc("/health", healthHandler)
    http.ListenAndServe(":8080", nil)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	status := http.StatusOK
	message := "\nprometheuscollector is running."
	macMode := os.Getenv("MAC") == "true"

	if macMode {
		if _, err := os.Stat("/etc/mdsd.d/config-cache/metricsextension/TokenConfig.json"); os.IsNotExist(err) {
			if _, err := os.Stat("/opt/microsoft/liveness/azmon-container-start-time"); err == nil {
				azmonContainerStartTimeStr, err := ioutil.ReadFile("/opt/microsoft/liveness/azmon-container-start-time")
				if err != nil {
					status = http.StatusServiceUnavailable
					message += "\nError reading azmon-container-start-time: " + err.Error()
				}

				azmonContainerStartTime, err := strconv.Atoi(strings.TrimSpace(string(azmonContainerStartTimeStr)))
				if err != nil {
					status = http.StatusServiceUnavailable
					message += "\nError converting azmon-container-start-time to integer: " + err.Error()
				}

				epochTimeNow := int(time.Now().Unix())
				duration := epochTimeNow - azmonContainerStartTime
				durationInMinutes := duration / 60

				if durationInMinutes%5 == 0 {
					message += fmt.Sprintf("\n%s No configuration present for the AKS resource\n", time.Now().Format("2006-01-02T15:04:05"))
				}

				if durationInMinutes > 15 {
					status = http.StatusServiceUnavailable
					message += "\nNo configuration present for the AKS resource"
				}
			}
		}
	} else {
		if !isProcessRunning("MetricsExtension") {
			status = http.StatusServiceUnavailable
			message += "\nMetrics Extension is not running."
		}
	}

	if !isProcessRunning("otelcollector") {
		status = http.StatusServiceUnavailable
		message += "\nOpenTelemetryCollector is not running."
	}

	if hasConfigChanged("/opt/inotifyoutput.txt") {
		status = http.StatusServiceUnavailable
		message += "\ninotifyoutput.txt has been updated - config changed"
	}

	if hasConfigChanged("/opt/inotifyoutput-mdsd-config.txt") {
		status = http.StatusServiceUnavailable
		message += "\ninotifyoutput-mdsd-config.txt has been updated - mdsd config changed"
	}

	w.WriteHeader(status)
	fmt.Fprintln(w, message)
	if (status != http.StatusOK) {
		fmt.Printf(message)
	}
}
