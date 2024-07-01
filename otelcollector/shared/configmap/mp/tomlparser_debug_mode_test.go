package configmapsettings

import (
	"fmt"
	"io"
	"math/rand"
	"os"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
)

var _ = ginkgo.Describe("When parsing debug mode settings", func() {
	ginkgo.Context("when debug mode is enabled", func() {

		ginkgo.BeforeEach(func() {
			suffix := createRandomString(5)
			err := createTempFiles(suffix, `enabled = true`)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should configure debug mode settings for a linux replica", func() {
			os.Setenv("CONTROLLER_TYPE", "ReplicaSet")
			os.Setenv("OS_TYPE", "linux")

			err := ConfigureDebugModeSettings()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
	
			// Verify that the environment variable file is created
			_, err = os.Stat(debugModeEnvVarPath)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
	
			// Verify the content of the environment variable file
			content, err := os.ReadFile(debugModeEnvVarPath)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(string(content)).To(gomega.Equal("export DEBUG_MODE_ENABLED=true\n"))
	
			// Verify the modification of the YAML configuration file
			config, err := parseYAMLConfigFile(replicaSetCollectorConfig)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config).NotTo(gomega.BeNil())
			gomega.Expect(config["service"].(map[interface{}]interface{})["pipelines"].(map[interface{}]interface{})["metrics"].(map[interface{}]interface{})["exporters"]).To(gomega.Equal([]interface{}{"otlp", "prometheus"}))
		})

		ginkgo.It("should not configure debug mode settings for a linux daemonset", func() {
			os.Setenv("CONTROLLER_TYPE", "DaemonSet")
			os.Setenv("OS_TYPE", "linux")

			err := ConfigureDebugModeSettings()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
	
			// Verify that the environment variable file is created
			_, err = os.Stat(debugModeEnvVarPath)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
	
			// Verify the content of the environment variable file
			content, err := os.ReadFile(debugModeEnvVarPath)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(string(content)).To(gomega.Equal("export DEBUG_MODE_ENABLED=true\n"))
	
			// Verify the modification of the YAML configuration file
			config, err := parseYAMLConfigFile(replicaSetCollectorConfig)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config).NotTo(gomega.BeNil())
			gomega.Expect(config["service"].(map[interface{}]interface{})["pipelines"].(map[interface{}]interface{})["metrics"].(map[interface{}]interface{})["exporters"]).To(gomega.Equal([]interface{}{"otlp"}))
		})

		ginkgo.It("should not configure debug mode settings for a windows daemonset", func() {
			os.Setenv("CONTROLLER_TYPE", "DaemonSet")
			os.Setenv("OS_TYPE", "windows")

			err := ConfigureDebugModeSettings()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
	
			// Verify that the environment variable file is created
			_, err = os.Stat(debugModeEnvVarPath)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
	
			// Verify the content of the environment variable file
			content, err := os.ReadFile(debugModeEnvVarPath)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(string(content)).To(gomega.Equal("DEBUG_MODE_ENABLED=true\n"))
	
			// Verify the modification of the YAML configuration file
			config, err := parseYAMLConfigFile(replicaSetCollectorConfig)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config).NotTo(gomega.BeNil())
			gomega.Expect(config["service"].(map[interface{}]interface{})["pipelines"].(map[interface{}]interface{})["metrics"].(map[interface{}]interface{})["exporters"]).To(gomega.Equal([]interface{}{"otlp"}))
		})

		ginkgo.AfterEach(func() {
			os.Unsetenv("CONTROLLER_TYPE")
			os.Unsetenv("OS_TYPE")
			configMapDebugMountPath = ""
			debugModeEnvVarPath = ""
			replicaSetCollectorConfig = ""
			fmt.Println("Cleaning up temp files")
			cleanupTempFiles()
		})
	})

	ginkgo.It("should handle a missing config map file", func() {
		os.Setenv("CONTROLLER_TYPE", "ReplicaSet")
		suffix := createRandomString(5)
		ginkgo.DeferCleanup(func() {
			cleanupTempFiles()
		})
		err := createTempFiles(suffix, "")
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		err = os.Remove(fmt.Sprintf("temp/debug-mode-%s", suffix))
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		err = ConfigureDebugModeSettings()
		gomega.Expect(err).To(gomega.HaveOccurred())
		gomega.Expect(err.Error()).To(gomega.ContainSubstring("configmap section not mounted, using defaults"))
	})

	ginkgo.It("should handle an error while parsing config map", func() {
		os.Setenv("CONTROLLER_TYPE", "ReplicaSet")
		suffix := createRandomString(5)
		ginkgo.DeferCleanup(func() {
			cleanupTempFiles()
		})
		err := createTempFiles(suffix, `		[ invalid_key = true		`)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		err = ConfigureDebugModeSettings()
		gomega.Expect(err).To(gomega.HaveOccurred())
		gomega.Expect(err.Error()).To(gomega.ContainSubstring("exception while parsing config map for debug mode"))
	})

	ginkgo.It("should handle an error while opening environment variable file", func() {
		os.Setenv("CONTROLLER_TYPE", "ReplicaSet")
		suffix := createRandomString(5)
		ginkgo.DeferCleanup(func() {
			cleanupTempFiles()
		})
		err := createTempFiles(suffix, `enabled = true`)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		debugModeEnvVarPath = "/nonexistant-path/envvarpath"

		err = ConfigureDebugModeSettings()
		gomega.Expect(err).To(gomega.HaveOccurred())
		gomega.Expect(err.Error()).To(gomega.ContainSubstring("Exception while opening file for writing prometheus-collector config environment variables"))
	})

	ginkgo.It("should handle an error while reading the replicaset collector config file", func() {
		os.Setenv("CONTROLLER_TYPE", "ReplicaSet")
		suffix := createRandomString(5)
		ginkgo.DeferCleanup(func() {
			cleanupTempFiles()
		})
		err := createTempFiles(suffix, `enabled = true`)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		replicaSetCollectorConfig = "/nonexistant-path/replicasetconfig"

		err = ConfigureDebugModeSettings()
		gomega.Expect(err).To(gomega.HaveOccurred())
		gomega.Expect(err.Error()).To(gomega.ContainSubstring("Exception while setting otlp in the exporter metrics for service pipeline when debug mode is enabled"))
	})
})

func parseYAMLConfigFile(filePath string) (map[string]interface{}, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("Exception while reading YAML config file: %v", err)
	}

	var config map[string]interface{}
	err = yaml.Unmarshal(content, &config)
	if err != nil {
		return nil, fmt.Errorf("Exception while parsing YAML config file: %v", err)
	}

	return config, nil
}

func createTempFiles(suffix string, debugModeString string) (error) {
	err := os.Mkdir("temp", os.ModePerm)
	if err != nil {
		return err
	}

	configMapDebugMountPath = fmt.Sprintf("temp/debug-mode-%s", suffix)
	debugModeEnvVarPath = fmt.Sprintf("temp/config_debug_mode_env_var-%s", suffix)
	replicaSetCollectorConfig = fmt.Sprintf("temp/collector-config-replicaset-%s.yml", suffix)
  sourceFile := "testdata/collector-config-replicaset.yml"

	source, err := os.Open(sourceFile)
	if err != nil {
		return err
	}
	defer source.Close()
	destination, err := os.Create(replicaSetCollectorConfig)
	if err != nil {
		return err
	}
	defer destination.Close()
	_, err = io.Copy(destination, source)
	if err != nil {
		return err
	}

	debugModeFile, err := os.Create(configMapDebugMountPath)
	if err != nil {
		return err
	}
	defer debugModeFile.Close()

	_, err = debugModeFile.WriteString(debugModeString)
	if err != nil {
		return err
	}

	return nil
}

func cleanupTempFiles() (error) {
	return os.RemoveAll("temp")
}

func createRandomString(length int) string {
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		result[i] = charset[rand.Intn(len(charset))]
	}

	return string(result)
}