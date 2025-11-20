package regionTests

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"

	"prometheus-collector/otelcollector/test/utils"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/monitor/azquery"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	K8sClient             *kubernetes.Clientset
	Cfg                   *rest.Config
	PrometheusQueryClient v1.API
	parmRuleName          string
	parmAmwResourceId     string
	azureClientId         string
	//parmKubeconfigPath    string //*************NEW - WTD***************************
	//verboseLogging        bool = false
)

const namespace = "kube-system"
const containerName = "prometheus-collector"
const controllerLabelName = "rsName"
const controllerLabelValue = "ama-metrics"
const AZURE_CLIENT_ID = "AZURE_CLIENT_ID"

func init() {
	//flag.StringVar(&parmKubeconfigPath, "kubeconfig", "", "Path to the kubeconfig file") //*************NEW - WTD***************************
	flag.StringVar(&parmRuleName, "parmRuleName", "", "Prometheus rule name to use in this test suite")
	flag.StringVar(&parmAmwResourceId, "parmAmwResourceId", "", "AMW resource id to use in this test suite")
	flag.StringVar(&azureClientId, "clientId", "", "Azure Client ID to use in this test suite")
}

func TestTest(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Suite")
}

var envConfig = cloud.Configuration{

	ActiveDirectoryAuthorityHost: "https://login.microsoftonline.eaglex.ic.gov/",
	Services: map[cloud.ServiceName]cloud.ServiceConfiguration{
		cloud.ResourceManager: {
			Endpoint: "https://management.azure.eaglex.ic.gov",
			Audience: "https://management.azure.eaglex.ic.gov",
		},
		azquery.ServiceNameMetrics: {
			Endpoint: "https://management.azure.eaglex.ic.gov",
			Audience: "https://management.azure.eaglex.ic.gov",
		},
	},
}

// // TODO: Move to setup_utils.go later - POSSIBLY UNNEEDED NOW //////////////////////////////
// func getKubeClient() (*kubernetes.Clientset, *rest.Config, error) {
// 	kubeconfig := os.Getenv("KUBECONFIG")
// 	if kubeconfig == "" {
// 		kubeconfig = filepath.Join(os.TempDir(), "kubeconfig.yaml")
// 	}

// 	fmt.Printf("env (KUBECONFIG): %s\r\n", kubeconfig)
// 	Expect(kubeconfig).NotTo(BeEmpty())

// 	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
// 	if err != nil {
// 		return nil, nil, err
// 	}

// 	client, err := kubernetes.NewForConfig(cfg)
// 	if err != nil {
// 		return nil, nil, err
// 	}

// 	return client, cfg, nil
// }

// // TODO: Need to parameterize the environment (cloud) configuration (including public -- the default)
// func createDefaultAzureCredential(options *azidentity.DefaultAzureCredentialOptions) (*azidentity.DefaultAzureCredential, error) {

// 	if options == nil {
// 		options = &azidentity.DefaultAzureCredentialOptions{}
// 	}

// 	strCloudEnv := os.Getenv("CLOUD_ENVIRONMENT")
// 	cloudEnv, err := utils.ParseCloudEnvironment(strCloudEnv)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to parse cloud environment: %w", err)
// 	}

// 	if cloudEnv != utils.Public {
// 		fmt.Printf("Using cloud environment: %s\r\n", strCloudEnv)

// 		var cloudConfig *cloud.Configuration
// 		cloudConfig, err = cloudEnv.ReadCloudConfig()
// 		if err != nil {
// 			return nil, fmt.Errorf("failed to read cloud config: %w", err)
// 		}

// 		options.ClientOptions.Cloud = *cloudConfig
// 		options.DisableInstanceDiscovery = true // reduces unnecessary discovery
// 	}

// 	cred, err := azidentity.NewDefaultAzureCredential(options)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to create default azure credential: %w", err)
// 	}
// 	return cred, nil
// }

// // TOTO: Figure out how to merge with GetQueryAccessToken() //////////////////////////////////////////////////
// func GetDefaultQueryAccessToken(scope string) (string, error) {

// 	if len(strings.TrimSpace(scope)) == 0 {
// 		return "", fmt.Errorf("scope is empty")
// 	}

// 	cred, err := utils.CreateDefaultAzureCredential(nil)
// 	if err != nil {
// 		return "", fmt.Errorf("Failed to create identity credential: %s", err.Error())
// 	}

// 	//"https://aksdemocluster-amw-aedfdtdva5erevau.westus2.prometheus.monitor.azure.com"
// 	// u, err := url.Parse(amwQueryEndpoint)
// 	// if err != nil {
// 	// 	return "", fmt.Errorf("invalid AMW_QUERY_ENDPOINT: %w", err)
// 	// }

// 	// scope := "https://usnateast.prometheus.monitor.azure.eaglex.ic.gov/.default" // e.g., https://prometheus.monitor.azure.eaglex.ic.gov/.default
// 	// fmt.Printf("Requesting access token for scope: %s\r\n", scope)

// 	// opts := policy.TokenRequestOptions{
// 	// 	Scopes: []string{scope},
// 	// }

// 	// accessToken, err := cred.GetToken(context.Background(), opts)
// 	// if err != nil {
// 	// 	//return "", fmt.Errorf("failed to get accesstoken: %s", err.Error())
// 	// 	fmt.Printf("failed to get accesstoken: %s\r\n", err.Error())
// 	// } else {
// 	// 	return accessToken.Token, nil
// 	// }

// 	// We are not using a regional prometheus scope.
// 	//scope := "https://prometheus.monitor.azure.eaglex.ic.gov/.default" // e.g., https://prometheus.monitor.azure.eaglex.ic.gov/.default

// 	fmt.Printf("Requesting access token for scope: %s\r\n", scope)
// 	opts := policy.TokenRequestOptions{
// 		Scopes: []string{scope},
// 	}

// 	accessToken, err := cred.GetToken(context.Background(), opts)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to get accesstoken: %s", err.Error())
// 	}

// 	return accessToken.Token, nil
// }

// func GetScopeFromEndpoint(amwQueryEndpoint string) (string, error) {
// 	u, err := url.Parse(amwQueryEndpoint)
// 	if err != nil {
// 		return "", fmt.Errorf("invalid AMW_QUERY_ENDPOINT: %w", err)
// 	}

// 	// Get the host (e.g., "aksdemocluster-amw-aedfdtdva5erevau.westus2.prometheus.monitor.azure.com")
// 	host := u.Host

// 	// Find "prometheus.monitor" and everything after it
// 	idx := strings.Index(host, "prometheus.monitor")
// 	if idx == -1 {
// 		return "", fmt.Errorf("invalid prometheus endpoint: missing 'prometheus.monitor' in host")
// 	}

// 	// Extract from "prometheus.monitor" onwards
// 	baseDomain := host[idx:]

// 	// Build the scope
// 	scope := fmt.Sprintf("https://%s/.default", baseDomain)
// 	return scope, nil
// }

// /*
//  * Create a Prometheus API client to use with the Managed Prometheus AMW Query API.
//  */
// //TODO: Pass Token into function.
// func CreatePromApiManagedClient(amwQueryEndpoint string) (v1.API, error) {
// 	scope, err := GetScopeFromEndpoint(amwQueryEndpoint)
// 	if err != nil {
// 		return nil, fmt.Errorf("Failed to get scope from endpoint: %s", err.Error())
// 	}

// 	token, err := GetDefaultQueryAccessToken(scope)
// 	if err != nil {
// 		return nil, fmt.Errorf("Failed to get query access token: %s", err.Error())
// 	}
// 	if token == "" {
// 		return nil, fmt.Errorf("Failed to get query access token: token is empty")
// 	}

// 	// Use the secure transport instead of the basic one
// 	secureTransport := utils.CreateSecureTransport(token)

// 	config := api.Config{
// 		Address:      amwQueryEndpoint,
// 		RoundTripper: secureTransport,
// 	}

// 	prometheusAPIClient, err := api.NewClient(config)
// 	if err != nil {
// 		return nil, fmt.Errorf("Failed to create Prometheus API client: %s", err.Error())
// 	}
// 	return v1.NewAPI(prometheusAPIClient), nil
// }

// // /*
// //  * The custom Prometheus API transport with the bearer token.
// //  */
// // type transport struct {
// // 	underlyingTransport http.RoundTripper
// // 	apiToken            string
// // }

// // /*
// //  * The custom RoundTrip with the bearer token added to the request header.
// //  */
// // func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
// // 	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", t.apiToken))
// // 	return t.underlyingTransport.RoundTrip(req)
// // }

// /*
//  * Enhanced transport with proper TLS certificate handling
//  */
// type secureTransport struct {
// 	underlyingTransport http.RoundTripper
// 	apiToken            string
// }

// /*
//  * The secure RoundTrip with proper certificate validation
//  */
// func (t *secureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
// 	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", t.apiToken))
// 	return t.underlyingTransport.RoundTrip(req)
// }

// /*
//  * Create a custom HTTP transport with proper certificate configuration
//  */
// func createSecureTransport(token string) *secureTransport {
// 	// Create a custom TLS config that uses system certificates + any custom ones
// 	tlsConfig := &tls.Config{
// 		MinVersion: tls.VersionTLS12, // Ensure modern TLS
// 	}

// 	// Load system certificate pool
// 	certPool, err := x509.SystemCertPool()
// 	if err != nil {
// 		fmt.Printf("Warning: Could not load system cert pool: %v\n", err)
// 		certPool = x509.NewCertPool()
// 	}

// 	// Check for custom certificate file (set by PowerShell script)
// 	certFile := os.Getenv("SSL_CERT_FILE")
// 	if certFile != "" {
// 		fmt.Printf("Loading additional certificates from: %s\n", certFile)
// 		certs, err := ioutil.ReadFile(certFile)
// 		if err == nil {
// 			if ok := certPool.AppendCertsFromPEM(certs); !ok {
// 				fmt.Printf("Warning: Could not parse certificates from %s\n", certFile)
// 			} else {
// 				fmt.Printf("✓ Successfully loaded additional certificates\n")
// 			}
// 		} else {
// 			fmt.Printf("Warning: Could not read certificate file: %v\n", err)
// 		}
// 	} else {
// 		fmt.Println("No additional SSL_CERT_FILE specified, using system certificates only.")
// 	}

// 	tlsConfig.RootCAs = certPool

// 	// Create transport with custom TLS config
// 	transport := &http.Transport{
// 		TLSClientConfig: tlsConfig,
// 	}

// 	return &secureTransport{
// 		underlyingTransport: transport,
// 		apiToken:            token,
// 	}
// }

//END -- TODO: move to amw_query_api_utils.go//////////////////////////////

var _ = BeforeSuite(func() {
	var err error
	fmt.Println("Getting kube client")
	////WTD K8sClient, Cfg, err = getKubeClient() ////************ utils.SetupKubernetesClient()
	K8sClient, Cfg, err = utils.SetupKubernetesClient()
	Expect(err).NotTo(HaveOccurred())

	amwQueryEndpoint := os.Getenv("AMW_QUERY_ENDPOINT")
	fmt.Printf("env (AMW_QUERY_ENDPOINT): %s\r\n", amwQueryEndpoint)
	Expect(amwQueryEndpoint).NotTo(BeEmpty())

	////PrometheusQueryClient, err = utils.CreatePrometheusAPIClient(amwQueryEndpoint)
	fmt.Println("Getting prom api client")
	PrometheusQueryClient, err = createPromApiManagedClient(amwQueryEndpoint)
	Expect(err).NotTo(HaveOccurred())
	Expect(PrometheusQueryClient).NotTo(BeNil())

	fmt.Printf("parmRuleName: %s\r\n", parmRuleName)
	Expect(parmRuleName).ToNot(BeEmpty())

	fmt.Printf("parmAmwResourceId: %s\r\n", parmAmwResourceId)
	Expect(parmAmwResourceId).ToNot(BeEmpty())

	// fmt.Printf("parmVerbose: %s\r\n", parmVerbose)
	// Expect(strings.ToLower(parmVerbose)).To(BeElementOf([]string{"true", "false"}), "parmVerbose must be either 'true' or 'false'.")

	////azureClientId = os.Getenv(AZURE_CLIENT_ID)
	fmt.Printf("Setting env variable %s to %s\r\n", AZURE_CLIENT_ID, azureClientId)
	_ = os.Setenv(AZURE_CLIENT_ID, azureClientId)
	fmt.Printf("azureClientId: %s\r\n", azureClientId)
	Expect(azureClientId).NotTo(BeEmpty())
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

			Expect(numErrLines).To(Equal(0))
		})

		It("Enumerate all the 'error' or 'warning' records in /MetricsExtensionConsoleDebugLog.log", func() {

			var lines []string = readFile(metricsExtDebugLogFileName, podName)
			count := 0

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
						count++
					}
				}
			}

			Expect(count).To(Equal(0))
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

			// Ensure there is at least one result
			vectorResult, ok := result.(model.Vector)
			Expect(ok).To(BeTrue(), "Result should be of type model.Vector")
			Expect(vectorResult).NotTo(BeEmpty(), "Result should not be empty")

			fmt.Printf("%d metrics were returned from the query.\r\n", vectorResult.Len())
		})

		It("Query the specified recording rule", func() {
			fmt.Printf("Examining the recording rule: %s", parmRuleName)

			warnings, result, err := utils.InstantQuery(PrometheusQueryClient, parmRuleName)

			fmt.Println(warnings)
			Expect(err).NotTo(HaveOccurred())

			// Ensure there is at least one result
			vectorResult, ok := result.(model.Vector)
			Expect(ok).To(BeTrue(), "Result should be of type model.Vector")
			Expect(vectorResult).NotTo(BeEmpty(), "Result should not be empty")

			fmt.Printf("%d metrics were returned from the recording rule.\r\n", vectorResult.Len())
		})

		It("Query Prometheus alerts", func() {
			fmt.Println("Querying Prometheus alerts")

			warnings, result, err := utils.InstantQuery(PrometheusQueryClient, "alerts")

			fmt.Println(warnings)
			Expect(err).NotTo(HaveOccurred())

			fmt.Println(result)
			fmt.Printf("Instant query results: %+v\r\n", result.(model.Value).String())
		})

		It("Query Azure Monitor for AMW usage and limits metrics", func() {
			////cred, err := azidentity.NewDefaultAzureCredential(nil)

			// Create a credential using the specified client ID
			cred, err := azidentity.NewManagedIdentityCredential(&azidentity.ManagedIdentityCredentialOptions{
				ID: azidentity.ClientID(azureClientId),
				ClientOptions: azcore.ClientOptions{
					Cloud: envConfig,
				},
			})
			if err != nil {
				log.Fatalf("failed to create managed identity credential: %v", err)
			}

			Expect(err).NotTo(HaveOccurred())

			// Options need to be passed to the "credential" and the "client" outside of the public cloud.
			client, err := azquery.NewMetricsClient(cred,
				&azquery.MetricsClientOptions{
					ClientOptions: azcore.ClientOptions{
						Cloud: envConfig,
					},
				},
			)
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

				Expect(a.TimeSeries).NotTo(BeNil())
				for j, t := range a.TimeSeries {
					fmt.Printf("TimeSeries #%d\r\n", j)

					Expect(t.Data).NotTo(BeNil())
					for k, d := range t.Data {
						// fmt.Printf("%d - ", k)
						// fmt.Print((*d).TimeStamp.GoString())
						fmt.Printf("%d - %s - Average(%f); Count(%f); Max(%f); Min(%f); Total(%f);\r\n",
							k, (*d).TimeStamp.GoString(),
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
