package regionTests_test

import (
	"fmt"
	"strings"
	"testing"

	"prometheus-collector/otelcollector/test/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func TestRegionTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RegionTests Suite")
}

var (
	K8sClient *kubernetes.Clientset
	Cfg       *rest.Config
)

const namespace = "kube-system"
const containerName = "prometheus-collector"
const controllerLabelName = "rsName"
const controllerLabelValue = "ama-metrics"

var _ = BeforeSuite(func() {
	var err error
	K8sClient, Cfg, err = utils.SetupKubernetesClient()

	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
})

var _ = Describe("Test", func() {

	var cmd []string
	var podName string = ""
	// var apiResponse utils.APIResponse

	BeforeEach(func() {
		cmd = []string{}

		v1Pod, err := utils.GetPodsWithLabel(K8sClient, namespace, controllerLabelName, controllerLabelValue)
		Expect(err).To(BeNil())

		fmt.Println("Available pods...")
		for _, p := range v1Pod {
			fmt.Println(p.Name)
		}

		if len(v1Pod) > 0 {
			podName = v1Pod[0].Name
		}
	})

	type metricExtConsoleLine struct {
		line   string
		dt     string
		status string
		data   string
	}

	type LineProcessor func(string) (bool, string)

	DescribeTable("Files Test",
		func(fileName string, proc LineProcessor) {
			Expect(podName).NotTo(BeEmpty())
			Expect(fileName).NotTo(BeEmpty())

			fmt.Printf("Examining %s\r\n", fileName)

			cmd = []string{"cat", fileName}
			stdout, _, err := utils.ExecCmd(K8sClient, Cfg, podName, containerName, namespace, cmd)
			Expect(err).To(BeNil())

			var lines []string = strings.Split(stdout, "\n")

			for _, rawLine := range lines {
				//fmt.Printf("raw line #%d: %s\r\n", i, rawLine)
				nonEmpty, formattedLine := proc(rawLine)
				if nonEmpty {
					//fmt.Printf("line #%d: %s\r\n", i, formattedLine)
					fmt.Printf("%s\r\n", formattedLine)
				} /*else {
					fmt.Println("<empty line>")
				}*/
			}
		},
		// func(fileName string, proc LineProcessor) string {
		// 	return fmt.Sprintf("Examining /opt/microsoft/linuxmonagent/%s", fileName)
		// },

		// Entry("Examine the contents of mdsd.info"),
		Entry(nil, "/opt/microsoft/linuxmonagent/mdsd.info", func(line string) (bool, string) {
			line = strings.Trim(line, " ")
			return len(line) > 0, line
		}),
		// Entry("Examine the contents of mdsd.err"),
		Entry(nil, "/opt/microsoft/linuxmonagent/mdsd.err", func(line string) (bool, string) {
			line = strings.Trim(line, " ")
			return len(line) > 0, line
		}),
		Entry(nil, "/MetricsExtensionConsoleDebugLog.log", func(line string) (bool, string) {
			line = strings.Trim(line, " ")

			var fields []string = strings.Fields(line)
			if len(fields) > 2 {
				metricExt := metricExtConsoleLine{line: line, dt: fields[0], status: fields[1], data: fields[2]}
				fmt.Println(metricExt.status)
			}

			return len(line) > 0, line
		}),
	)
})
