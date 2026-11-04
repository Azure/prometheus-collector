package utils

import (
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"

	"k8s.io/client-go/kubernetes"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

/*
 * The format of the response from the Prometheus UI API paths: /api/v1/*
 */
type APIResponse struct {
	PodName   string          `json:"-"`
	Status    string          `json:"status"`
	Data      json.RawMessage `json:"data"`
	ErrorType v1.ErrorType    `json:"errorType"`
	Error     string          `json:"error"`
	Warnings  []string        `json:"warnings,omitempty"`
}

/*
 * The scrape pool data from the API response.
 */
type ScrapePoolData struct {
	ScrapePools []string `json:"scrapePools"`
}

/*
 * The Prometheus Config from the API response.
 */
type PrometheusConfigData struct {
	PrometheusConfigYAML string `json:"yaml"`
}

// QueryPromUIFromPod queries one pod; use QueryPromUIFromPods to assert every replica or node.
func QueryPromUIFromPod(clientset *kubernetes.Clientset, cfg *rest.Config, namespace string, labelKey string, labelValue string, containerName string, queryPath string, isLinux bool, result *APIResponse) error {
	pods, err := GetPodsWithLabel(clientset, namespace, labelKey, labelValue)
	if err != nil {
		return err
	}

	response, err := queryPromUIPod(clientset, cfg, namespace, pods[0].Name, containerName, queryPath, isLinux)
	if err != nil {
		return err
	}
	*result = *response
	return nil
}

func QueryPromUIFromPods(clientset *kubernetes.Clientset, cfg *rest.Config, namespace string, labelKey string, labelValue string, containerName string, queryPath string, isLinux bool) ([]*APIResponse, error) {
	pods, err := GetPodsWithLabel(clientset, namespace, labelKey, labelValue)
	if err != nil {
		return nil, err
	}
	return queryPromUIPods(pods, func(podName string) (*APIResponse, error) {
		return queryPromUIPod(clientset, cfg, namespace, podName, containerName, queryPath, isLinux)
	})
}

func queryPromUIPods(pods []corev1.Pod, query func(string) (*APIResponse, error)) ([]*APIResponse, error) {
	if len(pods) == 0 {
		return nil, fmt.Errorf("no collector pods selected")
	}
	responses := make([]*APIResponse, 0, len(pods))
	for _, pod := range pods {
		response, err := query(pod.Name)
		if err != nil {
			return nil, fmt.Errorf("pod %s: %w", pod.Name, err)
		}
		if response == nil {
			return nil, fmt.Errorf("pod %s returned no Prometheus response", pod.Name)
		}
		responses = append(responses, response)
	}
	return responses, nil
}

func queryPromUIPod(clientset *kubernetes.Clientset, cfg *rest.Config, namespace, podName, containerName, queryPath string, isLinux bool) (*APIResponse, error) {
	var command []string
	if isLinux {
		command = []string{"curl", "--fail", "--silent", "--show-error", "--noproxy", "*", "--max-time", "30",
			"http://localhost:9090" + queryPath}
	} else {
		command = []string{"powershell", "-c", fmt.Sprintf("(Invoke-WebRequest -UseBasicParsing -TimeoutSec 30 \"http://localhost:9090%s\").Content", queryPath)}
	}
	stdout, _, err := ExecCmd(clientset, cfg, podName, containerName, namespace, command)
	if err != nil {
		return nil, err
	}
	var response APIResponse
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		return nil, fmt.Errorf("invalid Prometheus response from pod %s: %w", podName, err)
	}
	if response.Status != "success" {
		return nil, fmt.Errorf("Prometheus query failed in pod %s: status=%q error=%q", podName, response.Status, response.Error)
	}
	response.PodName = podName
	return &response, nil
}
