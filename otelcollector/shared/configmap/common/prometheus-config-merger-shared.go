package common

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"strings"

	"gopkg.in/yaml.v2"
)

// LogFunc allows callers to customize how messages are emitted. When nil, the
// helper falls back to log.Printf to preserve existing behaviour.
type LogFunc func(string, ...interface{})

func ensureLogFunc(logFn LogFunc) LogFunc {
	if logFn != nil {
		return logFn
	}
	return log.Printf
}

// UpdateScrapeIntervalConfig rewrites the scrape interval of every scrape
// config contained in the supplied YAML file.
func UpdateScrapeIntervalConfig(yamlConfigFile, scrapeIntervalSetting string, logFn LogFunc) error {
	logger := ensureLogFunc(logFn)
	logger("Updating scrape interval config for %s\n", yamlConfigFile)

	data, err := os.ReadFile(yamlConfigFile)
	if err != nil {
		return fmt.Errorf("common.UpdateScrapeIntervalConfig::read %s: %w", yamlConfigFile, err)
	}

	var config map[interface{}]interface{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("common.UpdateScrapeIntervalConfig::unmarshal %s: %w", yamlConfigFile, err)
	}

	scrapeConfigs, ok := config["scrape_configs"].([]interface{})
	if !ok {
		logger("common.UpdateScrapeIntervalConfig::no scrape_configs found in %s\n", yamlConfigFile)
		return nil
	}

	for _, scfg := range scrapeConfigs {
		if scfgMap, ok := scfg.(map[interface{}]interface{}); ok {
			scfgMap["scrape_interval"] = scrapeIntervalSetting
		}
	}

	updated, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("common.UpdateScrapeIntervalConfig::marshal %s: %w", yamlConfigFile, err)
	}

	if err := os.WriteFile(yamlConfigFile, updated, fs.FileMode(0644)); err != nil {
		return fmt.Errorf("common.UpdateScrapeIntervalConfig::write %s: %w", yamlConfigFile, err)
	}

	return nil
}

// AppendMetricRelabelConfig adds or augments the metric_relabel_configs stanza
// for every scrape config contained in yamlConfigFile.
func AppendMetricRelabelConfig(yamlConfigFile, keepListRegex string, logFn LogFunc) error {
	logger := ensureLogFunc(logFn)
	logger("Adding keep list regex or minimal ingestion regex for %s\n", yamlConfigFile)

	content, err := os.ReadFile(yamlConfigFile)
	if err != nil {
		return fmt.Errorf("common.AppendMetricRelabelConfig::read %s: %w", yamlConfigFile, err)
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(content, &config); err != nil {
		return fmt.Errorf("common.AppendMetricRelabelConfig::unmarshal %s: %w", yamlConfigFile, err)
	}

	keepListMetricRelabelConfig := map[string]interface{}{
		"source_labels": []interface{}{"__name__"},
		"action":        "keep",
		"regex":         keepListRegex,
	}

	scrapeConfigs, ok := config["scrape_configs"].([]interface{})
	if !ok {
		logger("common.AppendMetricRelabelConfig::no scrape_configs found in %s\n", yamlConfigFile)
		return nil
	}

	for idx, scfg := range scrapeConfigs {
		stringMap := make(map[string]interface{})
		scfgMap, ok := scfg.(map[interface{}]interface{})
		if !ok {
			continue
		}
		for k, v := range scfgMap {
			key, ok := k.(string)
			if !ok {
				return fmt.Errorf("common.AppendMetricRelabelConfig::non-string key in scrape config: %v", k)
			}
			stringMap[key] = v
		}

		if metricRelabelCfgs, ok := stringMap["metric_relabel_configs"].([]interface{}); ok {
			stringMap["metric_relabel_configs"] = append(metricRelabelCfgs, keepListMetricRelabelConfig)
		} else {
			stringMap["metric_relabel_configs"] = []interface{}{keepListMetricRelabelConfig}
		}

		interfaceMap := make(map[interface{}]interface{})
		for k, v := range stringMap {
			interfaceMap[k] = v
		}
		scrapeConfigs[idx] = interfaceMap
	}

	config["scrape_configs"] = scrapeConfigs

	updated, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("common.AppendMetricRelabelConfig::marshal %s: %w", yamlConfigFile, err)
	}

	if err := os.WriteFile(yamlConfigFile, updated, fs.FileMode(0644)); err != nil {
		return fmt.Errorf("common.AppendMetricRelabelConfig::write %s: %w", yamlConfigFile, err)
	}

	return nil
}

// ReplacePlaceholders substitutes $$PLACEHOLDER$$ tokens in yamlConfigFile with
// the corresponding environment variable values.
func ReplacePlaceholders(yamlConfigFile string, placeholders []string) error {
	contents, err := os.ReadFile(yamlConfigFile)
	if err != nil {
		return fmt.Errorf("common.ReplacePlaceholders::read %s: %w", yamlConfigFile, err)
	}

	replaced := string(contents)
	for _, placeholder := range placeholders {
		replaced = strings.ReplaceAll(replaced, fmt.Sprintf("$$%s$$", placeholder), os.Getenv(placeholder))
	}

	if err := os.WriteFile(yamlConfigFile, []byte(replaced), fs.FileMode(0644)); err != nil {
		return fmt.Errorf("common.ReplacePlaceholders::write %s: %w", yamlConfigFile, err)
	}
	return nil
}

// LoadYAMLFromFile loads the provided YAML file into a map representation.
func LoadYAMLFromFile(filename string) (map[interface{}]interface{}, error) {
	fileContent, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("common.LoadYAMLFromFile::read %s: %w", filename, err)
	}

	var yamlData map[interface{}]interface{}
	if err := yaml.Unmarshal(fileContent, &yamlData); err != nil {
		return nil, fmt.Errorf("common.LoadYAMLFromFile::unmarshal %s: %w", filename, err)
	}

	return yamlData, nil
}

// DeepMerge merges source into target recursively and returns target.
func DeepMerge(target, source map[interface{}]interface{}) map[interface{}]interface{} {
	for key, sourceValue := range source {
		targetValue, exists := target[key]

		if !exists {
			target[key] = sourceValue
			continue
		}

		targetMap, targetMapOk := targetValue.(map[interface{}]interface{})
		sourceMap, sourceMapOk := sourceValue.(map[interface{}]interface{})

		if targetMapOk && sourceMapOk {
			target[key] = DeepMerge(targetMap, sourceMap)
			continue
		}

		targetSlice, targetSliceOk := targetValue.([]interface{})
		sourceSlice, sourceSliceOk := sourceValue.([]interface{})
		if targetSliceOk && sourceSliceOk {
			target[key] = append(targetSlice, sourceSlice...)
			continue
		}

		target[key] = sourceValue
	}

	return target
}

// WriteYAML writes the provided map to the destination file as YAML.
func WriteYAML(dst string, data map[interface{}]interface{}) error {
	serialized, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("common.WriteYAML::marshal %s: %w", dst, err)
	}

	if err := os.WriteFile(dst, serialized, fs.FileMode(0644)); err != nil {
		return fmt.Errorf("common.WriteYAML::write %s: %w", dst, err)
	}

	return nil
}
