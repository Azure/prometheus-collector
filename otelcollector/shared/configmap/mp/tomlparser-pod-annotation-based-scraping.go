package configmapsettings

import (
	"fmt"
	"os"
	"regexp"

	"github.com/pelletier/go-toml"
)

const (
	LOGGING_PREFIX                     = "pod-annotation-based-scraping"
	envVariableTemplateName            = "AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX"
	envVariableAnnotationsEnabledName  = "AZMON_PROMETHEUS_POD_ANNOTATION_SCRAPING_ENABLED"
)

func parseConfigMapForPodAnnotations() (map[string]interface{}, error) {
	file, err := os.Open(configMapMountPathForPodAnnotation)
	if err != nil {
		return nil, fmt.Errorf("configmap section not mounted, using defaults")
	}
	defer file.Close()

	if data, err := os.ReadFile(configMapMountPathForPodAnnotation); err == nil {
		parsedConfig := make(map[string]interface{})
		if err := toml.Unmarshal(data, &parsedConfig); err == nil {
			return parsedConfig, nil
		} else {
			return nil, fmt.Errorf("exception while parsing config map for pod annotations: %v, using defaults, please check config map for pod annotations", err)
		}
	} else {
		return nil, fmt.Errorf("error reading config map file: %v", err)
	}
}

func isValidRegex(str string) bool {
	_, err := regexp.Compile(str)
	return err == nil
}

func writeConfigToFile(podannotationNamespaceRegex string) error {
	fmt.Printf("Writing configuration to file: %s\n", podAnnotationEnvVarPath)
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
		envVarString := fmt.Sprintf("%s%s='%s'\n", linuxPrefix, envVariableTemplateName, podannotationNamespaceRegex)
		envVarAnnotationsEnabled := fmt.Sprintf("%s%s=%s\n", linuxPrefix, envVariableAnnotationsEnabledName, "true")
		fmt.Printf("Writing to file: %s%s", envVarString, envVarAnnotationsEnabled)

		if _, err := file.WriteString(envVarString); err != nil {
			return fmt.Errorf("error writing to file: %v", err)
		}
		if _, err := file.WriteString(envVarAnnotationsEnabled); err != nil {
			return fmt.Errorf("error writing to file: %v", err)
		}

		fmt.Println("Configuration written to file successfully.")
	}
	return nil
}

func configurePodAnnotationSettings() error {
	parsedConfig, err := parseConfigMapForPodAnnotations()
	if err != nil || parsedConfig == nil{
		return err
	}
	podannotationNamespaceRegex, err := populatePodAnnotationNamespaceFromConfigMap(parsedConfig)
	if err != nil {
		return err
	}
	if err := writeConfigToFile(podannotationNamespaceRegex); err != nil {
		return err
	}
	return nil
}

func populatePodAnnotationNamespaceFromConfigMap(parsedConfig map[string]interface{}) (string, error) {
	regex, ok := parsedConfig["podannotationnamespaceregex"]
	if !ok || regex == nil {
		fmt.Printf("Pod annotation namespace regex does not have a value")
		return "", fmt.Errorf("Pod annotation namespace regex does not have a value")
	}
	regexString := regex.(string)
	if isValidRegex(regexString) {
		fmt.Printf("Using configmap namespace regex for podannotations: %s\n", regexString)
		return regexString, nil
	} else {
		return "", fmt.Errorf("Invalid namespace regex for podannotations: %s\n", regexString)
	}
}
