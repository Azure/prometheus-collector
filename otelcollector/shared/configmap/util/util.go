package util

import (
    "bufio"
    "fmt"
    "os"
    "path/filepath"
    "regexp"
    "strings"

    "github.com/prometheus-collector/shared"
)

// isValidRegex checks if a regex pattern is valid
func IsValidRegex(pattern string) bool {
	_, err := regexp.Compile(pattern)
	return err == nil
}

// isValidDuration checks if a duration string is valid
func IsValidDuration(duration string) bool {
	// Simplified pattern matching for durations like "30s", "1m", etc.
	pattern := regexp.MustCompile(`^(\d+[smhd])+$`)
	return pattern.MatchString(duration)
}

// SanitizeClusterName sanitizes a cluster name to be used as a label
func SanitizeClusterName(name string) string {
    if name == "" {
        return ""
    }
    
    sanitized := regexp.MustCompile(`[^0-9a-zA-Z]+`).ReplaceAllString(name, "_")
    return strings.Trim(sanitized, "_")
}

// GetClusterName gets the cluster name from environment
func GetClusterName() string {
    if mac := os.Getenv("MAC"); mac == "true" {
        clusterArray := strings.Split(strings.TrimSpace(os.Getenv("CLUSTER")), "/")
        return clusterArray[len(clusterArray)-1]
    }
    return os.Getenv("CLUSTER")
}

// ProcessEnvFile reads environment variables from a file and sets them
func ProcessEnvFile(path string) error {
    if !shared.FileExists(path) {
        return fmt.Errorf("file does not exist: %s", path)
    }
    
    // Open file
    file, err := os.Open(path)
    if err != nil {
        return fmt.Errorf("error opening file: %v", err)
    }
    defer file.Close()
    
    // Read line by line
    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        line := scanner.Text()
        parts := strings.SplitN(line, "=", 2)
        if len(parts) != 2 {
            continue
        }
        
        key := strings.TrimSpace(parts[0])
        value := strings.TrimSpace(parts[1])
        
        // Set environment variable
        shared.SetEnvAndSourceBashrcOrPowershell(key, value, true)
    }
    
    if err := scanner.Err(); err != nil {
        return fmt.Errorf("error reading file: %v", err)
    }
    
    return nil
}
// writeEnvVarsToFile writes environment variables to a file
func WriteEnvVarsToFile(path string, vars map[string]string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Open file for writing
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write each variable
	for key, value := range vars {
		if _, err := fmt.Fprintf(file, "%s=%s\n", key, value); err != nil {
			return err
		}

		// Also set environment variable
		shared.SetEnvAndSourceBashrcOrPowershell(key, value, true)
	}

	return nil
}

// writeYamlToFile writes a map as YAML to a file
func WriteYamlToFile(path string, data interface{}) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Marshal to YAML
	yamlData, err := yaml.Marshal(data)
	if err != nil {
		return err
	}

	// Write to file
	return os.WriteFile(path, yamlData, 0644)
}

// loadHashMap loads a YAML map from file
func LoadHashMap(path string) map[string]string {
	result := make(map[string]string)
	if shared.FileExists(path) {
		content, err := os.ReadFile(path)
		if err == nil {
			yaml.Unmarshal(content, &result)
		}
	}
	return result
}

// readYaml reads a YAML file into a map
func (cm *ConfigManager) readYaml(path string) (map[string]interface{}, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := yaml.Unmarshal(content, &result); err != nil {
		return nil, err
	}

	return result, nil
}


// GetFileContent reads a file and returns its content as string
func GetFileContent(path string) (string, error) {
    if !shared.FileExists(path) {
        return "", fmt.Errorf("file does not exist: %s", path)
    }
    
    content, err := os.ReadFile(path)
    if err != nil {
        return "", fmt.Errorf("error reading file: %v", err)
    }
    
    return strings.TrimSpace(string(content)), nil
}

// UpdateConfigWithKeepList updates a config file with a keep list regex
func UpdateConfigWithKeepList(configFile, keepListRegex string) error {
    if !shared.FileExists(configFile) {
        return fmt.Errorf("file does not exist: %s", configFile)
    }
    
    if keepListRegex == "" {
        return nil
    }
    
    content, err := os.ReadFile(configFile)
    if err != nil {
        return fmt.Errorf("error reading file: %v", err)
    }
    
    // Parse YAML
    var config map[string]interface{}
    if err := yaml.Unmarshal(content, &config); err != nil {
        return fmt.Errorf("error parsing YAML: %v", err)
    }
    
    // Update scrape configs
    if scrapeConfigs, ok := config["scrape_configs"].([]interface{}); ok {
        for i, cfg := range scrapeConfigs {
            if scrapeMap, ok := cfg.(map[string]interface{}); ok {
                // Create metric relabel config
                metricRelabel := map[string]interface{}{
                    "source_labels": []string{"__name__"},
                    "action":        "keep",
                    "regex":         keepListRegex,
                }
                
                // Add to existing relabel configs or create new
                if relabelConfigs, ok := scrapeMap["metric_relabel_configs"].([]interface{}); ok {
                    scrapeMap["metric_relabel_configs"] = append(relabelConfigs, metricRelabel)
                } else {
                    scrapeMap["metric_relabel_configs"] = []interface{}{metricRelabel}
                }
                
                // Update the scrape config
                scrapeConfigs[i] = scrapeMap
            }
        }
        
        // Update the config
        config["scrape_configs"] = scrapeConfigs
        
        // Write back to file
        yamlData, err := yaml.Marshal(config)
        if err != nil {
            return fmt.Errorf("error marshalling YAML: %v", err)
        }
        
        return os.WriteFile(configFile, yamlData, 0644)
    }
    
    return nil
}

// UpdateConfigWithInterval updates a config file with a scrape interval
func UpdateConfigWithInterval(configFile, interval string) error {
    if !shared.FileExists(configFile) {
        return fmt.Errorf("file does not exist: %s", configFile)
    }
    
    if interval == "" {
        return nil
    }
    
    content, err := os.ReadFile(configFile)
    if err != nil {
        return fmt.Errorf("error reading file: %v", err)
    }
    
    // Parse YAML
    var config map[string]interface{}
    if err := yaml.Unmarshal(content, &config); err != nil {
        return fmt.Errorf("error parsing YAML: %v", err)
    }
    
    // Update scrape configs
    if scrapeConfigs, ok := config["scrape_configs"].([]interface{}); ok {
        for i, cfg := range scrapeConfigs {
            if scrapeMap, ok := cfg.(map[string]interface{}); ok {
                scr