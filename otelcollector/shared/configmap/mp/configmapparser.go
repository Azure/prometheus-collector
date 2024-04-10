package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"strings"
)

func updateBashrc(lines []string) error {
	// Open .bashrc file for appending
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("error getting home directory: %v", err)
	}
	bashrcPath := homeDir + "/.bashrc"
	f, err := os.OpenFile(bashrcPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("error opening .bashrc file: %v", err)
	}
	defer f.Close()

	// Append lines to .bashrc
	for _, line := range lines {
		if _, err := f.WriteString(line + "\n"); err != nil {
			return fmt.Errorf("error appending to .bashrc file: %v", err)
		}
	}

	// Export the variables
	for _, line := range lines {
		parts := splitLine(line)
		os.Setenv(parts[0], parts[1])
	}

	// Reload .bashrc
	err = reloadBashrc()
	if err != nil {
		return fmt.Errorf("error reloading .bashrc: %v", err)
	}

	return nil
}

func splitLine(line string) []string {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return []string{}
	}
	return parts
}

func reloadBashrc() error {
	// Write a shell script to reload .bashrc
	reloadScript := "#!/bin/bash\nsource ~/.bashrc\n"
	err := os.WriteFile("/tmp/reload_bashrc.sh", []byte(reloadScript), 0744)
	if err != nil {
		return fmt.Errorf("error creating reload script: %v", err)
	}
	defer os.Remove("/tmp/reload_bashrc.sh")

	// Execute the reload script
	cmd := exec.Command("/bin/bash", "-c", "/tmp/reload_bashrc.sh")
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("error reloading .bashrc: %v", err)
	}

	return nil
}

func setConfigSchemaVersionEnv() {
	schemaVersionFile := "/etc/config/settings/schema-version"
	fileInfo, err := os.Stat(schemaVersionFile)
	if err != nil || fileInfo.Size() == 0 {
		return
	}
	content, err := os.ReadFile(schemaVersionFile)
	if err != nil {
		echoVar("Error reading schema version file:", err)
		return
	}
	trimmedContent := strings.TrimSpace(string(content))
	configSchemaVersion := strings.ReplaceAll(trimmedContent, " ", "")
	if len(configSchemaVersion) > 10 {
		configSchemaVersion = configSchemaVersion[:10]
	}
	os.Setenv("AZMON_AGENT_CFG_SCHEMA_VERSION", configSchemaVersion)

	bashrcPath := os.Getenv("HOME") + "/.bashrc"
	bashrc, err := os.OpenFile(bashrcPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		echoVar("Error opening .bashrc file:", err)
		return
	}
	defer bashrc.Close()

	_, err = fmt.Fprintf(bashrc, "\nexport AZMON_AGENT_CFG_SCHEMA_VERSION=%s", configSchemaVersion)
	if err != nil {
		echoVar("Error appending to .bashrc file:", err)
		return
	}
	reloadBashrc()
}

func setConfigFileVersionEnv() {
	configVersionFile := "/etc/config/settings/config-version"
	fileInfo, err := os.Stat(configVersionFile)
	if err != nil || fileInfo.Size() == 0 {
		return
	}
	content, err := os.ReadFile(configVersionFile)
	if err != nil {
		echoVar("Error reading config version file:", err)
		return
	}
	trimmedContent := strings.TrimSpace(string(content))
	configFileVersion := strings.ReplaceAll(trimmedContent, " ", "")
	if len(configFileVersion) > 10 {
		configFileVersion = configFileVersion[:10]
	}
	os.Setenv("AZMON_AGENT_CFG_FILE_VERSION", configFileVersion)

	bashrcPath := os.Getenv("HOME") + "/.bashrc"
	bashrcContent := fmt.Sprintf("\nexport AZMON_AGENT_CFG_FILE_VERSION=%s", configFileVersion)
	err = os.WriteFile(bashrcPath, []byte(bashrcContent), fs.FileMode(0644))
	if err != nil {
		echoVar("Error appending to .bashrc file:", err)
		return
	}
	reloadBashrc()
}

func parseSettingsForPodAnnotations() {
	// fmt.Printf("Start Processing - %s\n", LOGGING_PREFIX)
	fmt.Printf("Start Processing - pod annotations\n")
	if err := configurePodAnnotationSettings(); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	echoVar("Start Processing - pod annotations")
}

func parsePrometheusCollectorConfig() {
	parseConfigAndSetEnvInFile()
	filename := "/opt/microsoft/configmapparser/config_prometheus_collector_settings_env_var"
	handleEnvFileError(filename)
}

func parseDefaultScrapeSettings() {
	tomlparserDefaultScrapeSettings()
	filename := "/opt/microsoft/configmapparser/config_default_scrape_settings_env_var"
	handleEnvFileError(filename)
}

func parseDebugModeSettings() {
	ConfigureDebugModeSettings()
	filename := "/opt/microsoft/configmapparser/config_debug_mode_env_var"
	handleEnvFileError(filename)
}

func handleEnvFileError(filename string) {
	err := setEnvVarsFromFile(filename)
	if err != nil {
		fmt.Printf("Error when setting env for %s: %v\n", filename, err)
	}
}

func cleanAndTruncate(input string) string {
	input = strings.ReplaceAll(input, " ", "")
	if len(input) >= 10 {
		input = input[:10]
	}
	return input
}

func configmapparser() {
	setAgentConfigVersionEnv()
	setConfigSchemaVersionEnv()
	parseSettingsForPodAnnotations()
	parsePrometheusCollectorConfig()
	parseDefaultScrapeSettings()
	parseDebugModeSettings()

	tomlparserTargetsMetricsKeepList()
	tomlparserScrapeInterval()
	prometheusConfigMerger()

	os.Setenv("AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG", "false")
	os.Setenv("CONFIG_VALIDATOR_RUNNING_IN_AGENT", "true")
	env_for_update := []string{
		"export AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG=false",
		"export CONFIG_VALIDATOR_RUNNING_IN_AGENT=true",
	}
	err := updateBashrc(env_for_update)
	if err != nil {
		echoVar("Error updating .bashrc:", err)
		return
	}

	// Running promconfigvalidator if promMergedConfig.yml exists
	if _, err := os.Stat("/opt/promMergedConfig.yml"); err == nil {
		if os.Getenv("AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG") == "true" || !fileExists("/opt/microsoft/otelcollector/collector-config.yml") {
			echoVar("prom-config-validator::Prometheus custom config validation failed. The custom config will not be used")
			os.Setenv("AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG", "true")

			if fileExists("/opt/defaultsMergedConfig.yml") {
				echoVar("prom-config-validator::Running validator on just default scrape configs")
				startCommandAndWait("/opt/promconfigvalidator", "--config", "/opt/defaultsMergedConfig.yml", "--output", "/opt/ccp-collector-config-with-defaults.yml", "--otelTemplate", "/opt/microsoft/otelcollector/collector-config-template.yml")
				if !fileExists("/opt/collector-config-with-defaults.yml") {
					echoVar("prom-config-validator::Prometheus default scrape config validation failed. No scrape configs will be used")
				} else {
					copyFile("/opt/collector-config-with-defaults.yml", "/opt/microsoft/otelcollector/collector-config-default.yml")
				}
			}
			os.Setenv("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG", "true")
		} else if _, err := os.Stat("/opt/defaultsMergedConfig.yml"); err == nil {
			echoVar("prom-config-validator::No custom prometheus config found. Only using default scrape configs")
			cmd := exec.Command("/opt/promconfigvalidator", "--config", "/opt/defaultsMergedConfig.yml", "--output", "/opt/collector-config-with-defaults.yml", "--otelTemplate", "/opt/microsoft/otelcollector/collector-config-template.yml")
			if err := cmd.Run(); err != nil {
				echoVar("prom-config-validator::Prometheus default scrape config validation failed. No scrape configs will be used")
			} else {
				echoVar("prom-config-validator::Prometheus default scrape config validation succeeded, using this as collector config")
				if err := os.Link("/opt/collector-config-with-defaults.yml", "/opt/microsoft/otelcollector/collector-config-default.yml"); err != nil {
					echoVar("Error copying default config:", err)
				}
			}
			os.Setenv("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG", "true")
		} else {
			// This else block is needed, when there is no custom config mounted as config map or default configs enabled
			echoVar("prom-config-validator::No custom config via configmap or default scrape configs enabled.")
			os.Setenv("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG", "true")
		}

	}

	if _, err := os.Stat("/opt/microsoft/prom_config_validator_env_var"); err == nil {
		file, err := os.Open("/opt/microsoft/prom_config_validator_env_var")
		if err != nil {
			echoVar("Error opening file:", err)
			return
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			parts := strings.Split(line, "=")
			if len(parts) == 2 {
				key := parts[0]
				value := parts[1]
				os.Setenv(key, value)
			}
		}
		if err := scanner.Err(); err != nil {
			echoVar("Error reading file:", err)
		}

		cmd := exec.Command("source", "/opt/microsoft/prom_config_validator_env_var")
		if err := cmd.Run(); err != nil {
			echoVar("Error sourcing env file:", err)
			return
		}

		cmd = exec.Command("source", "~/.bashrc")
		if err := cmd.Run(); err != nil {
			echoVar("Error sourcing ~/.bashrc:", err)
			return
		}
	}

	echoVar("prom-config-validator::Use default prometheus config: %s\n", os.Getenv("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG"))
}
