package main

import (
	"os"
	"regexp"
	"strings"
)

func getControllerType() string {
	// Get CONTROLLER_TYPE environment variable
	controllerType := os.Getenv("CONTROLLER_TYPE")

	// Convert controllerType to lowercase and trim spaces
	controllerTypeLower := strings.ToLower(strings.TrimSpace(controllerType))

	return controllerTypeLower
}

func isValidRegex(input string) bool {
	_, err := regexp.Compile(input)
	return err == nil
}
