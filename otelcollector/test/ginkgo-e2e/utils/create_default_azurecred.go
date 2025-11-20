package utils

import (
	"fmt"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

func CreateDefaultAzureCredential(options *azidentity.DefaultAzureCredentialOptions) (*azidentity.DefaultAzureCredential, error) {

	if options == nil {
		options = &azidentity.DefaultAzureCredentialOptions{}
	}

	strCloudEnv := os.Getenv("CLOUD_ENVIRONMENT")
	cloudEnv, err := ParseCloudEnvironment(strCloudEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to parse 'CLOUD_ENVIRONMENT' environment variable (%s): %w", strCloudEnv, err)
	}

	// We only need to set the cloud configuration if it's not public.
	if cloudEnv != Public {
		fmt.Printf("Using cloud environment: %s\r\n", strCloudEnv)

		var cloudConfig *cloud.Configuration
		cloudConfig, err = cloudEnv.ReadCloudConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to read cloud config: %w", err)
		}

		options.ClientOptions.Cloud = *cloudConfig
		options.DisableInstanceDiscovery = true // reduces unnecessary discovery
	}

	cred, err := azidentity.NewDefaultAzureCredential(options)
	if err != nil {
		return nil, fmt.Errorf("failed to create default azure credential: %w", err)
	}
	return cred, nil
}
