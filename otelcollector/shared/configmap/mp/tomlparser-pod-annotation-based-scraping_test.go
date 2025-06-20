package configmapsettings

// import (
// 	"bufio"
// 	"fmt"
// 	"os"
// 	"strings"

// 	. "github.com/onsi/ginkgo/v2"
// 	. "github.com/onsi/gomega"
// )

// var _ = Describe("ConfigMapSettings", func() {
// 	Describe("parseConfigMapForPodAnnotations", func() {
// 		AfterEach(func() {
// 			cleanupEnvVars()
// 		})

// 		Context("when the config map file exists", func() {
// 			BeforeEach(func() {
// 				// Create a temporary file with the desired content
// 				fileContent := `podannotationnamespaceregex = "^namespace-regex|namespace-regex-2$"`
// 				file, err := os.CreateTemp("", "configmap")
// 				Expect(err).NotTo(HaveOccurred())
// 				defer file.Close()

// 				_, err = file.WriteString(fileContent)
// 				Expect(err).NotTo(HaveOccurred())

// 				// Set the configMapMountPathForPodAnnotation to the temporary file path
// 				configMapMountPathForPodAnnotation = file.Name()
// 				podAnnotationEnvVarPath = fmt.Sprintf("%s_out", configMapMountPathForPodAnnotation)

// 				setEnvVars(map[string]string{
// 					"AZMON_OPERATOR_ENABLED": "true",
// 					"CONTAINER_TYPE":         "ConfigReaderSidecar",
// 					"CONTROLLER_TYPE":        "ReplicaSet",
// 					"OS_TYPE":                "linux",
// 					"MODE":                   "advanced",
// 					"KUBE_STATE_NAME":        "ama-metrics-ksm",
// 					"POD_NAMESPACE":          "kube-system",
// 					"MAC":                    "true",
// 				})
// 			})

// 			AfterEach(func() {
// 				Expect(os.Remove(configMapMountPathForPodAnnotation)).To(Succeed())
// 			})

// 			It("should print the configmap namespace regex", func() {
// 				capturedOutput := captureOutput(func() {
// 					err := configurePodAnnotationSettings(make(map[string]map[string]string))
// 					Expect(err).NotTo(HaveOccurred())
// 				})

// 				Expect(capturedOutput).To(ContainSubstring("Using configmap namespace regex for podannotations: ^namespace-regex|namespace-regex-2$"))
// 			})

// 			It("should write the config to the output file", func() {
// 				err := configurePodAnnotationSettings(make(map[string]map[string]string))
// 				Expect(err).NotTo(HaveOccurred())

// 				content, err := os.ReadFile(podAnnotationEnvVarPath)
// 				Expect(err).NotTo(HaveOccurred())
// 				Expect(string(content)).To(Equal("AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX='^namespace-regex|namespace-regex-2$'\nAZMON_PROMETHEUS_POD_ANNOTATION_SCRAPING_ENABLED=true\n"))
// 			})
// 		})

// 		Context("when the config map file does not exist", func() {
// 			BeforeEach(func() {
// 				configMapMountPathForPodAnnotation = "/path/to/nonexistent/file"
// 				setEnvVars(map[string]string{
// 					"AZMON_OPERATOR_ENABLED": "true",
// 					"CONTAINER_TYPE":         "ConfigReaderSidecar",
// 					"CONTROLLER_TYPE":        "ReplicaSet",
// 					"OS_TYPE":                "linux",
// 					"MODE":                   "advanced",
// 					"KUBE_STATE_NAME":        "ama-metrics-ksm",
// 					"POD_NAMESPACE":          "kube-system",
// 					"MAC":                    "true",
// 				})
// 			})

// 			It("should return an error", func() {
// 				err := configurePodAnnotationSettings(map[string]map[string]string{})
// 				Expect(err).To(HaveOccurred())
// 				Expect(err.Error()).To(ContainSubstring("configmap section not mounted, using defaults"))
// 			})
// 		})

// 		Context("when the out file does not exist", func() {
// 			BeforeEach(func() {
// 				// Create a temporary file with the desired content
// 				fileContent := `podannotationnamespaceregex = "^namespace-regex|namespace-regex-2$"`
// 				file, err := os.CreateTemp("", "configmap")
// 				Expect(err).NotTo(HaveOccurred())
// 				defer file.Close()

// 				_, err = file.WriteString(fileContent)
// 				Expect(err).NotTo(HaveOccurred())

// 				// Set the configMapMountPathForPodAnnotation to the temporary file path
// 				configMapMountPathForPodAnnotation = file.Name()
// 				podAnnotationEnvVarPath = "/path/to/nonexistent/file"

// 				setEnvVars(map[string]string{
// 					"AZMON_OPERATOR_ENABLED": "true",
// 					"CONTAINER_TYPE":         "ConfigReaderSidecar",
// 					"CONTROLLER_TYPE":        "ReplicaSet",
// 					"OS_TYPE":                "linux",
// 					"MODE":                   "advanced",
// 					"KUBE_STATE_NAME":        "ama-metrics-ksm",
// 					"POD_NAMESPACE":          "kube-system",
// 					"MAC":                    "true",
// 				})
// 			})

// 			It("should return an error", func() {
// 				err := configurePodAnnotationSettings(map[string]map[string]string{})
// 				Expect(err).To(HaveOccurred())
// 				Expect(err.Error()).To(ContainSubstring("error opening file"))
// 			})
// 		})

// 		Context("when the config map file contains an invalid namespace regex", func() {
// 			BeforeEach(func() {
// 				// Create a temporary file with an invalid regex
// 				fileContent := `podannotationnamespaceregex = "invalid-regex("`
// 				file, err := os.CreateTemp("", "configmap")
// 				Expect(err).NotTo(HaveOccurred())
// 				defer file.Close()

// 				_, err = file.WriteString(fileContent)
// 				Expect(err).NotTo(HaveOccurred())

// 				// Set the configMapMountPathForPodAnnotation to the temporary file path
// 				configMapMountPathForPodAnnotation = file.Name()

// 				setEnvVars(map[string]string{
// 					"AZMON_OPERATOR_ENABLED": "true",
// 					"CONTAINER_TYPE":         "ConfigReaderSidecar",
// 					"CONTROLLER_TYPE":        "ReplicaSet",
// 					"OS_TYPE":                "linux",
// 					"MODE":                   "advanced",
// 					"KUBE_STATE_NAME":        "ama-metrics-ksm",
// 					"POD_NAMESPACE":          "kube-system",
// 					"MAC":                    "true",
// 				})
// 			})

// 			AfterEach(func() {
// 				Expect(os.Remove(configMapMountPathForPodAnnotation)).To(Succeed())
// 			})

// 			It("should return an error", func() {
// 				err := configurePodAnnotationSettings(map[string]map[string]string{})
// 				Expect(err).To(HaveOccurred())
// 				Expect(err.Error()).To(ContainSubstring("Invalid namespace regex for podannotations"))
// 			})
// 		})
// 	})
// })

// // Helper function to capture the output of fmt.Printf
// func captureOutput(f func()) string {
// 	old := os.Stdout
// 	r, w, _ := os.Pipe()
// 	os.Stdout = w

// 	f()

// 	w.Close()
// 	os.Stdout = old

// 	var buf strings.Builder
// 	scanner := bufio.NewScanner(r)
// 	for scanner.Scan() {
// 		buf.WriteString(scanner.Text())
// 		buf.WriteString("\n")
// 	}

// 	return buf.String()
// }

// // Functions setEnvVars and cleanupEnvVars are already defined in configmapparser_test.go
// // so they are removed from here to avoid redeclaration
