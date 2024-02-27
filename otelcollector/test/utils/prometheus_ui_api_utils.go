package utils

import (
	"encoding/json"

	"k8s.io/client-go/rest"

	"k8s.io/client-go/kubernetes"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"

	"fmt"
)

/*
 * The format of the response from the Prometheus UI API paths: /api/v1/*
 */
type APIResponse struct {
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

/*
 * Exec into the container in a pod with the specified namespace and label and curl the Prometheus UI with the specified path.
 */
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
