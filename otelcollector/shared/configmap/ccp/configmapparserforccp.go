package ccpconfigmapsettings

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	// "prometheus-collector/shared"
	"github.com/prometheus-collector/shared"
)

func Configmapparserforccp() {
	fmt.Printf("in configmapparserforccp")
	fmt.Printf("waiting for 30 secs...")
	time.Sleep(30 * time.Second) //needed to save a restart at times when config watcher sidecar starts up later than us and hence config map wasn't yet projected into emptydir volume yet during pod startups.

	configVersionPath := "/etc/config/settings/config-version"
	configSchemaPath := "/etc/config/settings/schema-version"

	entries, er := os.ReadDir("/etc/config/settings")
	if er != nil {
		fmt.Println("error listing /etc/config/settings", er)
	}

	for _, e := range entries {
		fmt.Println(e.Name())
	}

	fmt.Println("done listing /etc/config/settings")

	// Set agent config schema version
	if shared.ExistsAndNotEmpty(configSchemaPath) {
		configVersion, err := shared.ReadAndTrim(configVersionPath)
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
		fmt.Println("Configmapparserforccp setting env var AZMON_AGENT_CFG_FILE_VERSION:", configVersion)
		shared.SetEnvAndSourceBashrcOrPowershell("AZMON_AGENT_CFG_FILE_VERSION", configVersion, true)
	} else {
		fmt.Println("Configmapparserforccp fileversion file doesn't exist. or configmap doesn't exist:", configVersionPath)
	}

	// Set agent config file version
	if shared.ExistsAndNotEmpty(configVersionPath) {
		configSchemaVersion, err := shared.ReadAndTrim(configSchemaPath)
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
		fmt.Println("Configmapparserforccp setting env var AZMON_AGENT_CFG_SCHEMA_VERSION:", configSchemaVersion)
		shared.SetEnvAndSourceBashrcOrPowershell("AZMON_AGENT_CFG_SCHEMA_VERSION", configSchemaVersion, true)
	} else {
		fmt.Println("Configmapparserforccp schemaversion file doesn't exist. or configmap doesn't exist:", configSchemaPath)
	}

	////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	var parsedData map[string]map[string]string
	var err error
	if os.Getenv("AZMON_AGENT_CFG_SCHEMA_VERSION") == "v2" {
		filePaths := []string{"/etc/config/settings/controlplane-metrics", "/etc/config/settings/shared"}
		parsedData, err = ParseMetricsFiles(filePaths)
		if err != nil {
			fmt.Printf("Error parsing files: %v\n", err)
			return
		}

		// // Print the parsed data
		// fmt.Println("kubelet enabled:", parsedData["default-scrape-settings-enabled"]["kubelet"])
		// fmt.Println("kubelet keep list:", parsedData["default-targets-metrics-keep-list"]["kubelet"])
		// fmt.Println("kubelet scrape interval:", parsedData["default-targets-scrape-interval-settings"]["kubelet"])
		// fmt.Println("podannotationnamespaceregex:", parsedData["pod-annotation-based-scraping"]["podannotationnamespaceregex"])
		// fmt.Println("cluster_alias:", parsedData["prometheus-collector-settings"]["cluster_alias"])

		// Debug log: Print everything in parsedData
		fmt.Println("configmapparser::Debug: Printing everything in parsedData:")
		for section, keyValuePairs := range parsedData {
			fmt.Printf("Section: %s\n", section)
			for key, value := range keyValuePairs {
				fmt.Printf("  %s: %s\n", key, value)
			}
		}

	} else if os.Getenv("AZMON_AGENT_CFG_SCHEMA_VERSION") == "v1" {
		configDir := "/etc/config/settings"
		parsedData, err = ParseV1Config(configDir)
		if err != nil {
			fmt.Printf("Error parsing config: %v\n", err)
			return
		}

		// // Print the parsed data
		// fmt.Println("Parsed Data:")
		// for section, values := range parsedData {
		// 	fmt.Printf("Section: %s\n", section)
		// 	for key, value := range values {
		// 		fmt.Printf("  %s = %s\n", key, value)
		// 	}
		// }

		// Debug log: Print everything in parsedData
		fmt.Println("configmapparser::Debug: Printing everything in parsedData:")
		for section, keyValuePairs := range parsedData {
			fmt.Printf("Section: %s\n", section)
			for key, value := range keyValuePairs {
				fmt.Printf("  %s: %s\n", key, value)
			}
		}
	} else {
		fmt.Println("Invalid schema version. Using defaults.")
	}
	////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

	// Parse the configmap to set the right environment variables for prometheus collector settings
	parseConfigAndSetEnvInFile(parsedData)
	filename := "/opt/microsoft/configmapparser/config_prometheus_collector_settings_env_var"
	err = shared.SetEnvVarsFromFile(filename)
	if err != nil {
		fmt.Printf("Error when settinng env for /opt/microsoft/configmapparser/config_prometheus_collector_settings_env_var: %v\n", err)
	}

	// Parse the settings for default scrape configs
	tomlparserCCPDefaultScrapeSettings(parsedData)
	filename = "/opt/microsoft/configmapparser/config_default_scrape_settings_env_var"
	err = shared.SetEnvVarsFromFile(filename)
	if err != nil {
		fmt.Printf("Error when settinng env for /opt/microsoft/configmapparser/config_default_scrape_settings_env_var: %v\n", err)
	}

	// Parse the settings for default targets metrics keep list config
	tomlparserCCPTargetsMetricsKeepList(parsedData)

	prometheusCcpConfigMerger()

	shared.SetEnvAndSourceBashrcOrPowershell("AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG", "false", true)
	shared.SetEnvAndSourceBashrcOrPowershell("CONFIG_VALIDATOR_RUNNING_IN_AGENT", "true", true)

	// No need to merge custom prometheus config, only merging in the default configs
	shared.SetEnvAndSourceBashrcOrPowershell("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG", "true", true)
	shared.StartCommandAndWait("/opt/promconfigvalidator", "--config", "/opt/defaultsMergedConfig.yml", "--output", "/opt/ccp-collector-config-with-defaults.yml", "--otelTemplate", "/opt/microsoft/otelcollector/ccp-collector-config-template.yml")
	if !shared.Exists("/opt/ccp-collector-config-with-defaults.yml") {
		fmt.Printf("prom-config-validator::Prometheus default scrape config validation failed. No scrape configs will be used")
	} else {
		sourcePath := "/opt/ccp-collector-config-with-defaults.yml"
		destinationPath := "/opt/microsoft/otelcollector/ccp-collector-config-default.yml"
		err := shared.CopyFile(sourcePath, destinationPath)
		if err != nil {
			fmt.Printf("Error copying file: %v\n", err)
		} else {
			fmt.Println("File copied successfully.")
		}
	}
}

// ParseMetricsFiles parses multiple metrics configuration files into a nested map structure
func ParseMetricsFiles(filePaths []string) (map[string]map[string]string, error) {
	// Map to store the parsed data
	parsedData := make(map[string]map[string]string)

	for _, filePath := range filePaths {
		// Open the file
		file, err := os.Open(filePath)
		if err != nil {
			return nil, fmt.Errorf("error opening file %s: %w", filePath, err)
		}
		defer file.Close()

		// Scanner to read the file line by line
		scanner := bufio.NewScanner(file)
		var currentSection string

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())

			// Skip empty lines
			if line == "" {
				continue
			}

			// Check if the line is a new section
			if strings.HasSuffix(line, ": |-") {
				// Extract the section name
				currentSection = strings.TrimSuffix(line, ": |-")
				if parsedData[currentSection] == nil {
					parsedData[currentSection] = make(map[string]string)
				}
				continue
			}

			// Parse key-value pairs within a section
			if currentSection != "" && strings.Contains(line, "=") {
				parts := strings.SplitN(line, "=", 2)
				key := strings.TrimSpace(parts[0])
				value := shared.RemoveQuotes(strings.TrimSpace(parts[1]))
				parsedData[currentSection][key] = value
			}
		}

		// Handle scanner errors
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("error reading file %s: %w", filePath, err)
		}
	}

	return parsedData, nil
}

// ParseV1Config parses the v1 configuration from individual files into a nested map structure
func ParseV1Config(configDir string) (map[string]map[string]string, error) {
	// Map to store the parsed data
	parsedData := make(map[string]map[string]string)

	// Read all files in the configuration directory
	files, err := os.ReadDir(configDir)
	if err != nil {
		return nil, fmt.Errorf("error reading config directory: %w", err)
	}

	// Iterate over each file in the directory
	for _, file := range files {
		if file.IsDir() || strings.HasPrefix(file.Name(), ".") {
			continue
		}

		filePath := filepath.Join(configDir, file.Name())
		fileName := file.Name()

		// Open the file
		f, err := os.Open(filePath)
		if err != nil {
			return nil, fmt.Errorf("error opening file %s: %w", filePath, err)
		}
		defer f.Close()

		// Initialize a map for this section
		sectionData := make(map[string]string)

		// Read the file line by line
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())

			// Skip empty lines
			if line == "" {
				continue
			}

			// Parse key-value pairs
			if strings.Contains(line, "=") {
				parts := strings.SplitN(line, "=", 2)
				key := strings.TrimSpace(parts[0])
				value := shared.RemoveQuotes(strings.TrimSpace(parts[1]))
				sectionData[key] = value
			}
		}

		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("error reading file %s: %w", filePath, err)
		}

		// Add the section data to the parsed data map
		parsedData[fileName] = sectionData
	}

	return parsedData, nil
}
