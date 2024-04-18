package configmapsettings

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"strings"

	"prometheus-collector/shared"
)

func updateBashrc(lines []string) error {
	// Open .bashrc file for appending
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("error getting home directory: %v" + err.Error())
	}
	bashrcPath := homeDir + "/.bashrc"
	f, err := os.OpenFile(bashrcPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("error opening .bashrc file: %v" + err.Error())
	}
	defer f.Close()

	// Append lines to .bashrc
	for _, line := range lines {
		if _, err := f.WriteString(line + "\n"); err != nil {
			return fmt.Errorf("error appending to .bashrc file: %v" + err.Error())
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
		return fmt.Errorf("error reloading .bashrc: %v" + err.Error())
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
		return fmt.Errorf("error creating reload script: %v" + err.Error())
	}
	defer os.Remove("/tmp/reload_bashrc.sh")

	// Execute the reload script
	cmd := exec.Command("/bin/bash", "-c", "/tmp/reload_bashrc.sh")
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("error reloading .bashrc: %v" + err.Error())
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
		shared.EchoError("Error reading schema version file:" + err.Error())
		return
	}
	trimmedContent := strings.TrimSpace(string(content))
	configSchemaVersion := strings.ReplaceAll(trimmedContent, " ", "")
	if len(configSchemaVersion) > 10 {
		configSchemaVersion = configSchemaVersion[:10]
	}
	shared.SetEnvAndSourceBashrc("AZMON_AGENT_CFG_SCHEMA_VERSION", configSchemaVersion)

	bashrcPath := os.Getenv("HOME") + "/.bashrc"
	bashrc, err := os.OpenFile(bashrcPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		shared.EchoError("Error opening .bashrc file:" + err.Error())
		return
	}
	defer bashrc.Close()

	_, err = fmt.Fprintf(bashrc, "\nexport AZMON_AGENT_CFG_SCHEMA_VERSION=%s", configSchemaVersion)
	if err != nil {
		shared.EchoError("Error appending to .bashrc file:" + err.Error())
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
		shared.EchoError("Error reading config version file:" + err.Error())
		return
	}
	trimmedContent := strings.TrimSpace(string(content))
	configFileVersion := strings.ReplaceAll(trimmedContent, " ", "")
	if len(configFileVersion) > 10 {
		configFileVersion = configFileVersion[:10]
	}
	shared.SetEnvAndSourceBashrc("AZMON_AGENT_CFG_FILE_VERSION", configFileVersion)

	bashrcPath := os.Getenv("HOME") + "/.bashrc"
	bashrcContent := fmt.Sprintf("\nexport AZMON_AGENT_CFG_FILE_VERSION=%s", configFileVersion)
	err = os.WriteFile(bashrcPath, []byte(bashrcContent), fs.FileMode(0644))
	if err != nil {
		shared.EchoError("Error appending to .bashrc file:" + err.Error())
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
	fmt.Println("End Processing - pod annotations")
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
	err := shared.SetEnvVarsFromFile(filename)
	if err != nil {
		fmt.Printf("Error when setting env for %s: %v\n", filename, err)
	}
}

func Configmapparser() {
	setConfigFileVersionEnv()
	setConfigSchemaVersionEnv()
	parseSettingsForPodAnnotations()
	parsePrometheusCollectorConfig()
	parseDefaultScrapeSettings()
	parseDebugModeSettings()

	tomlparserTargetsMetricsKeepList()
	tomlparserScrapeInterval()
	prometheusConfigMerger()

	shared.SetEnvAndSourceBashrc("AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG", "false")
	shared.SetEnvAndSourceBashrc("CONFIG_VALIDATOR_RUNNING_IN_AGENT", "true")
	env_for_update := []string{
		"export AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG=false",
		"export CONFIG_VALIDATOR_RUNNING_IN_AGENT=true",
	}
	err := updateBashrc(env_for_update)
	if err != nil {
		shared.EchoError("Error updating .bashrc:" + err.Error())
		return
	}

	// Running promconfigvalidator if promMergedConfig.yml exists
	if _, err := os.Stat("/opt/promMergedConfig.yml"); err == nil {
		if os.Getenv("AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG") == "true" || !shared.FileExists("/opt/microsoft/otelcollector/collector-config.yml") {
			fmt.Println("prom-config-validator::Prometheus custom config validation failed. The custom config will not be used")
			shared.SetEnvAndSourceBashrc("AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG", "true")

			if shared.FileExists("/opt/defaultsMergedConfig.yml") {
				fmt.Println("prom-config-validator::Running validator on just default scrape configs")
				shared.StartCommandAndWait("/opt/promconfigvalidator", "--config", "/opt/defaultsMergedConfig.yml", "--output", "/opt/ccp-collector-config-with-defaults.yml", "--otelTemplate", "/opt/microsoft/otelcollector/collector-config-template.yml")
				if !shared.FileExists("/opt/collector-config-with-defaults.yml") {
					fmt.Println("prom-config-validator::Prometheus default scrape config validation failed. No scrape configs will be used")
				} else {
					shared.CopyFile("/opt/collector-config-with-defaults.yml", "/opt/microsoft/otelcollector/collector-config-default.yml")
				}
			}
			shared.SetEnvAndSourceBashrc("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG", "true")
		} else if _, err := os.Stat("/opt/defaultsMergedConfig.yml"); err == nil {
			fmt.Println("prom-config-validator::No custom prometheus config found. Only using default scrape configs")
			cmd := exec.Command("/opt/promconfigvalidator", "--config", "/opt/defaultsMergedConfig.yml", "--output", "/opt/collector-config-with-defaults.yml", "--otelTemplate", "/opt/microsoft/otelcollector/collector-config-template.yml")
			if err := cmd.Run(); err != nil {
				fmt.Println("prom-config-validator::Prometheus default scrape config validation failed. No scrape configs will be used")
			} else {
				fmt.Println("prom-config-validator::Prometheus default scrape config validation succeeded, using this as collector config")
				if err := os.Link("/opt/collector-config-with-defaults.yml", "/opt/microsoft/otelcollector/collector-config-default.yml"); err != nil {
					shared.EchoError("Error copying default config:" + err.Error())
				}
			}
			shared.SetEnvAndSourceBashrc("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG", "true")
		} else {
			// This else block is needed, when there is no custom config mounted as config map or default configs enabled
			fmt.Println("prom-config-validator::No custom config via configmap or default scrape configs enabled.")
			shared.SetEnvAndSourceBashrc("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG", "true")
		}

	}

	if _, err := os.Stat("/opt/microsoft/prom_config_validator_env_var"); err == nil {
		file, err := os.Open("/opt/microsoft/prom_config_validator_env_var")
		if err != nil {
			shared.EchoError("Error opening file:" + err.Error())
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
			shared.EchoError("Error reading file:" + err.Error())
		}

		cmd := exec.Command("source", "/opt/microsoft/prom_config_validator_env_var")
		if err := cmd.Run(); err != nil {
			shared.EchoError("Error sourcing env file:" + err.Error())
			return
		}

		cmd = exec.Command("source", "~/.bashrc")
		if err := cmd.Run(); err != nil {
			shared.EchoError("Error sourcing ~/.bashrc:" + err.Error())
			return
		}
	}

	fmt.Println("prom-config-validator::Use default prometheus config: %s\n", os.Getenv("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG"))
}
