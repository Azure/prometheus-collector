package configmapsettings

import (
	"bytes"
	"log"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pod annotation settings", func() {
	var settings map[string]map[string]string

	BeforeEach(func() {
		original := podAnnotationEnvVarPath
		DeferCleanup(func() { podAnnotationEnvVarPath = original })
		podAnnotationEnvVarPath = filepath.Join(GinkgoT().TempDir(), "pod-env")
		settings = map[string]map[string]string{
			"pod-annotation-based-scraping": {"podannotationnamespaceregex": "^namespace-regex|namespace-regex-2$"},
		}
	})

	It("logs and writes the unquoted namespace regex to the environment file", func() {
		var output bytes.Buffer
		original := log.Writer()
		log.SetOutput(&output)
		defer log.SetOutput(original)
		Expect(configurePodAnnotationSettings(settings)).To(Succeed())
		Expect(output.String()).To(ContainSubstring("Using configmap namespace regex for pod annotations: ^namespace-regex|namespace-regex-2$"))
		content, err := os.ReadFile(podAnnotationEnvVarPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(content)).To(Equal("AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX=^namespace-regex|namespace-regex-2$\nAZMON_PROMETHEUS_POD_ANNOTATION_SCRAPING_ENABLED=true\n"))
	})

	It("rejects a missing parsed config map", func() {
		err := configurePodAnnotationSettings(nil)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("configmap section not mounted"))
	})

	It("reports an unwritable output file", func() {
		podAnnotationEnvVarPath = filepath.Join(GinkgoT().TempDir(), "missing", "env")
		err := configurePodAnnotationSettings(settings)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("error opening file"))
	})

	It("rejects an invalid namespace regex", func() {
		settings["pod-annotation-based-scraping"]["podannotationnamespaceregex"] = "invalid-regex("
		err := configurePodAnnotationSettings(settings)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Invalid namespace regex for pod annotations"))
	})
})
