package configmapsettings

import (
	"fmt"
	"log"
	"os"
	"regexp"
)

const (
	LOGGING_PREFIX                    = "pod-annotation-based-scraping"
	envVariableTemplateName           = "AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX"
	envVariableAnnotationsEnabledName = "AZMON_PROMETHEUS_POD_ANNOTATION_SCRAPING_ENABLED"
)

func isValidRegex(str string) bool {
	_, err := regexp.Compile(str)
	return err == nil
}

func writeConfigToFile(podannotationNamespaceRegex string) error {
	log.Printf("Writing configuration to file: %s\n", podAnnotationEnvVarPath)
	file, err := os.Create(podAnnotationEnvVarPath)
	if err != nil {
		return fmt.Errorf("error opening file: %v", err)
	}
	defer file.Close()

	if podannotationNamespaceRegex != "" {
		linuxPrefix := ""
		//if os.Getenv("OS_TYPE") != "" && strings.ToLower(os.Getenv("OS_TYPE")) == "linux" {
		//	linuxPrefix = "export "
		//}
		// Writes the variable to the file in the format: AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX='value'
		envVarString := fmt.Sprintf("%s%s='%s'\n", linuxPrefix, envVariableTemplateName, podannotationNamespaceRegex)
		envVarAnnotationsEnabled := fmt.Sprintf("%s%s=%s\n", linuxPrefix, envVariableAnnotationsEnabledName, "true")
		log.Printf("Writing to file: %s%s", envVarString, envVarAnnotationsEnabled)

		if _, err := file.WriteString(envVarString); err != nil {
			return fmt.Errorf("error writing to file: %v", err)
		}
		if _, err := file.WriteString(envVarAnnotationsEnabled); err != nil {
			return fmt.Errorf("error writing to file: %v", err)
		}

		log.Println("Configuration written to file successfully.")
	}
	return nil
}

func configurePodAnnotationSettings(metricsConfigBySection map[string]map[string]string) error {
	if metricsConfigBySection == nil {
		return fmt.Errorf("configmap section not mounted, using defaults")
	}
	podannotationNamespaceRegex, err := populatePodAnnotationNamespaceFromConfigMap(metricsConfigBySection)
	if err != nil {
		return err
	}
	if err := writeConfigToFile(podannotationNamespaceRegex); err != nil {
		return err
	}
	return nil
}

func populatePodAnnotationNamespaceFromConfigMap(metricsConfigBySection map[string]map[string]string) (string, error) {
	// Access the nested map and value
	innerMap, ok := metricsConfigBySection["pod-annotation-based-scraping"]
	if !ok {
		log.Println("Pod annotation namespace regex configuration not found")
		return "", fmt.Errorf("pod annotation namespace regex configuration not found")
	}

	regex, ok := innerMap["podannotationnamespaceregex"]
	if !ok || regex == "" {
		log.Println("Pod annotation namespace regex does not have a value")
		return "", fmt.Errorf("pod annotation namespace regex does not have a value")
	}

	// Validate the regex
	if isValidRegex(regex) {
		log.Printf("Using configmap namespace regex for pod annotations: %s\n", regex)
		return regex, nil
	} else {
		return "", fmt.Errorf("Invalid namespace regex for pod annotations: %s", regex)
	}
}
