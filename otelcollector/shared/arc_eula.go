package shared

import (
	"fmt"
	"os"
	"strings"
)

// SetupArcEnvironment sets up environment variables and modifies .bashrc as needed for Azure Arc.
func SetupArcEnvironment() error {
	// Initialize IS_ARC_CLUSTER variable
	isArcCluster := "false"

	// Check if CLUSTER environment variable contains "connectedclusters"
	cluster := os.Getenv("CLUSTER")
	clusterLowerCase := strings.ToLower(cluster)
	if strings.Contains(clusterLowerCase, "connectedclusters") {
		isArcCluster = "true"
	}

	// Export IS_ARC_CLUSTER variable
	err := os.Setenv("IS_ARC_CLUSTER", isArcCluster)
	if err != nil {
		return fmt.Errorf("error setting environment variable: %w", err)
	}

	// Get the home directory
	home := os.Getenv("HOME")
	if home == "" {
		return fmt.Errorf("HOME environment variable not set")
	}

	// Create the path for .bashrc
	bashrcPath := home + "/.bashrc"

	// Check if .bashrc exists
	if _, err := os.Stat(bashrcPath); os.IsNotExist(err) {
		// Create .bashrc file
		if _, err := os.Create(bashrcPath); err != nil {
			return fmt.Errorf("error creating .bashrc file: %w", err)
		}
	}

	// Open .bashrc file for appending
	bashrcFile, err := os.OpenFile(bashrcPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("error opening .bashrc file: %w", err)
	}
	defer bashrcFile.Close()

	// Append export command to .bashrc file
	exportCommand := fmt.Sprintf("export IS_ARC_CLUSTER=%s\n", isArcCluster)
	if _, err := bashrcFile.WriteString(exportCommand); err != nil {
		return fmt.Errorf("error writing to .bashrc file: %w", err)
	}

	// EULA statement for Arc extension
	if isArcCluster == "true" {
		fmt.Println("MICROSOFT SOFTWARE LICENSE TERMS\n\nMICROSOFT Azure Arc-enabled Kubernetes\n\nThis software is licensed to you as part of your or your company's subscription license for Microsoft Azure Services. You may only use the software with Microsoft Azure Services and subject to the terms and conditions of the agreement under which you obtained Microsoft Azure Services. If you do not have an active subscription license for Microsoft Azure Services, you may not use the software. Microsoft Azure Legal Information: https://azure.microsoft.com/en-us/support/legal/")
	}

	return nil
}
