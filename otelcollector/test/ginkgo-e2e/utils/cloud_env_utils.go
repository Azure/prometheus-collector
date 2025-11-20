package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
)

// CloudEnvironment represents different Azure cloud environments
type CloudEnvironment int

const (
	Public CloudEnvironment = iota // 0
	USSec
	USNat
)

// String returns the string representation of the CloudEnvironment
func (c CloudEnvironment) String() string {
	switch c {
	case Public:
		return "public"
	case USSec:
		return "ussec"
	case USNat:
		return "usnat"
	default:
		return "unknown"
	}
}

// ParseCloudEnvironment converts a string to CloudEnvironment
func ParseCloudEnvironment(s string) (CloudEnvironment, error) {
	switch strings.ToLower(strings.Trim(s, " ")) { // Trim leading/trailing spaces.
	case "":
		return Public, nil
	case "public":
		return Public, nil
	case "ussec":
		return USSec, nil
	case "usnat":
		return USNat, nil
	default:
		return -1, fmt.Errorf("invalid cloud environment: %s", s)
	}
}

func (cloudEnv *CloudEnvironment) GenerateCloudConfigFilePath() string {
	return fmt.Sprintf("../configuration/cloudconfig_%s.json", cloudEnv.String())
}

func (cloudEnv CloudEnvironment) ReadCloudConfig() (*cloud.Configuration, error) {
	switch cloudEnv {
	case Public:
		config := cloud.AzurePublic
		return &config, nil
	case USSec:
		configUSSec, err := ReadCloudConfigFile(cloudEnv.GenerateCloudConfigFilePath())
		return configUSSec, err
	case USNat:
		configUSNat, err := ReadCloudConfigFile(cloudEnv.GenerateCloudConfigFilePath())
		return configUSNat, err
	default:
		return nil, fmt.Errorf("unsupported cloud environment: %s", cloudEnv.String())
	}
}

// ReadCloudConfig checks if the JSON file exists, then reads and deserializes it into a cloud.Configuration struct
func ReadCloudConfigFile(filePath string) (*cloud.Configuration, error) {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file does not exist: %s", filePath)
	} else if err != nil {
		return nil, fmt.Errorf("error checking file: %w", err)
	}

	// Read the file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Unmarshal JSON into cloud.Configuration struct
	var config cloud.Configuration
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Pretty print the entire config as JSON
	fmt.Println("Full configuration as JSON:")
	jsonBytes, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling config: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(jsonBytes))

	return &config, nil
}
