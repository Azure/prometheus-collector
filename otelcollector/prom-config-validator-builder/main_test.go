package main_test

import (
	"io/ioutil"
	"os"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = ginkgo.Describe("GenerateOtelConfig", func() {
	var (
		promFilePath          string
		outputFilePath        string
		otelConfigTemplatePath string
	)

	ginkgo.BeforeEach(func() {
		promFilePath = "path/to/prometheus.yml"
		outputFilePath = "path/to/output.yml"
		otelConfigTemplatePath = "path/to/otel-config-template.yml"
	})

	ginkgo.It("should generate the otel config file", func() {
		err := generateOtelConfig(promFilePath, outputFilePath, otelConfigTemplatePath)
		gomega.Expect(err).To(gomega.BeNil())

		// Verify that the output file exists
		_, err = os.Stat(outputFilePath)
		gomega.Expect(err).To(gomega.BeNil())

		// Verify the contents of the output file
		outputFileContents, err := ioutil.ReadFile(outputFilePath)
		gomega.Expect(err).To(gomega.BeNil())

		expectedContents := `...` // Replace with the expected contents of the output file
		gomega.Expect(string(outputFileContents)).To(gomega.Equal(expectedContents))
	})

	ginkgo.Describe("modifyRelabelConfigFields", func() {
		var (
			relabelConfig      map[interface{}]interface{}
			controllerType     string
			isOperatorEnabled  string
			expectedConfig     map[interface{}]interface{}
			expectedController string
			expectedOperator   string
		)

		ginkgo.BeforeEach(func() {
			relabelConfig = make(map[interface{}]interface{})
			controllerType = "daemonSet"
			isOperatorEnabled = "false"
			expectedConfig = make(map[interface{}]interface{})
			expectedController = "daemonSet"
			expectedOperator = "false"
		})

		ginkgo.It("should modify the regex field when regex is present", func() {
			relabelConfig["regex"] = "regex$$"
			expectedConfig["regex"] = "regex$"

			modifyRelabelConfigFields(relabelConfig, controllerType, isOperatorEnabled)

			gomega.Expect(relabelConfig).To(gomega.Equal(expectedConfig))
		})

		ginkgo.It("should modify the regex field when regex is present and controller type is not daemonSet", func() {
			relabelConfig["regex"] = "regex$$NODE_NAME$$NODE_IP"
			expectedConfig["regex"] = "regex$NODE_NAME$NODE_IP"

			modifyRelabelConfigFields(relabelConfig, "otherController", isOperatorEnabled)

			gomega.Expect(relabelConfig).To(gomega.Equal(expectedConfig))
		})

		ginkgo.It("should modify the regex field when regex is present and operator is enabled", func() {
			relabelConfig["regex"] = "regex$$NODE_NAME$$NODE_IP"
			expectedConfig["regex"] = "regex$NODE_NAME$NODE_IP"

			modifyRelabelConfigFields(relabelConfig, controllerType, "true")

			gomega.Expect(relabelConfig).To(gomega.Equal(expectedConfig))
		})

		ginkgo.It("should modify the replacement field when replacement is present", func() {
			relabelConfig["replacement"] = "replacement$$"
			expectedConfig["replacement"] = "replacement$"

			modifyRelabelConfigFields(relabelConfig, controllerType, isOperatorEnabled)

			gomega.Expect(relabelConfig).To(gomega.Equal(expectedConfig))
		})

		ginkgo.It("should modify the replacement field when replacement is present and controller type is not daemonSet", func() {
			relabelConfig["replacement"] = "replacement$$NODE_NAME$$NODE_IP"
			expectedConfig["replacement"] = "replacement$NODE_NAME$NODE_IP"

			modifyRelabelConfigFields(relabelConfig, "otherController", isOperatorEnabled)

			gomega.Expect(relabelConfig).To(gomega.Equal(expectedConfig))
		})

		ginkgo.It("should modify the replacement field when replacement is present and operator is enabled", func() {
			relabelConfig["replacement"] = "replacement$$NODE_NAME$$NODE_IP"
			expectedConfig["replacement"] = "replacement$NODE_NAME$NODE_IP"

			modifyRelabelConfigFields(relabelConfig, controllerType, "true")

			gomega.Expect(relabelConfig).To(gomega.Equal(expectedConfig))
		})
	})
})