package shared

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const settingsTestDir = "../test/configmapparser/settings"

func TestConfigmapParserForCCP_WhenSettingsAreValid_PrometheusConfigIsSuccessfullyGenerated(t *testing.T) {
	tmpDir := t.TempDir()
	loadSettings(t, tmpDir)
	// minimalingestionprofile=true, apiserver and etcd enabled
	updateSetting(tmpDir, "default-targets-metrics-keep-list", "minimalingestionprofile", "true")
	os.Setenv("GO_ENV", "test")
	os.Setenv("CONTROLLER_TYPE", replicasetControllerType)
	// Change the current working directory to the test tmp dir
	err := os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Error changing working directory for testing: %s", err)
	}
	// bug? config-version, wrong file check but it actually works if both version files exist

	ConfigmapParserForCCP()

	assert.True(t, existsAndNotEmpty(mergedDefaultConfigPath))
	generatedConfig, err := readAndTrim(mergedDefaultConfigPath)
	if err != nil {
		t.Fatalf("Error changing working directory for testing: %s", err)
	}
	assert.True(t, strings.Count(generatedConfig, "job_name") == 2)
	assert.Contains(t, generatedConfig, "job_name: controlplane-apiserver")
	assert.Contains(t, generatedConfig, "job_name: controlplane-etcd")
	// bug: minimalingestionprofile=true but it doesn't add the default metrics list
	assert.Contains(t, generatedConfig, controlplaneApiserverMinMac)
	assert.Contains(t, generatedConfig, controlplaneEtcdMinMac)
}

func loadSettings(t *testing.T, testDir string) {
	// Create directories to match what we see in prod
	settingsTargetDir := filepath.Join(testDir, "/etc/config/settings")
	defaultConfigFilesDir := filepath.Join(testDir, defaultPromConfigPathPrefix)
	optTestFilesDir := filepath.Join(testDir, "/opt")
	os.MkdirAll(filepath.Join(optTestFilesDir, "/microsoft/configmapparser"), os.ModePerm)
	os.MkdirAll(settingsTargetDir, os.ModePerm)
	os.MkdirAll(defaultConfigFilesDir, os.ModePerm)

	// Read configmap settings from the test directory
	configmapSettings, err := os.ReadDir(settingsTestDir)
	if err != nil {
		t.Fatalf("Error reading settings test files: %s", err)
	}

	// Copy configmap settings to test temp dir
	for _, settingsFile := range configmapSettings {
		// Skip directories
		if settingsFile.IsDir() {
			continue
		}

		from := filepath.Join(settingsTestDir, settingsFile.Name())
		to := filepath.Join(settingsTargetDir, settingsFile.Name())
		copyFile(from, to)
	}

	// Copy default prom configs
	for _, currentFile := range defaultFilesArray {
		filename := filepath.Base(currentFile)
		from := filepath.Join("../configmapparser/default-prom-configs/", filename)
		to := filepath.Join(defaultConfigFilesDir, filename)
		err := copyFile(from, to)
		if err != nil {
			t.Fatalf("Error writting default config files to test dir: %s", err)
		}
	}

	// Copy other files
	ccpCollectorSettings := "../test/configmapparser/opt/ccp-collector-config-with-defaults.yml"
	to := filepath.Join(optTestFilesDir, "ccp-collector-config-with-defaults.yml")
	copyFile(ccpCollectorSettings, to)
}

func updateSetting(testDir, configFileName, settingKey, value string) error {
	settingsFilename := filepath.Join(testDir, "/etc/config/settings", configFileName)
	content, _ := readAndTrim(settingsFilename)
	newContent := value
	if strings.Contains(content, "=") {
		newContent = replaceValue(content, settingKey, value)
	}
	if err := os.WriteFile(settingsFilename, []byte(newContent), os.ModePerm); err != nil {
		return err
	}
	return nil
}

func replaceValue(configSection, key, newValue string) string {
	settingStartPosition := strings.Index(configSection, key) + len(key)
	valueStartPosition := strings.Index(configSection[settingStartPosition:], "=") + settingStartPosition + 1
	nextLineBreak := strings.Index(configSection[valueStartPosition:], "\n")
	if nextLineBreak > 0 {
		valueEndPosition := nextLineBreak + valueStartPosition
		return configSection[:valueStartPosition] + newValue + configSection[valueEndPosition:]
	}

	return configSection[:valueStartPosition] + newValue
}
