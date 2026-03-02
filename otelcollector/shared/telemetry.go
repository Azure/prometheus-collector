package shared

import (
	"encoding/base64"
	"log"
	"os"
	"strings"
)

func SetupTelemetry(customEnvironment string) {
	// Convert customEnvironment to lowercase
	customEnvironmentLower := strings.ToLower(customEnvironment)

	// Variables to store the telemetry details
	var encodedAIKey, aiEndpoint, aiKey string

	// Setting up telemetry based on customEnvironment
	switch customEnvironmentLower {
	case "azurepubliccloud":
		encodedAIKey = os.Getenv("APPLICATIONINSIGHTS_AUTH_PUBLIC")
		log.Println("Setting telemetry output to the default azurepubliccloud instance")
	case "azureusgovernmentcloud":
		encodedAIKey = os.Getenv("APPLICATIONINSIGHTS_AUTH_USGOVERNMENT")
		aiEndpoint = "https://dc.applicationinsights.us/v2/track"
		// IngestionEndpoint=https://usgovvirginia-1.in.applicationinsights.azure.us/;AADAudience=https://monitor.azure.us/
		log.Println("Setting telemetry output to the azureusgovernmentcloud instance")
	case "azurechinacloud":
		encodedAIKey = os.Getenv("APPLICATIONINSIGHTS_AUTH_CHINACLOUD")
		aiEndpoint = "https://dc.applicationinsights.azure.cn/v2/track"
		// IngestionEndpoint=https://chinanorth3-0.in.applicationinsights.azure.cn/;AADAudience=https://monitor.azure.cn/
		log.Println("Setting telemetry output to the azurechinacloud instance")
	case "usnat":
		encodedAIKey = os.Getenv("APPLICATIONINSIGHTS_AUTH_USNAT")
		aiEndpoint = "https://dc.applicationinsights.azure.eaglex.ic.gov/v2/track"
		// IngestionEndpoint: usnateast-0.in.applicationinsights.azure.eaglex.ic.gov
		log.Println("Setting telemetry output to the usnat instance")
	case "ussec":
		encodedAIKey = os.Getenv("APPLICATIONINSIGHTS_AUTH_USSEC")
		aiEndpoint = "https://dc.applicationinsights.azure.microsoft.scloud/v2/track"
		// IngestionEndpoint: usseceast-0.in.applicationinsights.azure.microsoft.scloud
		log.Println("Setting telemetry output to the ussec instance")
	case "azurebleucloud":
		encodedAIKey = os.Getenv("APPLICATIONINSIGHTS_AUTH_BLEU")
		aiEndpoint = "https://bleufrancecentral-0.in.applicationinsights.sovcloud-api.fr/v2/track"
		// IngestionEndpoint: https://bleufrancecentral-0.in.applicationinsights.sovcloud-api.fr/
		// (global ingestion endpoint - dc.* not supported in bleu)
		log.Println("Setting telemetry output to the azurebleucloud instance")
	default:
		log.Printf("Unknown customEnvironment: %s, setting telemetry output to the default azurepubliccloud instance\n", customEnvironmentLower)
		encodedAIKey = os.Getenv("APPLICATIONINSIGHTS_AUTH_PUBLIC")
	}

	// Export APPLICATIONINSIGHTS_AUTH
	err := SetEnvAndSourceBashrcOrPowershell("APPLICATIONINSIGHTS_AUTH", encodedAIKey, false)
	if err != nil {
		log.Println("Error setting APPLICATIONINSIGHTS_AUTH environment variable:", err)
		return
	}

	// Export APPLICATIONINSIGHTS_ENDPOINT
	err = SetEnvAndSourceBashrcOrPowershell("APPLICATIONINSIGHTS_ENDPOINT", aiEndpoint, false)
	if err != nil {
		log.Println("Error setting APPLICATIONINSIGHTS_ENDPOINT environment variable:", err)
		return
	}

	// Setting TELEMETRY_APPLICATIONINSIGHTS_KEY
	aiKeyBytes, err := base64.StdEncoding.DecodeString(encodedAIKey)
	if err != nil {
		log.Println("Error decoding AI key:", err)
		return
	}
	aiKey = string(aiKeyBytes)

	err = SetEnvAndSourceBashrcOrPowershell("TELEMETRY_APPLICATIONINSIGHTS_KEY", aiKey, false)
	if err != nil {
		log.Println("Error setting TELEMETRY_APPLICATIONINSIGHTS_KEY environment variable:", err)
		return
	}

}
