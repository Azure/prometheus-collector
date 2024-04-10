package shared

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

func setupTelemetry(customEnvironment string) {
	// Convert customEnvironment to lowercase
	customEnvironmentLower := strings.ToLower(customEnvironment)

	// Variables to store the telemetry details
	var encodedAIKey, aiEndpoint, aiKey string

	// Setting up telemetry based on customEnvironment
	switch customEnvironmentLower {
	case "azurepubliccloud":
		encodedAIKey = os.Getenv("APPLICATIONINSIGHTS_AUTH_PUBLIC")
		fmt.Println("Setting telemetry output to the default azurepubliccloud instance")
	case "azureusgovernmentcloud":
		encodedAIKey = os.Getenv("APPLICATIONINSIGHTS_AUTH_USGOVERNMENT")
		aiEndpoint = "https://dc.applicationinsights.us/v2/track"
		// IngestionEndpoint=https://usgovvirginia-1.in.applicationinsights.azure.us/;AADAudience=https://monitor.azure.us/
		fmt.Println("Setting telemetry output to the azureusgovernmentcloud instance")
	case "azurechinacloud":
		encodedAIKey = os.Getenv("APPLICATIONINSIGHTS_AUTH_CHINACLOUD")
		aiEndpoint = "https://dc.applicationinsights.azure.cn/v2/track"
		// IngestionEndpoint=https://chinanorth3-0.in.applicationinsights.azure.cn/;AADAudience=https://monitor.azure.cn/
		fmt.Println("Setting telemetry output to the azurechinacloud instance")
	case "usnat":
		encodedAIKey = os.Getenv("APPLICATIONINSIGHTS_AUTH_USNAT")
		aiEndpoint = "https://dc.applicationinsights.azure.eaglex.ic.gov/v2/track"
		// IngestionEndpoint: usnateast-0.in.applicationinsights.azure.eaglex.ic.gov
		fmt.Println("Setting telemetry output to the usnat instance")
	case "ussec":
		encodedAIKey = os.Getenv("APPLICATIONINSIGHTS_AUTH_USSEC")
		aiEndpoint = "https://dc.applicationinsights.azure.microsoft.scloud/v2/track"
		// IngestionEndpoint: usseceast-0.in.applicationinsights.azure.microsoft.scloud
		fmt.Println("Setting telemetry output to the ussec instance")
	default:
		fmt.Printf("Unknown customEnvironment: %s, setting telemetry output to the default azurepubliccloud instance\n", customEnvironmentLower)
		encodedAIKey = os.Getenv("APPLICATIONINSIGHTS_AUTH_PUBLIC")
	}

	// Export APPLICATIONINSIGHTS_AUTH
	err := os.Setenv("APPLICATIONINSIGHTS_AUTH", encodedAIKey)
	if err != nil {
		fmt.Println("Error setting APPLICATIONINSIGHTS_AUTH environment variable:", err)
		return
	}

	// Append export commands to .bashrc file
	bashrcPath := os.Getenv("HOME") + "/.bashrc"
	bashrcFile, err := os.OpenFile(bashrcPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error opening .bashrc file:", err)
		return
	}
	defer bashrcFile.Close()

	exportAIKeyCommand := fmt.Sprintf("export APPLICATIONINSIGHTS_AUTH=%s\n", encodedAIKey)
	_, err = bashrcFile.WriteString(exportAIKeyCommand)
	if err != nil {
		fmt.Println("Error writing to .bashrc file:", err)
		return
	}

	if aiEndpoint != "" {
		exportEndpointCommand := fmt.Sprintf("export APPLICATIONINSIGHTS_ENDPOINT=\"%s\"\n", aiEndpoint)
		_, err = bashrcFile.WriteString(exportEndpointCommand)
		if err != nil {
			fmt.Println("Error writing to .bashrc file:", err)
			return
		}
	}

	// Setting TELEMETRY_APPLICATIONINSIGHTS_KEY
	aiKeyBytes, err := base64.StdEncoding.DecodeString(encodedAIKey)
	if err != nil {
		fmt.Println("Error decoding AI key:", err)
		return
	}
	aiKey = string(aiKeyBytes)

	err = os.Setenv("TELEMETRY_APPLICATIONINSIGHTS_KEY", aiKey)
	if err != nil {
		fmt.Println("Error setting TELEMETRY_APPLICATIONINSIGHTS_KEY environment variable:", err)
		return
	}

	exportTelegrafCommand := fmt.Sprintf("export TELEMETRY_APPLICATIONINSIGHTS_KEY=%s\n", aiKey)
	_, err = bashrcFile.WriteString(exportTelegrafCommand)
	if err != nil {
		fmt.Println("Error writing to .bashrc file:", err)
		return
	}
}
