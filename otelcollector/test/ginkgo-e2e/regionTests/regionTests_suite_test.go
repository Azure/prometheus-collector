package regionTests

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	"prometheus-collector/otelcollector/test/utils"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/monitor/azquery"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	K8sClient             *kubernetes.Clientset
	Cfg                   *rest.Config
	PrometheusQueryClient v1.API
	parmRuleName          string
	parmAmwResourceId     string
)

const namespace = "kube-system"
const containerName = "prometheus-collector"
const controllerLabelName = "rsName"
const controllerLabelValue = "ama-metrics"

func TestTest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Suite")
}

func init() {
	flag.StringVar(&parmRuleName, "parmRuleName", "", "Prometheus rule name to use in this test suite")
	flag.StringVar(&parmAmwResourceId, "parmAmwResourceId", "", "AMW resource id to use in this test suite")
}

var _ = BeforeSuite(func() {
	var err error
	K8sClient, Cfg, err = utils.SetupKubernetesClient()
	Expect(err).NotTo(HaveOccurred())

	amwQueryEndpoint := os.Getenv("AMW_QUERY_ENDPOINT")
	fmt.Printf("env (AMW_QUERY_ENDPOINT): %s\r\n", amwQueryEndpoint)
	Expect(amwQueryEndpoint).NotTo(BeEmpty())

	PrometheusQueryClient, err = utils.CreatePrometheusAPIClient(amwQueryEndpoint)
	Expect(err).NotTo(HaveOccurred())
	Expect(PrometheusQueryClient).NotTo(BeNil())

	fmt.Printf("parmRuleName: %s\r\n", parmRuleName)
	Expect(parmRuleName).ToNot(BeEmpty())

	fmt.Printf("parmAmwResourceId: %s\r\n", parmAmwResourceId)
	Expect(parmAmwResourceId).ToNot(BeEmpty())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
})

func readFile(fileName string, podName string) []string {
	fmt.Printf("Examining %s\r\n", fileName)
	var cmd []string = []string{"cat", fileName}
	stdout, _, err := utils.ExecCmd(K8sClient, Cfg, podName, containerName, namespace, cmd)
	Expect(err).To(BeNil())

	return strings.Split(stdout, "\n")
}

func writeLines(lines []string) int {
	count := 0
	for _, rawLine := range lines {
		//fmt.Printf("raw line #%d: %s\r\n", i, rawLine)
		line := strings.Trim(rawLine, " ")
		if len(line) > 0 {
			//fmt.Printf("line #%d: %s\r\n", i, line)
			fmt.Printf("%s\r\n", line)
			count++
		} else {
			fmt.Println("<empty line>")
		}
	}

	return count
}

func safeDereferenceFloatPtr(f *float64) float64 {
	if f != nil {
		return *f
	}
	return 0.0
}

var _ = Describe("Regions Suite", func() {

	const mdsdErrFileName = "/opt/microsoft/linuxmonagent/mdsd.err"
	const mdsdInfoFileName = "/opt/microsoft/linuxmonagent/mdsd.info"
	const mdsdWarnFileName = "/opt/microsoft/linuxmonagent/mdsd.warn"
	const metricsExtDebugLogFileName = "/MetricsExtensionConsoleDebugLog.log"
	const metricsextension = "/etc/mdsd.d/config-cache/metricsextension"
	const ERROR = "error"
	const WARN = "warn"

	var podName string = ""

	type metricExtConsoleLine struct {
		line   string
		dt     string
		status string
		data   string
	}

	BeforeEach(func() {
		v1Pod, err := utils.GetPodsWithLabel(K8sClient, namespace, controllerLabelName, controllerLabelValue)
		Expect(err).To(BeNil())
		Expect(len(v1Pod)).To(BeNumerically(">", 0))

		fmt.Printf("pod array length: %d\r\n", len(v1Pod))
		fmt.Printf("Available pods matching '%s'='%s'\r\n", controllerLabelName, controllerLabelValue)
		for _, p := range v1Pod {
			fmt.Println(p.Name)
		}

		if len(v1Pod) > 0 {
			podName = v1Pod[0].Name
			fmt.Printf("Choosing the pod: %s\r\n", podName)
		}

		Expect(podName).ToNot(BeEmpty())
	})

	Context("Examine selected files and directories", func() {

		It("Check that there are no errors in /opt/microsoft/linuxmonagent/mdsd.err", func() {

			numErrLines := writeLines(readFile(mdsdErrFileName, podName))
			if numErrLines > 0 {
				fmt.Printf("%s is not empty.\r\n", mdsdErrFileName)
				writeLines(readFile(mdsdInfoFileName, podName))
				writeLines(readFile(mdsdWarnFileName, podName))
			}
		})

		It("Enumerate all the 'error' or 'warning' records in /MetricsExtensionConsoleDebugLog.log", func() {

			var lines []string = readFile(metricsExtDebugLogFileName, podName)

			// for i := 0; i < 10; i++ {
			// 	line := lines[i]
			for _, line := range lines {
				//fmt.Printf("#line: %d, %s \r\n", i, line)

				var fields []string = strings.Fields(line)
				if len(fields) > 2 {
					metricExt := metricExtConsoleLine{line: line, dt: fields[0], status: fields[1], data: fields[2]}
					//fmt.Println(metricExt.status)
					status := strings.ToLower(metricExt.status)
					if strings.Contains(status, ERROR) || strings.Contains(status, WARN) {
						fmt.Println(line)
					}
				}
			}
		})

		It("Check that /etc/mdsd.d/config-cache/metricsextension exists", func() {

			var cmd []string = []string{"ls", "/etc/mdsd.d/config-cache/"}
			stdout, _, err := utils.ExecCmd(K8sClient, Cfg, podName, containerName, namespace, cmd)
			Expect(err).To(BeNil())

			metricsExtExists := false

			list := strings.Split(stdout, "\n")
			for i := 0; i < len(list) && !metricsExtExists; i++ {
				s := list[i]
				fmt.Println(s)
				metricsExtExists = (strings.Compare(s, "metricsextension") == 0)
			}

			Expect(metricsExtExists).To(BeTrue())

			fmt.Printf("%s exists.\r\n", metricsextension)
		})
	})

	Context("Examine Prometheus via the AMW", func() {
		It("Query for a metric", func() {
			query := "up"

			fmt.Printf("Examining metrics via the query: '%s'\r\n", query)

			warnings, result, err := utils.InstantQuery(PrometheusQueryClient, query)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())

			fmt.Println(result)
		})

		It("Check that the specified recording rule exists", func() {
			fmt.Printf("Examining the recording rule: %s", parmRuleName)

			warnings, result, err := utils.InstantQuery(PrometheusQueryClient, parmRuleName)

			fmt.Println(warnings)
			Expect(err).NotTo(HaveOccurred())

			fmt.Println(result)
		})

		It("Query Prometheus alerts", func() {
			warnings, result, err := utils.InstantQuery(PrometheusQueryClient, "alerts")

			fmt.Println(warnings)
			Expect(err).NotTo(HaveOccurred())

			fmt.Println(result)
		})

		It("Query Azure Monitor for AMW usage and limits metrics", func() {
			cred, err := azidentity.NewDefaultAzureCredential(nil)
			Expect(err).NotTo(HaveOccurred())

			client, err := azquery.NewMetricsClient(cred, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(client).ToNot(BeNil())

			var response azquery.MetricsClientQueryResourceResponse
			timespan := azquery.TimeInterval("PT30M")
			metricNames := "ActiveTimeSeriesLimit,ActiveTimeSeriesPercentUtilization"
			response, err = client.QueryResource(context.Background(),
				parmAmwResourceId,
				&azquery.MetricsClientQueryResourceOptions{
					Timespan:        to.Ptr(timespan),
					Interval:        to.Ptr("PT5M"),
					MetricNames:     &metricNames,
					Aggregation:     to.SliceOfPtrs(azquery.AggregationTypeAverage, azquery.AggregationTypeCount),
					Top:             nil,
					OrderBy:         to.Ptr("Average asc"),
					Filter:          nil,
					ResultType:      nil,
					MetricNamespace: nil,
				})

			Expect(err).NotTo(HaveOccurred())

			fmt.Printf("%d Metrics returned\r\n", len(response.Response.Value))
			for i, v := range response.Response.Value {
				var a azquery.Metric = *v
				fmt.Printf("ID[%d]: %s\r\n", i, *(a.ID))
				fmt.Printf("Timeseries length: %d\r\n", len(a.TimeSeries))
				for j, t := range a.TimeSeries {
					fmt.Printf("TimeSeries #%d\r\n", j)

					for k, d := range t.Data {
						// fmt.Printf("%d - ", k)
						// fmt.Print((*d).TimeStamp.GoString())
						fmt.Printf("%d - %s - Average(%f); Count(%f); Max(%f); Min(%f); Total(%f);\r\n", k,
							(*d).TimeStamp.GoString(),
							safeDereferenceFloatPtr((*d).Average),
							safeDereferenceFloatPtr((*d).Count),
							safeDereferenceFloatPtr((*d).Maximum),
							safeDereferenceFloatPtr((*d).Minimum),
							safeDereferenceFloatPtr((*d).Total))
					}
				}
			}
		})
	})
})
