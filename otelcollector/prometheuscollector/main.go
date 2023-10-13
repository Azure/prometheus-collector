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
)

func main(){
	mac := os.Getenv("MAC")
    controllerType := os.Getenv("CONTROLLER_TYPE")
	clusterOverride := os.Getenv("CLUSTER_OVERRIDE")
	cluster := os.Getenv("CLUSTER")
	aksRegion := os.Getenv("AKSREGION")

	configmapparser()

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

	fmt.Println("Waiting for 10s for token adapter sidecar to be up and running so that it can start serving IMDS requests")
	time.Sleep(10 * time.Second)
	
	fmt.Println("Starting MDSD")
	startCommand("/usr/sbin/mdsd", "-a", "-A", "-e", "/opt/microsoft/linuxmonagent/mdsd.err", "-w", "/opt/microsoft/linuxmonagent/mdsd.warn", "-o", "/opt/microsoft/linuxmonagent/mdsd.info", "-q", "/opt/microsoft/linuxmonagent/mdsd.qos")
	
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

func startCommand(command string, args ...string) {
	cmd := exec.Command(command, args...)

	// Set environment variables from os.Environ()
	cmd.Env = append(os.Environ())
	// Print the environment variables being passed into the cmd
	fmt.Println("Environment variables being passed into the command:")
	for _, v := range cmd.Env {
		fmt.Println(v)
	}
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
	// Set agent config schema version
    if existsAndNotEmpty("/etc/config/settings/schema-version") {
        configSchemaVersion, _ := readAndTrim("/etc/config/settings/schema-version")
        configSchemaVersion = strings.ReplaceAll(configSchemaVersion, " ", "")
        configSchemaVersion = configSchemaVersion[:10]
        os.Setenv("AZMON_AGENT_CFG_SCHEMA_VERSION", configSchemaVersion)
    }

    // Set agent config file version
    if existsAndNotEmpty("/etc/config/settings/config-version") {
        configFileVersion, _ := readAndTrim("/etc/config/settings/config-version")
        configFileVersion = strings.ReplaceAll(configFileVersion, " ", "")
        configFileVersion = configFileVersion[:10]
        os.Setenv("AZMON_AGENT_CFG_FILE_VERSION", configFileVersion)
    }

	// Parse the settings for pod annotations
	startCommand("ruby", "/opt/microsoft/configmapparser/tomlparser-pod-annotation-based-scraping.rb")
	// sets env : AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX in /opt/microsoft/configmapparser/config_def_pod_annotation_based_scraping
	
	filename := "/opt/microsoft/configmapparser/config_def_pod_annotation_based_scraping"
	err := setEnvVarsFromFile(filename)
	if err != nil {
		fmt.Printf("Error when settinng env for /opt/microsoft/configmapparser/config_def_pod_annotation_based_scraping: %v\n", err)
		return
	}

	// Parse the configmap to set the right environment variables for prometheus collector settings
	startCommand("ruby", "/opt/microsoft/configmapparser/tomlparser-prometheus-collector-settings.rb")
	// sets env : AZMON_DEFAULT_METRIC_ACCOUNT_NAME, AZMON_CLUSTER_LABEL, AZMON_CLUSTER_ALIAS, AZMON_OPERATOR_ENABLED_CHART_SETTING in /opt/microsoft/configmapparser/config_prometheus_collector_settings_env_var
	filename = "/opt/microsoft/configmapparser/config_prometheus_collector_settings_env_var"
	err = setEnvVarsFromFile(filename)
	if err != nil {
		fmt.Printf("Error when settinng env for /opt/microsoft/configmapparser/config_prometheus_collector_settings_env_var: %v\n", err)
		return
	}

	// Parse the settings for default scrape configs
	startCommand("ruby", "/opt/microsoft/configmapparser/tomlparser-default-scrape-settings.rb")
	// sets env: AZMON_PROMETHEUS_KUBELET_SCRAPING_ENABLED...AZMON_PROMETHEUS_POD_ANNOTATION_SCRAPING_ENABLED in /opt/microsoft/configmapparser/config_default_scrape_settings_env_var
	filename = "/opt/microsoft/configmapparser/config_default_scrape_settings_env_var"
	err = setEnvVarsFromFile(filename)
	if err != nil {
		fmt.Printf("Error when settinng env for /opt/microsoft/configmapparser/config_default_scrape_settings_env_var: %v\n", err)
		return
	}

	// Parse the settings for debug mode
	startCommand("ruby", "/opt/microsoft/configmapparser/tomlparser-debug-mode.rb")
	// sets env: DEBUG_MODE_ENABLED in /opt/microsoft/configmapparser/config_debug_mode_env_var
	filename = "/opt/microsoft/configmapparser/config_debug_mode_env_var"
	err = setEnvVarsFromFile(filename)
	if err != nil {
		fmt.Printf("Error when settinng env for /opt/microsoft/configmapparser/config_debug_mode_env_var: %v\n", err)
		return
	}

	// Parse the settings for default targets metrics keep list config
    startCommand("ruby", "/opt/microsoft/configmapparser/tomlparser-default-targets-metrics-keep-list.rb")
	// sets regexhas file /opt/microsoft/configmapparser/config_def_targets_metrics_keep_list_hash

	// Parse the settings for default-targets-scrape-interval-settings config
    startCommand("ruby", "/opt/microsoft/configmapparser/tomlparser-scrape-interval.rb")

    // Merge default and custom prometheus config
    if os.Getenv("AZMON_OPERATOR_ENABLED") == "true" || os.Getenv("CONTAINER_TYPE") == "ConfigReaderSidecar" {
        startCommand("ruby", "/opt/microsoft/configmapparser/prometheus-config-merger-with-operator.rb")
    } else {
        startCommand("ruby", "/opt/microsoft/configmapparser/prometheus-config-merger.rb")
    }

	os.Setenv("AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG", "false")
	os.Setenv("CONFIG_VALIDATOR_RUNNING_IN_AGENT", "true")

	if exists("/opt/promMergedConfig.yml") {
        startCommand("/opt/promconfigvalidator", "--config", "/opt/promMergedConfig.yml", "--output", "/opt/microsoft/otelcollector/collector-config.yml", "--otelTemplate", "/opt/microsoft/otelcollector/collector-config-template.yml")
        if !exists("/opt/microsoft/otelcollector/collector-config.yml") {
			// if cmd.ProcessState.ExitCode() != 0 || !exists("/opt/microsoft/otelcollector/collector-config.yml") {
            // Handle validation failure
            os.Setenv("AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG", "true")

            if exists("/opt/defaultsMergedConfig.yml") {
                startCommand("/opt/promconfigvalidator", "--config", "/opt/defaultsMergedConfig.yml", "--output", "/opt/collector-config-with-defaults.yml", "--otelTemplate", "/opt/microsoft/otelcollector/collector-config-template.yml")
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
    } else {
        // Handle case where no custom config or default configs are enabled
    }

}
