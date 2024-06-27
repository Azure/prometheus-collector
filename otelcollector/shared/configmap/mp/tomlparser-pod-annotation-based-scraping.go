package configmapsettings

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
)

const (
	LOGGING_PREFIX                     = "pod-annotation-based-scraping"
	configMapMountPathForPodAnnotation = "/etc/config/settings/pod-annotation-based-scraping"
	configOutputFilePath               = "/opt/microsoft/configmapparser/config_def_pod_annotation_based_scraping"
	envVariableTemplateName            = "AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX"
)

var podannotationNamespaceRegex string

func parseConfigMapForPodAnnotations() error {
	// Check if config map file exists
	file, err := os.Open(configMapMountPathForPodAnnotation)
	if err != nil {
		return fmt.Errorf("configmap section not mounted, using defaults")
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if regex := extractRegex(line); regex != "" {
			if isValidRegex(regex) {
				podannotationNamespaceRegex = regex
				fmt.Printf("Using configmap namespace regex for podannotations: %s\n", podannotationNamespaceRegex)
				break
			} else {
				return fmt.Errorf("invalid namespace regex for podannotations")
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading config map: %v", err)
	}
	return nil
}

func extractRegex(line string) string {
	if len(line) >= len(envVariableTemplateName)+1 && line[:len(envVariableTemplateName)] == envVariableTemplateName {
		return line[len(envVariableTemplateName)+1:]
	}
	return ""
}

func isValidRegex(str string) bool {
	_, err := regexp.Compile(str)
	return err == nil
}

func writeConfigToFile() error {
	fmt.Printf("Writing configuration to file: %s\n", configOutputFilePath)
	file, err := os.Create(configOutputFilePath)
	if err != nil {
		return fmt.Errorf("error opening file: %v", err)
	}
	defer file.Close()

	if _, err := file.WriteString(fmt.Sprintf("%s='%s'\n", envVariableTemplateName, podannotationNamespaceRegex)); err != nil {
		return fmt.Errorf("error writing to file: %v", err)
	}

	fmt.Println("Configuration written to file successfully.")
	return nil
}

func configurePodAnnotationSettings() error {
	if err := parseConfigMapForPodAnnotations(); err != nil {
		return err
	}
	if err := writeConfigToFile(); err != nil {
		return err
	}
	return nil
}
