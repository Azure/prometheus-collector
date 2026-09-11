package shared

import (
	"fmt"
	"os"
	"strings"
)

// SetupArcEnvironment sets up the IS_ARC_CLUSTER environment variable for Azure Arc.
func SetupArcEnvironment() error {
	// Initialize IS_ARC_CLUSTER variable
	isArcCluster := "false"

	// Check if CLUSTER environment variable contains "connectedclusters"
	cluster := os.Getenv("CLUSTER")
	clusterLowerCase := strings.ToLower(cluster)
	if strings.Contains(clusterLowerCase, "connectedclusters") {
		isArcCluster = "true"
	}

	// Export IS_ARC_CLUSTER variable. Child processes inherit it because every process
	// launcher in this package sets cmd.Env = append(os.Environ()).
	err := os.Setenv("IS_ARC_CLUSTER", isArcCluster)
	if err != nil {
		return fmt.Errorf("error setting environment variable: %w", err)
	}

	// EULA statement for Arc extension
	if isArcCluster == "true" {
		fmt.Println("MICROSOFT SOFTWARE LICENSE TERMS\n\nMICROSOFT Azure Arc-enabled Kubernetes\n\nThis software is licensed to you as part of your or your company's subscription license for Microsoft Azure Services. You may only use the software with Microsoft Azure Services and subject to the terms and conditions of the agreement under which you obtained Microsoft Azure Services. If you do not have an active subscription license for Microsoft Azure Services, you may not use the software. Microsoft Azure Legal Information: https://azure.microsoft.com/en-us/support/legal/")
	}

	return nil
}
