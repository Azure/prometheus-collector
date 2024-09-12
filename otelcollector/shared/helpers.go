package shared

import (
	"fmt"
	"log"
	"os"
	"os/exec"
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
