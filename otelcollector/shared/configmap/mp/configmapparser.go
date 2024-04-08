package main

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"strings"
)

func reloadBashrc() {
	// Write a shell script to reload .bashrc
	reloadScript := "#!/bin/bash\nsource ~/.bashrc\n"
	err := os.WriteFile("/tmp/reload_bashrc.sh", []byte(reloadScript), fs.FileMode(0744))
	if err != nil {
		fmt.Println("Error creating reload script:", err)
		return
	}
	defer os.Remove("/tmp/reload_bashrc.sh")

	// Execute the reload script
	cmd := exec.Command("/bin/bash", "-c", "/tmp/reload_bashrc.sh")
	err = cmd.Run()
	if err != nil {
		fmt.Println("Error reloading .bashrc:", err)
		return
	}
}

func setConfigSchemaVersionEnv() {
	schemaVersionFile := "/etc/config/settings/schema-version"
	fileInfo, err := os.Stat(schemaVersionFile)
	if err != nil || fileInfo.Size() == 0 {
		return
	}
	content, err := os.ReadFile(schemaVersionFile)
	if err != nil {
		fmt.Println("Error reading schema version file:", err)
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
		fmt.Println("Error opening .bashrc file:", err)
		return
	}
	defer bashrc.Close()

	_, err = fmt.Fprintf(bashrc, "\nexport AZMON_AGENT_CFG_SCHEMA_VERSION=%s", configSchemaVersion)
	if err != nil {
		fmt.Println("Error appending to .bashrc file:", err)
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
		fmt.Println("Error reading config version file:", err)
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
		fmt.Println("Error appending to .bashrc file:", err)
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
	fmt.Println("Start Processing - pod annotations")
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

	os.Setenv("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG", "true")
	startCommandAndWait("/opt/promconfigvalidator", "--config", "/opt/defaultsMergedConfig.yml", "--output", "/opt/ccp-collector-config-with-defaults.yml", "--otelTemplate", "/opt/microsoft/otelcollector/ccp-collector-config-template.yml")

	if !exists("/opt/ccp-collector-config-with-defaults.yml") {
		fmt.Println("prom-config-validator::Prometheus default scrape config validation failed. No scrape configs will be used")
	} else {
		sourcePath := "/opt/ccp-collector-config-with-defaults.yml"
		destinationPath := "/opt/microsoft/otelcollector/ccp-collector-config-default.yml"
		if err := copyFile(sourcePath, destinationPath); err != nil {
			fmt.Printf("Error copying file: %v\n", err)
		} else {
			fmt.Println("File copied successfully.")
		}
	}
}

func main() {
	configmapparser()
}
