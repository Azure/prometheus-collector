package shared

import (
	"os"
	"regexp"
	"strings"
	"log"
	"os/exec"
	"fmt"
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
		fluentBitConfigFile = "/opt/fluent-bit/fluent-bit.conf"
		if clusterOverride == "true" {
			meConfigFile = "/usr/sbin/me_internal.config"
		} else {
			meConfigFile = "/usr/sbin/me.config"
		}
	case os.Getenv("OS_TYPE") != "windows":
		fluentBitConfigFile = "/opt/fluent-bit/fluent-bit.conf"
		if clusterOverride == "true" {
			meConfigFile = "/usr/sbin/me_ds_internal.config"
		} else {
			meConfigFile = "/usr/sbin/me_ds.config"
		}
	default:
		fluentBitConfigFile = "/opt/fluent-bit/fluent-bit-windows.conf"
		if clusterOverride == "true" {
			meConfigFile = "/usr/sbin/me_ds_internal_win.config"
		} else {
			meConfigFile = "/usr/sbin/me_ds_win.config"
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
}
