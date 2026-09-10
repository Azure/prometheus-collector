package configmapsettings

import (
	"bytes"
	"log"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus-collector/shared"
)

var _ = Describe("ConfigMapSettings", func() {
	Describe("parseConfigMapForPodAnnotations", func() {
		var configDir string
		var metricsConfigBySection map[string]map[string]string

		BeforeEach(func() {
			configDir = GinkgoT().TempDir()
			setTestPath(&configMapMountPathForPodAnnotation, filepath.Join(configDir, "pod-annotation-based-scraping"))
			setTestPath(&podAnnotationEnvVarPath, filepath.Join(GinkgoT().TempDir(), "pod-annotation-env"))
			metricsConfigBySection = nil
		})

		parseSettings := func(content string) {
			Expect(os.WriteFile(configMapMountPathForPodAnnotation, []byte(content), 0600)).To(Succeed())
			var err error
			metricsConfigBySection, err = shared.ParseV1Config(configDir)
			Expect(err).NotTo(HaveOccurred())
		}

		Context("when the config map file exists", func() {
			BeforeEach(func() {
				parseSettings(`podannotationnamespaceregex = "^namespace-regex|namespace-regex-2$"`)
			})

			It("should log the configmap namespace regex", func() {
				var output bytes.Buffer
				originalOutput := log.Writer()
				log.SetOutput(&output)
				DeferCleanup(log.SetOutput, originalOutput)

				Expect(configurePodAnnotationSettings(metricsConfigBySection)).To(Succeed())
				Expect(output.String()).To(ContainSubstring("Using configmap namespace regex for pod annotations: ^namespace-regex|namespace-regex-2$"))
			})

			It("should write the config to the output file", func() {
				Expect(configurePodAnnotationSettings(metricsConfigBySection)).To(Succeed())

				content, err := os.ReadFile(podAnnotationEnvVarPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(Equal("AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX='^namespace-regex|namespace-regex-2$'\nAZMON_PROMETHEUS_POD_ANNOTATION_SCRAPING_ENABLED=true\n"))
			})
		})

		It("should report when the config map directory does not exist", func() {
			var err error
			metricsConfigBySection, err = shared.ParseV1Config(filepath.Join(configDir, "missing"))
			Expect(err).To(MatchError(ContainSubstring("failed to read config directory")))

			Expect(configurePodAnnotationSettings(metricsConfigBySection)).To(MatchError("configmap section not mounted, using defaults"))
			Expect(podAnnotationEnvVarPath).NotTo(BeAnExistingFile())
		})

		It("should report an error opening the output file", func() {
			parseSettings(`podannotationnamespaceregex = "^namespace-regex|namespace-regex-2$"`)
			podAnnotationEnvVarPath = filepath.Join(configDir, "missing", "pod-annotation-env")

			Expect(configurePodAnnotationSettings(metricsConfigBySection)).To(MatchError(ContainSubstring("error opening file")))
		})

		It("should reject an invalid namespace regex without writing the output file", func() {
			parseSettings(`podannotationnamespaceregex = "invalid-regex("`)

			Expect(configurePodAnnotationSettings(metricsConfigBySection)).To(MatchError("Invalid namespace regex for pod annotations: invalid-regex("))
			Expect(podAnnotationEnvVarPath).NotTo(BeAnExistingFile())
		})

		It("should report a missing annotation section", func() {
			Expect(configurePodAnnotationSettings(map[string]map[string]string{})).To(MatchError("pod annotation namespace regex configuration not found"))
			Expect(podAnnotationEnvVarPath).NotTo(BeAnExistingFile())
		})

		It("should reject an empty namespace regex", func() {
			parseSettings(`podannotationnamespaceregex = ""`)

			Expect(configurePodAnnotationSettings(metricsConfigBySection)).To(MatchError("pod annotation namespace regex does not have a value"))
			Expect(podAnnotationEnvVarPath).NotTo(BeAnExistingFile())
		})
	})
})
