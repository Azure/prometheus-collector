package configmapsettings

import (
	"fmt"
	"os"
	"regexp"
)

const (
	LOGGING_PREFIX                    = "pod-annotation-based-scraping"
	envVariableTemplateName           = "AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX"
	envVariableAnnotationsEnabledName = "AZMON_PROMETHEUS_POD_ANNOTATION_SCRAPING_ENABLED"
)

func parseConfigMapForPodAnnotations() (map[string]interface{}, error) {
	data, err := os.ReadFile(configMapMountPathForPodAnnotation)
	if err != nil {
		return nil, fmt.Errorf("configmap section not mounted or unreadable: %v", err)
	}

	parsedConfig := make(map[string]interface{})
	if err := toml.Unmarshal(data, &parsedConfig); err != nil {
		return nil, fmt.Errorf("exception parsing config map: %v", err)
	}
	return parsedConfig, nil
}

func isValidRegex(str string) bool {
	_, err := regexp.Compile(str)
	return err == nil
}

func writeConfigToFile(podannotationNamespaceRegex string) error {
	if podannotationNamespaceRegex == "" {
		return nil
	}

	file, err := os.Create(podAnnotationEnvVarPath)
	if err != nil {
		return fmt.Errorf("error creating file: %v", err)
	}
	defer file.Close()

	envVarString := fmt.Sprintf("%s='%s'\n", envVariableTemplateName, podannotationNamespaceRegex)
	envVarAnnotationsEnabled := fmt.Sprintf("%s=%s\n", envVariableAnnotationsEnabledName, "true")

	if _, err := file.WriteString(envVarString + envVarAnnotationsEnabled); err != nil {
		return fmt.Errorf("error writing to file: %v", err)
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

	return writeConfigToFile(podannotationNamespaceRegex)
}

func populatePodAnnotationNamespaceFromConfigMap(metricsConfigBySection map[string]map[string]string) (string, error) {
	// Access the nested map and value
	innerMap, ok := metricsConfigBySection["pod-annotation-based-scraping"]
	if !ok {
		fmt.Println("Pod annotation namespace regex configuration not found")
		return "", fmt.Errorf("pod annotation namespace regex configuration not found")
	}

	regex, ok := innerMap["podannotationnamespaceregex"]
	if !ok || regex == "" {
		fmt.Println("Pod annotation namespace regex does not have a value")
		return "", fmt.Errorf("pod annotation namespace regex does not have a value")
	}

	// Validate the regex
	if isValidRegex(regex) {
		fmt.Printf("Using configmap namespace regex for pod annotations: %s\n", regex)
		return regex, nil
	} else {
		return "", fmt.Errorf("Invalid namespace regex for pod annotations: %s", regex)
	}

	return regexString, nil
}
