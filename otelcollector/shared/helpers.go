package shared

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func GetEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func GetControllerType() string {
	// Get CONTROLLER_TYPE environment variable
	controllerType := os.Getenv("CONTROLLER_TYPE")

	// Convert controllerType to lowercase and trim spaces
	controllerTypeLower := strings.ToLower(strings.TrimSpace(controllerType))

	return controllerTypeLower
}

func IsValidRegex(input string) bool {
	_, err := regexp.Compile(input)
	return err == nil
}

func DetermineConfigFiles(controllerType, clusterOverride string) (string, string) {
	var meConfigFile, fluentBitConfigFile string

	switch {
	case strings.ToLower(controllerType) == "replicaset":
		fluentBitConfigFile = "/opt/fluent-bit/fluent-bit.yaml"
		if clusterOverride == "true" {
			meConfigFile = "/usr/sbin/me_internal.config"
		} else {
			meConfigFile = "/usr/sbin/me.config"
		}
	case os.Getenv("OS_TYPE") != "windows":
		fluentBitConfigFile = "/opt/fluent-bit/fluent-bit-daemonset.yaml"
		if clusterOverride == "true" {
			meConfigFile = "/usr/sbin/me_ds_internal.config"
		} else {
			meConfigFile = "/usr/sbin/me_ds.config"
		}
	default:
		fluentBitConfigFile = "/opt/fluent-bit/fluent-bit-windows.conf"
		if clusterOverride == "true" {
			meConfigFile = "/opt/metricextension/me_ds_internal_win.config"
		} else {
			meConfigFile = "/opt/metricextension/me_ds_win.config"
		}
	}

	return meConfigFile, fluentBitConfigFile
}

func LogVersionInfo() {
	if meVersion, err := ReadVersionFile("/opt/metricsextversion.txt"); err == nil {
		FmtVar("ME_VERSION", meVersion)
	} else {
		log.Printf("Error reading ME version file: %v\n", err)
	}

	if golangVersion, err := ReadVersionFile("/opt/goversion.txt"); err == nil {
		FmtVar("GOLANG_VERSION", golangVersion)
	} else {
		log.Printf("Error reading Golang version file: %v\n", err)
	}

	if otelCollectorVersion, err := exec.Command("/opt/microsoft/otelcollector/otelcollector", "--version").Output(); err == nil {
		FmtVar("OTELCOLLECTOR_VERSION", string(otelCollectorVersion))
	} else {
		log.Printf("Error getting otelcollector version: %v\n", err)
	}

	if prometheusVersion, err := ReadVersionFile("/opt/microsoft/otelcollector/PROMETHEUS_VERSION"); err == nil {
		FmtVar("PROMETHEUS_VERSION", prometheusVersion)
	} else {
		log.Printf("Error reading Prometheus version file: %v\n", err)
	}
}

func StartTelegraf() {
	fmt.Println("Starting Telegraf")

	if telemetryDisabled := os.Getenv("TELEMETRY_DISABLED"); telemetryDisabled != "true" {
		if os.Getenv("OS_TYPE") == "linux" {
			controllerType := os.Getenv("CONTROLLER_TYPE")
			azmonOperatorEnabled := os.Getenv("AZMON_OPERATOR_ENABLED")

			var telegrafConfig string

			switch {
			case controllerType == "ReplicaSet" && azmonOperatorEnabled == "true":
				telegrafConfig = "/opt/telegraf/telegraf-prometheus-collector-ta-enabled.conf"
			case controllerType == "ReplicaSet":
				telegrafConfig = "/opt/telegraf/telegraf-prometheus-collector.conf"
			default:
				telegrafConfig = "/opt/telegraf/telegraf-prometheus-collector-ds.conf"
			}

			telegrafCmd := exec.Command("/usr/bin/telegraf", "--config", telegrafConfig)
			telegrafCmd.Stdout = os.Stdout
			telegrafCmd.Stderr = os.Stderr
			if err := telegrafCmd.Start(); err != nil {
				fmt.Println("Error starting telegraf:", err)
				return
			}

			telegrafVersion, _ := os.ReadFile("/opt/telegrafversion.txt")
			fmt.Printf("TELEGRAF_VERSION=%s\n", string(telegrafVersion))
		}
	} else {
		telegrafPath := "C:\\opt\\telegraf\\telegraf.exe"
		configPath := "C:\\opt\\telegraf\\telegraf-prometheus-collector-windows.conf"

		// Install Telegraf service
		installCmd := exec.Command(telegrafPath, "--service", "install", "--config", configPath)
		if err := installCmd.Run(); err != nil {
			log.Fatalf("Error installing Telegraf service: %v\n", err)
		}

		// Set delayed start if POD_NAME is set
		serverName := os.Getenv("POD_NAME")
		if serverName != "" {
			setDelayCmd := exec.Command("sc.exe", fmt.Sprintf("\\\\%s", serverName), "config", "telegraf", "start= delayed-auto")
			if err := setDelayCmd.Run(); err != nil {
				log.Printf("Failed to set delayed start for Telegraf: %v\n", err)
			} else {
				fmt.Println("Successfully set delayed start for Telegraf")
			}
		} else {
			fmt.Println("Failed to get environment variable POD_NAME to set delayed Telegraf start")
		}

		// Run Telegraf in test mode
		testCmd := exec.Command(telegrafPath, "--config", configPath, "--test")
		testCmd.Stdout = os.Stdout
		testCmd.Stderr = os.Stderr
		if err := testCmd.Run(); err != nil {
			log.Printf("Error running Telegraf in test mode: %v\n", err)
		}

		// Start Telegraf service
		startCmd := exec.Command(telegrafPath, "--service", "start")
		if err := startCmd.Run(); err != nil {
			log.Printf("Error starting Telegraf service: %v\n", err)
		}

		// Check if Telegraf is running, retry if necessary
		for {
			statusCmd := exec.Command("sc.exe", "query", "telegraf")
			output, err := statusCmd.CombinedOutput()
			if err != nil {
				log.Printf("Error checking Telegraf service status: %v\n", err)
				time.Sleep(30 * time.Second)
				continue
			}

			if string(output) != "" {
				fmt.Println("Telegraf is running")
				break
			}

			fmt.Println("Trying to start Telegraf again in 30 seconds, since it might not have been ready...")
			time.Sleep(30 * time.Second)
			startCmd := exec.Command(telegrafPath, "--service", "start")
			if err := startCmd.Run(); err != nil {
				log.Printf("Error starting Telegraf service again: %v\n", err)
			}
		}
	}
}

func SetEnvVariablesForWindows() {
	// Set Windows version (Microsoft Windows Server 2019 Datacenter or 2022 Datacenter)
	out, err := exec.Command("wmic", "os", "get", "Caption").Output()
	if err != nil {
		log.Fatalf("Failed to get Windows version: %v", err)
	}
	windowsVersion := strings.TrimSpace(string(out))
	windowsVersion = strings.Split(windowsVersion, "\n")[1] // Extract version name

	// Set environment variables for process and machine
	os.Setenv("windowsVersion", windowsVersion)
	SetEnvAndSourceBashrcOrPowershell("windowsVersion", windowsVersion, true)

	// Resource ID override
	mac := os.Getenv("MAC")
	cluster := os.Getenv("CLUSTER")
	nodeName := os.Getenv("NODE_NAME")
	if mac == "" {
		if cluster == "" {
			fmt.Printf("CLUSTER is empty or not set. Using %s as CLUSTER\n", nodeName)
			os.Setenv("customResourceId", nodeName)
			SetEnvAndSourceBashrcOrPowershell("customResourceId", nodeName, true)
		} else {
			os.Setenv("customResourceId", cluster)
			SetEnvAndSourceBashrcOrPowershell("customResourceId", cluster, true)
		}
	} else {
		SetEnvAndSourceBashrcOrPowershell("customResourceId", cluster, true)

		aksRegion := os.Getenv("AKSREGION")
		SetEnvAndSourceBashrcOrPowershell("customRegion", aksRegion, true)

		// Set variables for Telegraf
		SetTelegrafVariables(aksRegion, cluster)
	}

	// Set monitoring-related variables
	SetMonitoringVariables()

	// Handle custom environment settings
	customEnvironment := strings.ToLower(os.Getenv("customEnvironment"))
	mcsEndpoint, mcsGlobalEndpoint := GetMcsEndpoints(customEnvironment)

	// Set MCS endpoint environment variables
	SetEnvAndSourceBashrcOrPowershell("MCS_AZURE_RESOURCE_ENDPOINT", mcsEndpoint, true)
	SetEnvAndSourceBashrcOrPowershell("MCS_GLOBAL_ENDPOINT", mcsGlobalEndpoint, true)
}

func SetTelegrafVariables(aksRegion, cluster string) {
	SetEnvAndSourceBashrcOrPowershell("AKSREGION", aksRegion, true)
	SetEnvAndSourceBashrcOrPowershell("CLUSTER", cluster, true)
	azmonClusterAlias := os.Getenv("AZMON_CLUSTER_ALIAS")
	SetEnvAndSourceBashrcOrPowershell("AZMON_CLUSTER_ALIAS", azmonClusterAlias, true)
}

func SetMonitoringVariables() {
	SetEnvAndSourceBashrcOrPowershell("MONITORING_ROLE_INSTANCE", "cloudAgentRoleInstanceIdentity", true)
	SetEnvAndSourceBashrcOrPowershell("MA_RoleEnvironment_OsType", "Windows", true)
	SetEnvAndSourceBashrcOrPowershell("MONITORING_VERSION", "2.0", true)
	SetEnvAndSourceBashrcOrPowershell("MONITORING_ROLE", "cloudAgentRoleIdentity", true)
	SetEnvAndSourceBashrcOrPowershell("MONITORING_IDENTITY", "use_ip_address", true)
	SetEnvAndSourceBashrcOrPowershell("MONITORING_USE_GENEVA_CONFIG_SERVICE", "false", true)
	SetEnvAndSourceBashrcOrPowershell("SKIP_IMDS_LOOKUP_FOR_LEGACY_AUTH", "true", true)
	SetEnvAndSourceBashrcOrPowershell("ENABLE_MCS", "true", true)
	SetEnvAndSourceBashrcOrPowershell("MDSD_USE_LOCAL_PERSISTENCY", "false", true)
	SetEnvAndSourceBashrcOrPowershell("MA_RoleEnvironment_Location", os.Getenv("AKSREGION"), true)
	SetEnvAndSourceBashrcOrPowershell("MA_RoleEnvironment_ResourceId", os.Getenv("CLUSTER"), true)
	SetEnvAndSourceBashrcOrPowershell("MCS_CUSTOM_RESOURCE_ID", os.Getenv("CLUSTER"), true)
}

func GetMcsEndpoints(customEnvironment string) (string, string) {
	var mcsEndpoint, mcsGlobalEndpoint string

	switch customEnvironment {
	case "azurepubliccloud":
		aksRegion := strings.ToLower(os.Getenv("AKSREGION"))
		if aksRegion == "eastus2euap" || aksRegion == "centraluseuap" {
			mcsEndpoint = "https://monitor.azure.com/"
			mcsGlobalEndpoint = "https://global.handler.canary.control.monitor.azure.com"
		} else {
			mcsEndpoint = "https://monitor.azure.com/"
			mcsGlobalEndpoint = "https://global.handler.control.monitor.azure.com"
		}
	case "azureusgovernmentcloud":
		mcsEndpoint = "https://monitor.azure.us/"
		mcsGlobalEndpoint = "https://global.handler.control.monitor.azure.us"
	case "azurechinacloud":
		mcsEndpoint = "https://monitor.azure.cn/"
		mcsGlobalEndpoint = "https://global.handler.control.monitor.azure.cn"
	case "usnat":
		mcsEndpoint = "https://monitor.azure.eaglex.ic.gov/"
		mcsGlobalEndpoint = "https://global.handler.control.monitor.azure.eaglex.ic.gov"
	case "ussec":
		mcsEndpoint = "https://monitor.azure.microsoft.scloud/"
		mcsGlobalEndpoint = "https://global.handler.control.monitor.azure.microsoft.scloud/"
	default:
		fmt.Printf("Unknown customEnvironment: %s, setting mcs endpoint to default azurepubliccloud values\n", customEnvironment)
		mcsEndpoint = "https://monitor.azure.com/"
		mcsGlobalEndpoint = "https://global.handler.control.monitor.azure.com"
	}
	return mcsEndpoint, mcsGlobalEndpoint
}

// Helper function to remove surrounding quotes from a string
func RemoveQuotes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// parseConfigLines parses lines from a scanner and adds key-value pairs to the specified section
func parseConfigLines(scanner *bufio.Scanner, filePath string, currentSection string, metricsConfigBySection map[string]map[string]string) string {
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Detect section headers (supports top-level keys)
		if strings.HasSuffix(line, ": |-") {
			currentSection = strings.TrimSuffix(line, ": |-")
			if _, exists := metricsConfigBySection[currentSection]; !exists {
				metricsConfigBySection[currentSection] = make(map[string]string)
			}
			continue
		}

		// Process key-value pairs within sections
		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				log.Printf("warning: skipping malformed line in file %s: %q", filePath, line)
				continue
			}

			key := strings.TrimSpace(parts[0])
			value := RemoveQuotes(strings.TrimSpace(parts[1]))

			if key == "" {
				log.Printf("warning: skipping empty key in file %s: %q", filePath, line)
				continue
			}

			// Handle top-level keys in a separate section
			if currentSection == "" {
				currentSection = "prometheus-collector-settings"
				if _, exists := metricsConfigBySection[currentSection]; !exists {
					metricsConfigBySection[currentSection] = make(map[string]string)
				}
			}

			metricsConfigBySection[currentSection][key] = value
		}
	}

	return currentSection
}

// ParseMetricsFiles parses multiple metrics configuration files into a nested map structure.
func ParseMetricsFiles(filePaths []string) (map[string]map[string]string, error) {
	metricsConfigBySection := make(map[string]map[string]string)

	for _, filePath := range filePaths {
		file, err := os.Open(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to open file %s: %w", filePath, err)
		}
		defer func() {
			if cerr := file.Close(); cerr != nil {
				log.Printf("warning: failed to close file %s: %v", filePath, cerr)
			}
		}()

		scanner := bufio.NewScanner(file)
		parseConfigLines(scanner, filePath, "", metricsConfigBySection)

		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
		}
	}

	return metricsConfigBySection, nil
}

// ParseV1Config parses the v1 configuration from individual files into a nested map structure.
func ParseV1Config(configDir string) (map[string]map[string]string, error) {
	metricsConfigBySection := make(map[string]map[string]string)

	files, err := os.ReadDir(configDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read config directory %s: %w", configDir, err)
	}

	for _, file := range files {
		if file.IsDir() || strings.HasPrefix(file.Name(), ".") {
			continue
		}

		filePath := filepath.Join(configDir, file.Name())

		// Open the file safely with closure for proper cleanup
		f, err := os.Open(filePath)
		if err != nil {
			log.Printf("error: unable to open file %s: %v", filePath, err)
			continue
		}
		defer func() {
			if cerr := f.Close(); cerr != nil {
				log.Printf("warning: failed to close file %s: %v", filePath, cerr)
			}
		}()

		scanner := bufio.NewScanner(f)
		sectionData := make(map[string]string)
		metricsConfigBySection[file.Name()] = sectionData

		parseConfigLines(scanner, filePath, file.Name(), metricsConfigBySection)

		if err := scanner.Err(); err != nil {
			log.Printf("error: failed to read file %s: %v", filePath, err)
			continue
		}

		if cerr := f.Close(); cerr != nil {
			log.Printf("warning: failed to close file %s: %v", filePath, cerr)
		}
	}

	return metricsConfigBySection, nil
}
