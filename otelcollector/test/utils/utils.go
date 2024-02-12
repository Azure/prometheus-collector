package utils

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/util/homedir"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"

	"bytes"
	"fmt"
	"io"

	"github.com/google/uuid"
)

func SetupKubernetesClient() (*kubernetes.Clientset, *rest.Config, error) {
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	cfg, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		cfg, err = rest.InClusterConfig()
		if err != nil {
			return nil, nil, err
		}
	}
	
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, nil, err
	}

	return client, cfg, nil
}

func CheckContainerLogsForErrors(clientset *kubernetes.Clientset, namespace, labelName, labelValue string) error {
	// Get all pods with the given label
	pods, err := GetPodsWithLabel(clientset, namespace, labelName, labelValue)
	if err != nil {
		return err
	}

	// Check the logs of each container in each pod for errors
	for _, pod := range pods {
		for _, container := range pod.Spec.Containers {
			logs, err := getContainerLogs(clientset, pod.Namespace, pod.Name, container.Name)
			if err != nil {
				return err
			}

			if strings.Contains(logs, "error") || strings.Contains(logs, "Error") {
				// Get the exact log line of the error
				for _, line := range strings.Split(logs, "\n") {

					if strings.Contains(line, "error") || strings.Contains(line, "Error") {

						// Exclude known error lines that are transient
						shouldExcludeLine := false
						for _, lineToExclude := range LogLineErrorsToExclude {
							if strings.Contains(line, lineToExclude) {
								shouldExcludeLine = true
								break
							}
						}
						if shouldExcludeLine {
							continue
						}

						return fmt.Errorf("Logs for container %s in pod %s contain errors:\n %s", container.Name, pod.Name, line)
					}
				}
			}
		}
	}
	return nil
}

func GetPodsWithLabel(clientset *kubernetes.Clientset, namespace string, labelKey string, labelValue string) ([]corev1.Pod, error) {
	podList, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelKey + "=" + labelValue,
	})
	if err != nil {
		return nil, err
	}
	if podList == nil || len(podList.Items) == 0{
		return nil, fmt.Errorf("no pods found with label %s=%s", labelKey, labelValue)
	}

	return podList.Items, nil
}

func getContainerLogs(clientset *kubernetes.Clientset, namespace string, podName string, containerName string) (string, error) {
	req := clientset.CoreV1().RESTClient().Get().
		Namespace(namespace).
		Name(podName).
		Resource("pods").
		SubResource("log").
		Param("container", containerName).
		Param("timestamps", "true")

	readCloser, err := req.Stream(context.Background())
	if err != nil {
		return "", fmt.Errorf("failed to get logs for container %s in pod %s: %v", containerName, podName, err)
	}
	defer readCloser.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, readCloser)
	if err != nil {
		return "", fmt.Errorf("failed to read logs for container %s in pod %s: %v", containerName, podName, err)
	}

	return buf.String(), nil
}

func CheckAllProcessesRunning(K8sClient *kubernetes.Clientset, Cfg *rest.Config, labelName, labelValue, namespace, containerName string, processes []string) error {
	var processesGrepStringBuilder strings.Builder
	for _, process := range processes {
		processesGrepStringBuilder.WriteString(fmt.Sprintf("ps | grep \"%s\" | grep -v grep && ", process))
	}
	processesGrepString := strings.TrimSuffix(processesGrepStringBuilder.String(), " && ")

	command := []string{"bash", "-c", processesGrepString}
	pods, err := GetPodsWithLabel(K8sClient, namespace, labelName, labelValue)
	if err != nil {
		return errors.New(fmt.Sprintf("Error when getting pods with label %s=%s: %v", labelName, labelValue, err))
	}

	for _, pod := range pods {
		_, _, err := ExecCmd(K8sClient, Cfg, pod.Name, containerName, namespace, command)
		if err != nil {
			return errors.New(fmt.Sprintf("Error when running command %v in the container: %v", command, err))
		}
	}
	return nil
}

func ExecCmd(client *kubernetes.Clientset, config *rest.Config, podName string, containerName string, namespace string, command []string) (stdout string, stderr string, err error) {
	req := client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec")
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		return "", "", errors.New(fmt.Sprintf("Error setting up exec request: %v", err))
	}

	parameterCodec := runtime.NewParameterCodec(scheme)
	req.VersionedParams(&corev1.PodExecOptions{
		Command:   command,
		Container: containerName,
		Stdin:     false,
		Stdout:    true,
		Stderr:    true,
		TTY:       false,
	}, parameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return "", "", errors.New(fmt.Sprintf("Error while creating command executor: %v", err))
	}

	var stdoutB, stderrB bytes.Buffer
	if err := exec.Stream(remotecommand.StreamOptions{
		Stdout: &stdoutB,
		Stderr: &stderrB,
	}); err != nil {
		return stdoutB.String(), stderrB.String(), errors.New(fmt.Sprintf("Error when running command %v in the container: %v", command, err))
	}

	return stdoutB.String(), stderrB.String(), nil
}

func CheckLivenessProbeRestartForProcess(K8sClient *kubernetes.Clientset, Cfg *rest.Config, labelName, labelValue, namespace, containerName, terminatedMessage, processName string, restartCommand []string, timeout int64) error {
	pods, err := GetPodsWithLabel(K8sClient, namespace, labelName, labelValue)
	if err != nil {
		return err
	}

	for _, pod := range pods {
		_, stderr, err := ExecCmd(K8sClient, Cfg, pod.Name, containerName, namespace, restartCommand)
		if err != nil {
			return err
		}
	
		if stderr != "" {
			return fmt.Errorf("stderr: %s", stderr)
		}

		err = WatchForPodRestart(K8sClient, namespace, labelName, labelValue, timeout, pod.Name, containerName, terminatedMessage)
		if err != nil {
			return err
		}
	}

	return nil
}

func WatchForPodRestart(K8sClient *kubernetes.Clientset, namespace, labelName, labelValue string, timeout int64, podName, containerName, terminatedMessage string) error {
	watcher, err := K8sClient.CoreV1().Pods(namespace).Watch(context.Background(), metav1.ListOptions{
		LabelSelector:   fmt.Sprintf("%s=%s", labelName, labelValue),
		TimeoutSeconds: &timeout,
	})
	if err != nil {
		return err
	}
	defer watcher.Stop()

	for {
		select {
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return fmt.Errorf(" %s pod did not restart before timeout", podName)
			}
			if event.Type != "MODIFIED" {
				continue
			}

			p, ok := event.Object.(*corev1.Pod)
			if !ok {
				return fmt.Errorf("event.Object is not of type *corev1.Pod")
			}

			fmt.Printf("Pod %s is status %v\n", p.Name, p.Status)

			for _, containerStatus := range p.Status.ContainerStatuses {
				if containerStatus.Name == containerName && containerStatus.LastTerminationState.Terminated != nil {
					if containerStatus.LastTerminationState.Terminated.Reason == "Error" {//&& strings.Contains(containerStatus.LastTerminationState.Terminated.Message, terminatedMessage) {
						return nil
					}
				}
			}
		}
		break
	}

	return nil
}

func CheckIfAllContainersAreRunning(clientset *kubernetes.Clientset, namespace, labelKey string, labelValue string) (error) {
	pods, err := GetPodsWithLabel(clientset, namespace, labelKey, labelValue)
	if err != nil {
		return errors.New(fmt.Sprintf("Error getting pods with the specified labels: %v", err))
	}

	for _, pod := range pods {
		if pod.Status.Phase != corev1.PodRunning {
			return errors.New(fmt.Sprintf("Pod is not runinng. Phase is: %v", pod.Status.Phase))
		}

		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.State.Running == nil {
				return errors.New(fmt.Sprintf("Container %s is not running", containerStatus.Name))
			}
		}
	}

	return nil
}

type APIResponse struct {
	Status    string          `json:"status"`
	Data      json.RawMessage `json:"data"`
	ErrorType v1.ErrorType    `json:"errorType"`
	Error     string          `json:"error"`
	Warnings  []string        `json:"warnings,omitempty"`
}

type ScrapePoolData struct {
	ScrapePools []string `json:"scrapePools"`
}

type PrometheusConfigData struct {
	PrometheusConfigYAML string `json:"yaml"`
}

func QueryPromUIFromPod(clientset *kubernetes.Clientset, cfg *rest.Config, namespace string, labelKey string, labelValue string, containerName string, queryPath string, result *APIResponse) (error) {
	pods, err := GetPodsWithLabel(clientset, namespace, labelKey, labelValue)
	if err != nil {
		return err
	}

	for _, pod := range pods {
		// Execute the command and capture the output
		command := []string{"sh", "-c", fmt.Sprintf("curl \"http://localhost:9090%s\"", queryPath)}
		stdout, _, err := ExecCmd(clientset, cfg, pod.Name, containerName, namespace, command)
		if err != nil {
			return err
		}

		if stdout == "" {
			return fmt.Errorf("Curl for %s was empty", queryPath)
		}

		err = json.Unmarshal([]byte(stdout), &result)
		if err != nil {
			return fmt.Errorf("Failed to unmarshal the json: %s", err.Error())
		}
	
		if result.Status != "success" {
			return fmt.Errorf("Failed to query from Prometheus UI: %s", stdout)
		}

		return nil
	}

	return nil
}

type TokenResponse struct {
	TokenType    string `json:"token_type"`
	ExpiresIn    string `json:"expires_in"`
	ExtExpiresIn string `json:"ext_expires_in"`
	ExpiresOn    string `json:"expires_on"`
	NotBefore    string `json:"not_before"`
	Resource     string `json:"resource"`
	AccessToken  string `json:"access_token"`
}

func GetQueryAccessToken() (string, error) {
	apiUrl := "https://login.microsoftonline.com/72f988bf-86f1-41af-91ab-2d7cd011db47/oauth2/token"
	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", os.Getenv("QUERY_ACCESS_CLIENT_ID"))
	data.Set("client_secret", os.Getenv("QUERY_ACCESS_CLIENT_SECRET"))
	data.Set("resource", "https://prometheus.monitor.azure.com")

	client := &http.Client{}
	r, err := http.NewRequest(http.MethodPost, apiUrl, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("Failed create request for authorization token: %s", err.Error())
	}
	r.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(r)
	if err != nil {
		return "", fmt.Errorf("Failed to request authorization token: %s", err.Error())
	}
	fmt.Printf("response: %v\n",resp)
	defer resp.Body.Close()
  body, err := ioutil.ReadAll(resp.Body)
  if err != nil {
		return "", fmt.Errorf("Failed to read body of auth token response: %s", err.Error())
	}

	var tokenResponse TokenResponse
	err = json.Unmarshal([]byte(body), &tokenResponse)
	if err != nil {
		return "", fmt.Errorf("Failed to unmarshal the token response: %s", err.Error())
	}

	return tokenResponse.AccessToken, nil
}

type transport struct {
	underlyingTransport http.RoundTripper
	apiToken string
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", t.apiToken))
	return t.underlyingTransport.RoundTrip(req)
}

func CreatePrometheusAPIClient() (v1.API, error) {
	token, err := GetQueryAccessToken()
	if err != nil {
		return nil, fmt.Errorf("Failed to get query access token: %s", err.Error())
	}
	if token == "" {
		return nil, fmt.Errorf("Failed to get query access token: token is empty")
	}
	config := api.Config{
		Address: os.Getenv("AMW_QUERY_ENDPOINT"),
		RoundTripper: &transport{underlyingTransport: http.DefaultTransport, apiToken: token},
	}
	prometheusAPIClient, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("Failed to create Prometheus API client: %s", err.Error())
	}
	return v1.NewAPI(prometheusAPIClient), nil
}

func RunQuery(api v1.API, query string) (v1.Warnings, error) {
	result, warnings, err := api.Query(context.Background(), query, time.Now())
	if err != nil {
		return warnings, fmt.Errorf("Failed to run query: %s", err.Error())
	}
	for _, sample := range result.(model.Vector) {
		fmt.Printf("Metric: %s\n", sample.Metric)
		fmt.Printf("Metric Name: %s\n", sample.Metric["__name__"])
		fmt.Printf("Cluster: %s\n", sample.Metric["cluster"])
		fmt.Printf("Job: %s\n", sample.Metric["job"])
		fmt.Printf("Instance: %s\n", sample.Metric["instance"])
		fmt.Printf("external_label_1: %s\n", sample.Metric["external_label_1"])
		fmt.Printf("external_label_123: %s\n", sample.Metric["external_label_123"])
		fmt.Printf("Value: %s\n", sample.Value)
		fmt.Printf("Timestamp: %s\n", sample.Timestamp)
		fmt.Printf("Histogram: %s\n", sample.Histogram)
	}

	return warnings, nil
}

func GetAndUpdateConfigMap(clientset *kubernetes.Clientset) error {
	namespace := "kube-system"
	configMapName := "ama-metrics-settings-configmap"
	ctx := context.Background()

	// Get the configmap
	configMap, err := clientset.CoreV1().ConfigMaps(namespace).Get(ctx, configMapName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Failed to get configmap: %s", err.Error())
	}

	// Update the configmap
	configMap.Data["test_field"] = uuid.New().String()
	_, err = clientset.CoreV1().ConfigMaps(namespace).Update(ctx, configMap, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("Failed to update configmap: %s", err.Error())
	}

	return nil
}
