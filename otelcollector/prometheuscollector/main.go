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
	"io"
	"bufio"
	"strconv"
	"path/filepath"
)

func main(){
	mac := os.Getenv("MAC")
    controllerType := os.Getenv("CONTROLLER_TYPE")
	clusterOverride := os.Getenv("CLUSTER_OVERRIDE")
	cluster := os.Getenv("CLUSTER")
	aksRegion := os.Getenv("AKSREGION")
	ccpMetricsEnabled := os.Getenv("CCP_METRICS_ENABLED")

	// wait for configmap sync container to finish initialization
	waitForConfigmapSyncContainer()

	outputFile := "/opt/inotifyoutput.txt"
	err := monitorInotify(outputFile)
	if err != nil {
		log.Fatal(err)
	}

	if ccpMetricsEnabled == "true" {
		confgimapparserforccp()
	} else {
		configmapparser()
	}

	var meConfigFile string

	if controllerType == "replicaset" {
		if clusterOverride == "true" {
			meConfigFile = "/usr/sbin/me_internal.config"
		} else {
			meConfigFile = "/usr/sbin/me.config"
		}
	} else {
		// If controllerType is not "replicaset," exit the program with a status code of 1 and a failure message.
		println("Failed: controllerType is not 'replicaset'")
		os.Exit(1)
		// if clusterOverride == "true" {
		// 	meConfigFile = "/usr/sbin/me_ds_internal.config"
		// } else {
		// 	meConfigFile = "/usr/sbin/me_ds.config"
		// }
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

	fmt.Println("Waiting for 10s for token adapter sidecar to be up and running so that it can start serving IMDS requests")
	time.Sleep(10 * time.Second)
	
	fmt.Println("Starting MDSD")
	startCommand("/usr/sbin/mdsd", "-a", "-A", "-e", "/opt/microsoft/linuxmonagent/mdsd.err", "-w", "/opt/microsoft/linuxmonagent/mdsd.warn", "-o", "/opt/microsoft/linuxmonagent/mdsd.info", "-q", "/opt/microsoft/linuxmonagent/mdsd.qos")
	
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

	// cmd := exec.Command("/opt/microsoft/otelcollector/otelcollector", "--config", collectorConfig)
	// err = cmd.Start()
	// if err != nil {
	// 	fmt.Printf("Error starting otelcollector: %v\n", err)
	// }
	fmt.Println("startCommand otelcollector")
	startCommand("/opt/microsoft/otelcollector/otelcollector", "--config", collectorConfig)

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

		// // Wait for the inotify process to finish (which won't happen as it's running as a daemon)
		// err = inotifyCommand.Wait()
		// if err != nil {
		// 	log.Fatalf("Error waiting for inotify process: %v\n", err)
		// }
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

func isProcessRunning(processName string) bool {
    // List all processes in the current process group
    pid := os.Getpid()
    processes, err := os.ReadDir("/proc")
    if err != nil {
        fmt.Println("Error:", err)
        return false
    }

    for _, processDir := range processes {
        if processDir.IsDir() {
            processID := processDir.Name()
            _, err := os.Stat("/proc/" + processID + "/cmdline")
            if err == nil {
                cmdline, err := os.ReadFile("/proc/" + processID + "/cmdline")
                if err == nil {
                    if strings.Contains(string(cmdline), processName) {
                        // Skip the current process (this program)
                        if processID != fmt.Sprintf("%d", pid) {
                            return true
                        }
                    }
                }
            }
        }
    }

    return false
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

func startCommand(command string, args ...string) {
	cmd := exec.Command(command, args...)

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
}

func startCommandAndWait(command string, args ...string) {
	cmd := exec.Command(command, args...)

	// Set environment variables from os.Environ()
	// cmd.Env = append(os.Environ())
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
	}
}

func printMdsdVersion() {
	cmd := exec.Command("mdsd", "--version")
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		fmt.Printf("Error getting MDSD version: %v\n", err)
		return
	}
	fmt.Print("MDSD_VERSION=" + string(output))
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
	cmd := exec.Command("/usr/sbin/MetricsExtension", "-Logger", "File", "-LogLevel", "Info", "-LocalControlChannel", "-TokenSource", "AMCS", "-DataDirectory", "/etc/mdsd.d/config-cache/metricsextension", "-Input", "otlp_grpc_prom", "-ConfigOverridesFilePath", "/usr/sbin/me.config")
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
        fmt.Printf("Error starting MetricsExtension: %v\n", err)
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

// existsAndNotEmpty checks if a file exists and is not empty.
func existsAndNotEmpty(filename string) bool {
    info, err := os.Stat(filename)
    if os.IsNotExist(err) {
        return false
    }
    if err != nil {
        // Handle the error, e.g., log it or return false
        return false
    }
    if info.Size() == 0 {
        return false
    }
    return true
}

// readAndTrim reads the content of a file and trims leading and trailing spaces.
func readAndTrim(filename string) (string, error) {
    content, err := ioutil.ReadFile(filename)
    if err != nil {
        return "", err
    }
    trimmedContent := strings.TrimSpace(string(content))
    return trimmedContent, nil
}

func exists(path string) bool {
    _, err := os.Stat(path)
    if err != nil {
        if os.IsNotExist(err) {
            return false
        }
    }
    return true
}

func copyFile(sourcePath, destinationPath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destinationFile, err := os.Create(destinationPath)
	if err != nil {
		return err
	}
	defer destinationFile.Close()

	_, err = io.Copy(destinationFile, sourceFile)
	if err != nil {
		return err
	}

	return nil
}

func setEnvVarsFromFile(filename string) error {
	// Check if the file exists
	_, e := os.Stat(filename)
	if os.IsNotExist(e) {
		return fmt.Errorf("File does not exist: %s", filename)
	}
	// Open the file for reading
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, "=")
		if len(parts) != 2 {
			fmt.Printf("Skipping invalid line: %s\n", line)
			continue
		}

		key := parts[0]
		value := parts[1]

		// Set the environment variable
		err := os.Setenv(key, value)
		if err != nil {
			return err
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}

func configmapparser(){
	fmt.Printf("in confgimapparser")
	configVersionPath := "/etc/config/settings/config-version"
	configSchemaPath := "/etc/config/settings/schema-version"
	// Set agent config schema version
    if existsAndNotEmpty("/etc/config/settings/schema-version") {
        configVersion, err := readAndTrim(configVersionPath)
		if err != nil {
			fmt.Println("Error reading config version file:", err)
			return
		}
		// Remove all spaces and take the first 10 characters
		configVersion = strings.ReplaceAll(configVersion, " ", "")
		if len(configVersion) >= 10 {
			configVersion = configVersion[:10]
		}
		// Set the environment variable
		os.Setenv("AZMON_AGENT_CFG_FILE_VERSION", configVersion)
    }

    // Set agent config file version
    if existsAndNotEmpty("/etc/config/settings/config-version") {
        configSchemaVersion, err := readAndTrim(configSchemaPath)
		if err != nil {
			fmt.Println("Error reading config schema version file:", err)
			return
		}
		// Remove all spaces and take the first 10 characters
		configSchemaVersion = strings.ReplaceAll(configSchemaVersion, " ", "")
		if len(configSchemaVersion) >= 10 {
			configSchemaVersion = configSchemaVersion[:10]
		}
		// Set the environment variable
		os.Setenv("AZMON_AGENT_CFG_SCHEMA_VERSION", configSchemaVersion)
    }

	// Parse the settings for pod annotations
	startCommandAndWait("ruby", "/opt/microsoft/configmapparser/tomlparser-pod-annotation-based-scraping.rb")
	// sets env : AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX in /opt/microsoft/configmapparser/config_def_pod_annotation_based_scraping
	
	filename := "/opt/microsoft/configmapparser/config_def_pod_annotation_based_scraping"
	err := setEnvVarsFromFile(filename)
	if err != nil {
		fmt.Printf("Error when settinng env for /opt/microsoft/configmapparser/config_def_pod_annotation_based_scraping: %v\n", err)
	}

	// Parse the configmap to set the right environment variables for prometheus collector settings
	startCommandAndWait("ruby", "/opt/microsoft/configmapparser/tomlparser-prometheus-collector-settings.rb")
	// sets env : AZMON_DEFAULT_METRIC_ACCOUNT_NAME, AZMON_CLUSTER_LABEL, AZMON_CLUSTER_ALIAS, AZMON_OPERATOR_ENABLED_CHART_SETTING in /opt/microsoft/configmapparser/config_prometheus_collector_settings_env_var
	filename = "/opt/microsoft/configmapparser/config_prometheus_collector_settings_env_var"
	err = setEnvVarsFromFile(filename)
	if err != nil {
		fmt.Printf("Error when settinng env for /opt/microsoft/configmapparser/config_prometheus_collector_settings_env_var: %v\n", err)
	}

	// Parse the settings for default scrape configs
	startCommandAndWait("ruby", "/opt/microsoft/configmapparser/tomlparser-default-scrape-settings.rb")
	// sets env: AZMON_PROMETHEUS_KUBELET_SCRAPING_ENABLED...AZMON_PROMETHEUS_POD_ANNOTATION_SCRAPING_ENABLED in /opt/microsoft/configmapparser/config_default_scrape_settings_env_var
	filename = "/opt/microsoft/configmapparser/config_default_scrape_settings_env_var"
	err = setEnvVarsFromFile(filename)
	if err != nil {
		fmt.Printf("Error when settinng env for /opt/microsoft/configmapparser/config_default_scrape_settings_env_var: %v\n", err)
	}

	// Parse the settings for debug mode
	startCommandAndWait("ruby", "/opt/microsoft/configmapparser/tomlparser-debug-mode.rb")
	// sets env: DEBUG_MODE_ENABLED in /opt/microsoft/configmapparser/config_debug_mode_env_var
	filename = "/opt/microsoft/configmapparser/config_debug_mode_env_var"
	err = setEnvVarsFromFile(filename)
	if err != nil {
		fmt.Printf("Error when settinng env for /opt/microsoft/configmapparser/config_debug_mode_env_var: %v\n", err)
	}

	// Parse the settings for default targets metrics keep list config
    startCommandAndWait("ruby", "/opt/microsoft/configmapparser/tomlparser-default-targets-metrics-keep-list.rb")
	// sets regexhas file /opt/microsoft/configmapparser/config_def_targets_metrics_keep_list_hash

	// Parse the settings for default-targets-scrape-interval-settings config
    startCommandAndWait("ruby", "/opt/microsoft/configmapparser/tomlparser-scrape-interval.rb")

    // Merge default and custom prometheus config
    if os.Getenv("AZMON_OPERATOR_ENABLED") == "true" || os.Getenv("CONTAINER_TYPE") == "ConfigReaderSidecar" {
        startCommandAndWait("ruby", "/opt/microsoft/configmapparser/prometheus-config-merger-with-operator.rb")
    } else {
        startCommandAndWait("ruby", "/opt/microsoft/configmapparser/prometheus-config-merger.rb")
    }

	os.Setenv("AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG", "false")
	os.Setenv("CONFIG_VALIDATOR_RUNNING_IN_AGENT", "true")

	if exists("/opt/promMergedConfig.yml") {
        startCommandAndWait("/opt/promconfigvalidator", "--config", "/opt/promMergedConfig.yml", "--output", "/opt/microsoft/otelcollector/collector-config.yml", "--otelTemplate", "/opt/microsoft/otelcollector/collector-config-template.yml")
        if !exists("/opt/microsoft/otelcollector/collector-config.yml") {
			// if cmd.ProcessState.ExitCode() != 0 || !exists("/opt/microsoft/otelcollector/collector-config.yml") {
            // Handle validation failure
            os.Setenv("AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG", "true")

            if exists("/opt/defaultsMergedConfig.yml") {
                startCommandAndWait("/opt/promconfigvalidator", "--config", "/opt/defaultsMergedConfig.yml", "--output", "/opt/collector-config-with-defaults.yml", "--otelTemplate", "/opt/microsoft/otelcollector/collector-config-template.yml")
                if !exists("/opt/collector-config-with-defaults.yml") {
					// if cmd.ProcessState.ExitCode() != 0 || !exists("/opt/collector-config-with-defaults.yml") {
                    // Handle default scrape config validation failure
                } else {
                    // Copy the validated default config
                    // Handle success
					sourcePath := "/opt/collector-config-with-defaults.yml"
					destinationPath := "/opt/microsoft/otelcollector/collector-config-default.yml"

					err := copyFile(sourcePath, destinationPath)
					if err != nil {
						fmt.Printf("Error copying file: %v\n", err)
					} else {
						fmt.Println("File copied successfully.")
					}
                }
            }

            os.Setenv("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG", "true")
        }
    } else if exists("/opt/defaultsMergedConfig.yml") {
        // Handle case where no custom config is found
        // Handle default scrape config validation
        // Copy the validated default config
        os.Setenv("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG", "true")
		startCommandAndWait("/opt/promconfigvalidator", "--config", "/opt/defaultsMergedConfig.yml", "--output", "/opt/collector-config-with-defaults.yml", "--otelTemplate", "/opt/microsoft/otelcollector/collector-config-template.yml")
		if !exists("/opt/collector-config-with-defaults.yml") {
			fmt.Printf("prom-config-validator::Prometheus default scrape config validation failed. No scrape configs will be used")
		} else {
			sourcePath := "/opt/collector-config-with-defaults.yml"
			destinationPath := "/opt/microsoft/otelcollector/collector-config-default.yml"
			err := copyFile(sourcePath, destinationPath)
					if err != nil {
						fmt.Printf("Error copying file: %v\n", err)
					} else {
						fmt.Println("File copied successfully.")
					}
		}
    } else {
        // Handle case where no custom config or default configs are enabled
		os.Setenv("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG", "true")
    }

}

func confgimapparserforccp() {
	fmt.Printf("in confgimapparserforccp")
	configVersionPath := "/etc/config/settings/config-version"
	configSchemaPath := "/etc/config/settings/schema-version"
	// Set agent config schema version
    if existsAndNotEmpty("/etc/config/settings/schema-version") {
        configVersion, err := readAndTrim(configVersionPath)
		if err != nil {
			fmt.Println("Error reading config version file:", err)
			return
		}
		// Remove all spaces and take the first 10 characters
		configVersion = strings.ReplaceAll(configVersion, " ", "")
		if len(configVersion) >= 10 {
			configVersion = configVersion[:10]
		}
		// Set the environment variable
		os.Setenv("AZMON_AGENT_CFG_FILE_VERSION", configVersion)
    }

    // Set agent config file version
    if existsAndNotEmpty("/etc/config/settings/config-version") {
        configSchemaVersion, err := readAndTrim(configSchemaPath)
		if err != nil {
			fmt.Println("Error reading config schema version file:", err)
			return
		}
		// Remove all spaces and take the first 10 characters
		configSchemaVersion = strings.ReplaceAll(configSchemaVersion, " ", "")
		if len(configSchemaVersion) >= 10 {
			configSchemaVersion = configSchemaVersion[:10]
		}
		// Set the environment variable
		os.Setenv("AZMON_AGENT_CFG_SCHEMA_VERSION", configSchemaVersion)
    }

	// Parse the configmap to set the right environment variables for prometheus collector settings
	startCommandAndWait("ruby", "/opt/microsoft/configmapparser/tomlparser-prometheus-collector-settings.rb")
	// sets env : AZMON_DEFAULT_METRIC_ACCOUNT_NAME, AZMON_CLUSTER_LABEL, AZMON_CLUSTER_ALIAS, AZMON_OPERATOR_ENABLED_CHART_SETTING in /opt/microsoft/configmapparser/config_prometheus_collector_settings_env_var
	filename := "/opt/microsoft/configmapparser/config_prometheus_collector_settings_env_var"
	err := setEnvVarsFromFile(filename)
	if err != nil {
		fmt.Printf("Error when settinng env for /opt/microsoft/configmapparser/config_prometheus_collector_settings_env_var: %v\n", err)
	}

	// Parse the settings for default scrape configs
	startCommandAndWait("ruby", "/opt/microsoft/configmapparser/tomlparser-ccp-default-scrape-settings.rb")
	// sets env: AZMON_PROMETHEUS_KUBELET_SCRAPING_ENABLED...AZMON_PROMETHEUS_POD_ANNOTATION_SCRAPING_ENABLED in /opt/microsoft/configmapparser/config_default_scrape_settings_env_var
	filename = "/opt/microsoft/configmapparser/config_default_scrape_settings_env_var"
	err = setEnvVarsFromFile(filename)
	if err != nil {
		fmt.Printf("Error when settinng env for /opt/microsoft/configmapparser/config_default_scrape_settings_env_var: %v\n", err)
	}

	// Parse the settings for default targets metrics keep list config
    startCommandAndWait("ruby", "/opt/microsoft/configmapparser/tomlparser-ccp-default-targets-metrics-keep-list.rb")
	// sets regexhas file /opt/microsoft/configmapparser/config_def_targets_metrics_keep_list_hash

	startCommandAndWait("ruby", "/opt/microsoft/configmapparser/prometheus-ccp-config-merger.rb")

	os.Setenv("AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG", "false")
	os.Setenv("CONFIG_VALIDATOR_RUNNING_IN_AGENT", "true")

	// No need to merge custom prometheus config, only merging in the default configs
	os.Setenv("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG", "true")
		startCommandAndWait("/opt/promconfigvalidator", "--config", "/opt/defaultsMergedConfig.yml", "--output", "/opt/collector-config-with-defaults.yml", "--otelTemplate", "/opt/microsoft/otelcollector/collector-config-template.yml")
		if !exists("/opt/collector-config-with-defaults.yml") {
			fmt.Printf("prom-config-validator::Prometheus default scrape config validation failed. No scrape configs will be used")
		} else {
			sourcePath := "/opt/collector-config-with-defaults.yml"
			destinationPath := "/opt/microsoft/otelcollector/collector-config-default.yml"
			err := copyFile(sourcePath, destinationPath)
					if err != nil {
						fmt.Printf("Error copying file: %v\n", err)
					} else {
						fmt.Println("File copied successfully.")
					}
		}
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

func healthHandler(w http.ResponseWriter, r *http.Request) {
	status := http.StatusOK
	message := "prometheuscollector is running."
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
}

func monitorInotify(outputFile string) error {
	// Start inotify to watch for changes
	fmt.Println("Starting inotify for watching config map update")

	_, err := os.Create(outputFile)
	if err != nil {
		log.Fatalf("Error creating output file: %v\n", err)
	}

	// Define the command to start inotify
	inotifyCommand := exec.Command(
		"inotifywait",
		"/etc/config/settings",
		"--daemon",
		"--recursive",
		"--outfile", outputFile,
		"--event", "create,delete",
		"--format", "%e : %T",
		"--timefmt", "+%s",
	)

	// Start the inotify process
	err = inotifyCommand.Start()
	if err != nil {
		log.Fatalf("Error starting inotify process: %v\n", err)
	}

	return nil
}

func waitForFileCreation(directory, targetFile string) (string, error) {
	for {
		dir, err := os.Open(directory)
		if err != nil {
			return "", err
		}
		defer dir.Close()

		files, err := dir.Readdir(0)
		if err != nil {
			return "", err
		}

		for _, file := range files {
			if file.Name() == targetFile {
				return file.Name(), nil
			}
		}

		time.Sleep(time.Second) // Sleep for a second before checking again
	}
}

func waitForConfigmapSyncContainer() {
	settingsChangedFile := "/etc/config/settings/inotifysettingscreated"
	ccpMetricsEnabled := os.Getenv("CCP_METRICS_ENABLED")
	if ccpMetricsEnabled == "true" {
		_, err := os.Stat(settingsChangedFile)
		if os.IsNotExist(err) {
			// Disable appinsights telemetry for ccp metrics
			os.Setenv("DISABLE_TELEMETRY", "true")

			_, err := os.Stat(settingsChangedFile)
			if os.IsNotExist(err) {
				fmt.Println("Waiting for ama-metrics-config-sync container to finish initialization...")

				for {
					event, err := waitForFileCreation(filepath.Dir(settingsChangedFile), filepath.Base(settingsChangedFile))
					if err != nil {
						fmt.Println(err)
						break
					}
					if event == filepath.Base(settingsChangedFile) {
						break
					}
				}
			}
		}
	}
}
