package configmapsettings

import (
	"fmt"
	"os"

	"github.com/pelletier/go-toml"
)

var ConfigMapSettings = map[string]*ConfigMapSection{
	SchemaVersion: createConfigMapSection("schema-version", map[string]Setting {
		"schema_version": {
			EnvVarName: "AZMON_AGENT_CFG_SCHEMA_VERSION",
		},
	}),
	ConfigVersion: createConfigMapSection("config-version", map[string]Setting {
		"config_version": {
			EnvVarName: "AZMON_AGENT_CFG_SCHEMA_VERSION",
		},
	}),
	PrometheusCollectorSettings: createConfigMapSection("prometheus-collector-settings", map[string]Setting{
		"default_metric_account_name": {
			EnvVarName: "AZMON_DEFAULT_METRIC_ACCOUNT_NAME",
		},
		"cluster_alias": {
			EnvVarName: "AZMON_CLUSTER_ALIAS",
		},
		"operator_enabled": {
			EnvVarName: "AZMON_OPERATOR_ENABLED_CFG_MAP_SETTING",
		},),
	AnnotationSettings: createConfigMapSection("pod-annotation-based-scraping", "podannotationNamespaceRegex": {
			EnvVarName: "AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX",
		},
		"podannotationEnabled": {
			EnvVarName: "",
		},),
	DebugModeSettings: createConfigMapSection("debug-mode", map[string]Setting{
		"enabled": {
			EnvVarName: "DEBUG_MODE_ENABLED",
		},
	}),
}

type ConfigMapSection struct {
	SectionName      string
	Settings         map[string]Setting
	SectionMountPath string
	EnvVarFilePath   string
	ValidateValue    func(string) bool
}

type Setting struct {
	Value      string
	EnvVarName string
}

func createConfigMapSection(sectionName string, settings map[string]Setting) *ConfigMapSection {
	return &ConfigMapSection{
		SectionName:      sectionName,
		Settings:         settings,
		SectionMountPath: "/etc/config/settings/" + sectionName,
	}
}

func (cmp *ConfigMapSection) Configure() error {
	parsedConfig, err := cmp.parseConfigSection()
	if err != nil {
		return err
	}

	if err = cmp.populateValuesForFields(parsedConfig); err != nil {
		return err
	}

	return cmp.SetSettingsEnvVars()
}

func (cmp *ConfigMapSection) populateValuesForFields(parsedConfig map[string]interface{}) error {

	// Loop through each field in SectionFieldsValues and set values from parsedConfig
	for fieldName := range cmp.Settings {
		if value, ok := parsedConfig[fieldName]; ok {
			if strValue, ok := value.(string); ok {
				// If there's a validation function defined, use it to validate the field value
				if cmp.ValidateValue != nil {
					if !cmp.ValidateValue(strValue) {
						return fmt.Errorf("For field %s, value %s is not valid", fieldName, strValue)
					}
				}

				setting := cmp.Settings[fieldName]
				setting.Value = strValue
				cmp.Settings[fieldName] = setting
			} else {
				return fmt.Errorf("field %s is not a string", fieldName)
			}
		} else {
			return fmt.Errorf("field %s not found in config map", fieldName)
		}
	}

	return nil
}

func (cmp *ConfigMapSection) setSettingsEnvVars() error {

	file, err := os.Create(cmp.EnvVarFilePath)
	if err != nil {
		return fmt.Errorf("error creating file: %v", err)
	}
	defer file.Close()

	for fieldName, setting := range cmp.Settings {
		if setting.Value == "" {
			continue
		}

		if setting.EnvVarName == "" {
			continue
		}

		envVarString := fmt.Sprintf("%s='%s'\n", setting.EnvVarName, setting.Value)

		if _, err := file.WriteString(envVarString); err != nil {
			return fmt.Errorf("error writing to file for field %s: %v", fieldName, err)
		}
	}

	return nil
}

func (cmp *ConfigMapSection) parseConfigSection() (map[string]interface{}, error) {
	data, err := os.ReadFile(cmp.SectionMountPath)
	if err != nil {
		return nil, fmt.Errorf("configmap section not mounted or unreadable: %v", err)
	}

	parsedConfig := make(map[string]interface{})
	if err := toml.Unmarshal(data, &parsedConfig); err != nil {
		return nil, fmt.Errorf("exception parsing config map: %v", err)
	}
	return parsedConfig, nil
}
