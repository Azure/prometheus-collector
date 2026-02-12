package common

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/prometheus-collector/shared"
)

const maxVersionLength = 10

// SetEnvFromFile reads the value from the provided file, sanitizes it, and sets
// the environment variable using shared.SetEnvAndSourceBashrcOrPowershell. If
// the file does not exist or is empty, the defaultValue is used instead. The
// sanitized value is returned to the caller for further use.
func SetEnvFromFile(filePath, envVarName, defaultValue string) string {
	info, err := os.Stat(filePath)
	if err != nil || info.Size() == 0 {
		shared.SetEnvAndSourceBashrcOrPowershell(envVarName, defaultValue, true)
		return defaultValue
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		shared.EchoError("Error reading " + filepath.Base(filePath) + ":" + err.Error())
		shared.SetEnvAndSourceBashrcOrPowershell(envVarName, defaultValue, true)
		return defaultValue
	}

	trimmed := strings.TrimSpace(string(content))
	sanitized := strings.ReplaceAll(trimmed, " ", "")
	if len(sanitized) > maxVersionLength {
		sanitized = sanitized[:maxVersionLength]
	}

	shared.SetEnvAndSourceBashrcOrPowershell(envVarName, sanitized, true)
	return sanitized
}

// LoadMetricsConfiguration parses the config map contents based on the schema
// version. For v2 schema, it reads the files listed in v2FilePaths. For v1, it
// parses the directory pointed by configDir. When no schema version is set or
// an unsupported value is provided, an empty map and SchemaVersion.Nil are
// returned so callers can gracefully fall back to defaults.
func LoadMetricsConfiguration(schemaVersion string, v2FilePaths []string, configDir string) (map[string]map[string]string, error) {
	parsedSchema := shared.ParseSchemaVersion(schemaVersion)
	switch parsedSchema {
	case shared.SchemaVersion.V2:
		if len(v2FilePaths) == 0 {
			log.Println("LoadMetricsConfiguration::v2 schema detected but no file paths provided")
			return map[string]map[string]string{}, nil
		}

		metricsConfigBySection, err := shared.ParseMetricsFiles(v2FilePaths)
		if err != nil {
			return nil, err
		}
		return metricsConfigBySection, nil
	case shared.SchemaVersion.V1:
		metricsConfigBySection, err := shared.ParseV1Config(configDir)
		if err != nil {
			return nil, err
		}
		return metricsConfigBySection, nil
	default:
		log.Println("LoadMetricsConfiguration::Invalid schema version or no configmap present. Using defaults.")
		return map[string]map[string]string{}, nil
	}
}
