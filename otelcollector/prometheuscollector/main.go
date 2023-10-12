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
)

func main() {
	mac := os.Getenv("MAC")
    controllerType := os.Getenv("controllerType")
	clusterOverride := os.Getenv("CLUSTER_OVERRIDE")
	cluster := os.Getenv("CLUSTER")
	aksRegion := os.Getenv("AKSREGION")

	var meConfigFile string

	if controllerType == "replicaset" {
		if clusterOverride == "true" {
			meConfigFile = "/usr/sbin/me_internal.config"
		} else {
			meConfigFile = "/usr/sbin/me.config"
		}
	} else {
		if clusterOverride == "true" {
			meConfigFile = "/usr/sbin/me_ds_internal.config"
		} else {
			meConfigFile = "/usr/sbin/me_ds.config"
		}
	}
	fmt.Println("meConfigFile:", meConfigFile)

    if mac == "true" {
		// Wait for addon-token-adapter to be healthy
		tokenAdapterWaitSecs := 60
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

	fmt.Println("Waiting for 120s for token adapter sidecar to be up and running so that it can start serving IMDS requests")
	time.Sleep(120 * time.Second)
	
	fmt.Println("Starting MDSD")
	startMdsd()
	
	fmt.Print("MDSD_VERSION=")
	printMdsdVersion()
	
	fmt.Println("Waiting for 30s for MDSD to get the config and put them in place for ME")
	time.Sleep(30 * time.Second)
	
	// fmt.Println("Reading me config file as a string for configOverrides parameter")
	// meConfigString := readMeConfigFileAsString(meConfigFile)
	
	fmt.Println("Starting metricsextension with config overrides")
	startMetricsExtensionWithConfigOverrides(meConfigFile)

    // Get ME version
	meVersion, err := readVersionFile("/opt/metricsextversion.txt")
	if err != nil {
		fmt.Printf("Error reading ME version file: %v\n", err)
	} else {
		fmtVar("ME_VERSION", meVersion)
	}

	// Get Ruby version
	rubyVersion, err := exec.Command("ruby", "--version").Output()
	if err != nil {
		fmt.Printf("Error getting Ruby version: %v\n", err)
	} else {
		fmtVar("RUBY_VERSION", string(rubyVersion))
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
		// Commenting this out since config can be applied via CRD
		// fmt.Println("Starting otelcollector with only default scrape configs enabled")
		collectorConfig = "/opt/microsoft/otelcollector/collector-config-default.yml"
	} else {
		fmt.Println("Starting otelcollector")
		collectorConfig = "/opt/microsoft/otelcollector/collector-config.yml"
	}

	cmd := exec.Command("/opt/microsoft/otelcollector/otelcollector", "--config", collectorConfig)
	err = cmd.Start()
	if err != nil {
		fmt.Printf("Error starting otelcollector: %v\n", err)
	}

	otelCollectorVersion, err := exec.Command("/opt/microsoft/otelcollector/otelcollector", "--version").Output()
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

		// Wait for the inotify process to finish (which won't happen as it's running as a daemon)
		err = inotifyCommand.Wait()
		if err != nil {
			log.Fatalf("Error waiting for inotify process: %v\n", err)
		}
	}

    // Expose a health endpoint for liveness probe
    http.HandleFunc("/health", healthHandler)
    http.ListenAndServe(":8080", nil)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
    prometheuscollectorRunning := isProcessRunning("prometheuscollector")

    if prometheuscollectorRunning {
        w.WriteHeader(http.StatusOK)
        fmt.Fprintln(w, "prometheuscollector is running.")
    } else {
        w.WriteHeader(http.StatusServiceUnavailable)
        fmt.Fprintln(w, "prometheuscollector is not running.")
    }
}

func isProcessRunning(processName string) bool {
    cmd := exec.Command("pgrep", processName)
    err := cmd.Run()
    return err == nil
}

func readEnvVarsFromEnvMdsdFile(envMdsdFile string) ([]string, error) {
	content, err := ioutil.ReadFile(envMdsdFile)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	var envVars []string
	for _, line := range lines {
		// Trim any leading/trailing spaces and ignore empty lines
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			name, value := parts[0], parts[1]
			envVars = append(envVars, name+"="+value)
		}
	}

	return envVars, nil
}
func startMdsd() {
	cmd := exec.Command("/usr/sbin/mdsd", "-a", "-A", "-e", "/opt/microsoft/linuxmonagent/mdsd.err", "-w", "/opt/microsoft/linuxmonagent/mdsd.warn", "-o", "/opt/microsoft/linuxmonagent/mdsd.info", "-q", "/opt/microsoft/linuxmonagent/mdsd.qos")
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
        fmt.Printf("Error starting MDSD: %v\n", err)
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

}

func printMdsdVersion() {
	cmd := exec.Command("mdsd", "--version")
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		fmt.Printf("Error getting MDSD version: %v\n", err)
		return
	}
	fmt.Print(string(output))
}

func readMeConfigFileAsString(meConfigFile string) string {
	content, err := ioutil.ReadFile(meConfigFile)
	if err != nil {
		fmt.Printf("Error reading ME config file: %v\n", err)
		return ""
	}
	return string(content)
}

func startMetricsExtensionWithConfigOverrides(configOverrides string) {
	cmd := exec.Command("/usr/sbin/MetricsExtension", "-Logger File -LogLevel Info -LocalControlChannel -TokenSource AMCS -DataDirectory /etc/mdsd.d/config-cache/metricsextension -Input otlp_grpc_prom -ConfigOverridesFilePath /usr/sbin/me.config")
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "customResourceId=" + os.Getenv("CLUSTER"))
	cmd.Env = append(cmd.Env, "customRegion=" + os.Getenv("AKSREGION"))
	fmt.Println("cmd.Env for MetricsExtension")
	fmt.Println(cmd.Env)
	err := cmd.Start()
	if err != nil {
		fmt.Printf("Error starting MetricsExtension with configOverrides: %v\n", err)
	}
}

func readVersionFile(filePath string) (string, error) {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func fmtVar(name, value string) {
	fmt.Printf("%s=\"%s\"\n", name, value)
}
