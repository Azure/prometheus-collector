package shared

import (
	"os"
	"regexp"
	"strings"
)

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
