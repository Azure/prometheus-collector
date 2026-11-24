package utils

import (
	"encoding/json"
	"fmt"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"slices"
	"strings"
	"time"
)

/*
 * The format of the response from the Prometheus UI API paths: /api/v1/*
 */
type APIResponse struct {
	Status    string           `json:"status"`
	Data      json.RawMessage  `json:"data"`
	ErrorType promv1.ErrorType `json:"errorType"`
	Error     string           `json:"error"`
	Warnings  []string         `json:"warnings,omitempty"`
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
func QueryPromUIFromPod(clientset *kubernetes.Clientset, cfg *rest.Config, namespace string, labelKey string, labelValue string, containerName string, queryPath string, isLinux bool, result *APIResponse) error {
	pods, err := GetPodsWithLabel(clientset, namespace, labelKey, labelValue)
	if err != nil {
		return err
	}

	for _, pod := range pods {
		// Execute the command and capture the output
		var command []string
		if isLinux {
			command = []string{"sh", "-c", fmt.Sprintf("curl \"http://localhost:9090%s\"", queryPath)}
		} else {
			command = []string{"powershell", "-c", fmt.Sprintf("(curl \"http://localhost:9090%s\" %s).Content", queryPath, "-UseBasicParsing")}
		}
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

	}

	return nil
}

/*
 * Exec into the container in a pod with the specified namespace and label and curl the Prometheus UI with the specified path.
 */
func QueryPromUIFromPods(clientset *kubernetes.Clientset, cfg *rest.Config, namespace string, labelKey string, labelValue string, containerName string, queryPath string, isLinux bool, isReplicaSet bool, result []*APIResponse) error {
	pods, err := GetPodsWithLabel(clientset, namespace, labelKey, labelValue)
	if err != nil {
		return err
	}
	return queryPromUIFromPodList(clientset, cfg, pods, namespace, containerName, queryPath, isLinux, isReplicaSet, result)
}

func queryPromUIFromPodList(clientset *kubernetes.Clientset, cfg *rest.Config, pods []corev1.Pod, namespace, containerName, queryPath string, isLinux, isReplicaSet bool, result []*APIResponse) error {
	for i, pod := range pods {
		// Execute the command and capture the output
		var command []string
		if isLinux {
			command = []string{"sh", "-c", fmt.Sprintf("curl \"http://localhost:9090%s\"", queryPath)}
		} else {
			command = []string{"powershell", "-c", fmt.Sprintf("(curl \"http://localhost:9090%s\" %s).Content", queryPath, "-UseBasicParsing")}
		}
		stdout, _, err := ExecCmd(clientset, cfg, pod.Name, containerName, namespace, command)
		if err != nil {
			return err
		}

		if stdout == "" {
			return fmt.Errorf("Curl for %s was empty", queryPath)
		}

		err = json.Unmarshal([]byte(stdout), &result[i])
		if err != nil {
			return fmt.Errorf("Failed to unmarshal the json: %s", err.Error())
		}

		if result[i].Status != "success" {
			return fmt.Errorf("Failed to query from Prometheus UI: %s", stdout)
		}

		if !isReplicaSet {
			return nil
		}
	}

	return nil
}

type TransitionTarget struct {
	ScrapePool       string            `json:"scrapePool"`
	Labels           map[string]string `json:"labels"`
	DiscoveredLabels map[string]string `json:"discoveredLabels"`
	Health           string            `json:"health"`
	LastError        string            `json:"lastError"`
	LastScrape       time.Time         `json:"lastScrape"`
}

type CollectorAPIResponse struct {
	*APIResponse
	PodName string
}

func QueryCollectorPromUI(path string) ([]CollectorAPIResponse, error) {
	pods, err := GetPodsWithLabel(K8sClient, "kube-system", "rsName", "ama-metrics")
	if err != nil {
		return nil, err
	}
	responses := make([]*APIResponse, len(pods))
	for i := range responses {
		responses[i] = &APIResponse{}
	}
	if err := queryPromUIFromPodList(K8sClient, Cfg, pods, "kube-system",
		"prometheus-collector", path, true, true, responses); err != nil {
		return nil, err
	}
	result := make([]CollectorAPIResponse, len(pods))
	for i, response := range responses {
		result[i] = CollectorAPIResponse{APIResponse: response, PodName: pods[i].Name}
	}
	return result, nil
}

func TransitionTargets() ([]TransitionTarget, error) {
	responses, err := QueryCollectorPromUI("/api/v1/targets")
	if err != nil {
		return nil, err
	}
	var targets []TransitionTarget
	for _, response := range responses {
		var data struct {
			ActiveTargets []TransitionTarget `json:"activeTargets"`
		}
		if err := json.Unmarshal(response.Data, &data); err != nil {
			return nil, err
		}
		targets = append(targets, data.ActiveTargets...)
	}
	return targets, nil
}

func ValidateTransitionTargets(targets []TransitionTarget, jobs []string, health string, after time.Time) error {
	for _, job := range jobs {
		found := false
		for _, target := range targets {
			if target.ScrapePool != job {
				continue
			}
			found = true
			if target.Health != health || target.LastScrape.Before(after) {
				return fmt.Errorf("%s must have a fresh %s scrape", job, health)
			}
			if health == "down" && !strings.Contains(target.LastError, "401") {
				return fmt.Errorf("%s must fail authentication, not an unrelated scrape error", job)
			}
		}
		if !found {
			return fmt.Errorf("missing expected monitor target %s", job)
		}
	}
	return nil
}

func ValidateAnnotationTargets(targets []TransitionTarget, allowed []string, after time.Time) error {
	found := map[string]bool{}
	for _, target := range targets {
		pod := target.Labels["kubernetes_pod_name"]
		if discovered := target.DiscoveredLabels["__meta_kubernetes_pod_name"]; discovered != "" {
			pod = discovered
		}
		if target.ScrapePool != "kubernetes-pods" || (pod != "annotation-target" && pod != "unannotated-target") {
			continue
		}
		namespace := target.Labels["kubernetes_namespace"]
		if discovered := target.DiscoveredLabels["__meta_kubernetes_namespace"]; discovered != "" {
			namespace = discovered
		}
		if !slices.Contains([]string{"ama-settings-included", "ama-settings-excluded", "ama-settings-included-similar"}, namespace) {
			continue
		}
		if target.Labels["kubernetes_namespace"] != namespace || target.Labels["kubernetes_pod_name"] != pod {
			return fmt.Errorf("annotation fixture %s/%s is missing its exported identity labels", namespace, pod)
		}
		if pod == "unannotated-target" || !slices.Contains(allowed, namespace) {
			return fmt.Errorf("excluded annotation fixture %s/%s remains active", namespace, pod)
		}
		if target.Health != "up" || target.LastScrape.Before(after) {
			return fmt.Errorf("annotation fixture %s/%s has no fresh healthy scrape", namespace, pod)
		}
		found[namespace] = true
	}
	for _, namespace := range allowed {
		if !found[namespace] {
			return fmt.Errorf("missing admitted annotation target in %s", namespace)
		}
	}
	return nil
}
