// package main

// import (
// 	"fmt"
// 	"io/ioutil"
// 	"os"
// 	"os/exec"
// 	"strings"
// 	"time"
// )

// func main() {
// 	macMode := os.Getenv("MAC") == "true"

// 	// Check for the absence of TokenConfig.json file in MAC mode
// 	if macMode {
// 		if _, err := os.Stat("/etc/mdsd.d/config-cache/metricsextension/TokenConfig.json"); os.IsNotExist(err) {
// 			checkAndHandleMACConfiguration()
// 		}
// 	} else {
// 		// Check if ME is not running
// 		if !isProcessRunning("MetricsExt") {
// 			fmt.Println("Metrics Extension is not running")
// 			os.Exit(1)
// 		}

// 		// Check if the certificates have changed
// 		checkCertificateChange()
// 	}

// 	// Check if otelcollector is not running
// 	if !isProcessRunning("otelcollector") {
// 		fmt.Println("OpenTelemetryCollector is not running")
// 		os.Exit(1)
// 	}

// 	// Check for config changes
// 	if hasConfigChanged("/opt/inotifyoutput.txt") {
// 		fmt.Println("inotifyoutput.txt has been updated - config changed")
// 		os.Exit(1)
// 	}
// }

// func checkAndHandleMACConfiguration() {
// 	if _, err := os.Stat("/opt/microsoft/liveness/azmon-container-start-time"); err == nil {
// 		azmonContainerStartTimeStr, err := ioutil.ReadFile("/opt/microsoft/liveness/azmon-container-start-time")
// 		if err != nil {
// 			fmt.Println("Error reading azmon-container-start-time:", err)
// 			os.Exit(1)
// 		}

// 		azmonContainerStartTime, err := strconv.Atoi(strings.TrimSpace(string(azmonContainerStartTimeStr)))
// 		if err != nil {
// 			fmt.Println("Error converting azmon-container-start-time to integer:", err)
// 			os.Exit(1)
// 		}

// 		epochTimeNow := int(time.Now().Unix())
// 		duration := epochTimeNow - azmonContainerStartTime
// 		durationInMinutes := duration / 60

// 		if durationInMinutes%5 == 0 {
// 			fmt.Printf("%s No configuration present for the AKS resource\n", time.Now().Format("2006-01-02T15:04:05"))
// 		}

// 		if durationInMinutes > 15 {
// 			fmt.Println("No configuration present for the AKS resource")
// 			os.Exit(1)
// 		}
// 	}
// }

// func isProcessRunning(processName string) bool {
// 	cmd := exec.Command("ps", "-ef")
// 	output, err := cmd.Output()
// 	if err != nil {
// 		fmt.Println("Error running ps -ef:", err)
// 		os.Exit(1)
// 	}

// 	return strings.Contains(string(output), processName)
// }

// func checkCertificateChange() {
// 	if _, err := os.Stat("/etc/config/settings/akv"); err == nil {
// 		if _, err := os.Stat("/opt/akv-copy/akv"); err == nil {
// 			cmd := exec.Command("diff", "-r", "-q", "/etc/config/settings/akv", "/opt/akv-copy/akv")
// 			if err := cmd.Run(); err != nil {
// 				fmt.Println("A Metrics Account certificate has changed")
// 				os.Exit(1)
// 			}
// 		}
// 	}
// }

// func hasConfigChanged(filePath string) bool {
// 	if _, err := os.Stat(filePath); err == nil {
// 		fileInfo, err := os.Stat(filePath)
// 		if err != nil {
// 			fmt.Println("Error getting file info:", err)
// 			os.Exit(1)
// 		}

// 		return fileInfo.Size() > 0
// 	}

// 	return false
// }
