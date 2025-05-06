package configmapsettings

import (
	"fmt"
	"os"
	"regexp"

	"github.com/pelletier/go-toml"
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

func configurePodAnnotationSettings() error {
	parsedConfig, err := parseConfigMapForPodAnnotations()
	if err != nil {
		return err
	}

	podannotationNamespaceRegex, err := populatePodAnnotationNamespaceFromConfigMap(parsedConfig)
	if err != nil {
		return err
	}

	return writeConfigToFile(podannotationNamespaceRegex)
}

func populatePodAnnotationNamespaceFromConfigMap(parsedConfig map[string]interface{}) (string, error) {
	regex, ok := parsedConfig["podannotationnamespaceregex"]
	if !ok || regex == nil {
		return "", fmt.Errorf("pod annotation namespace regex not found")
	}

	regexString := regex.(string)
	if !isValidRegex(regexString) {
		return "", fmt.Errorf("invalid namespace regex: %s", regexString)
	}

	return regexString, nil
}
