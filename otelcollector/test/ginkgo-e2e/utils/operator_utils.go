package utils

import (
	"context"
	"fmt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"strings"
)

func MonitorClient(resource, namespace string) dynamic.ResourceInterface {
	client, err := dynamic.NewForConfig(Cfg)
	Expect(err).NotTo(HaveOccurred())
	return client.Resource(schema.GroupVersionResource{
		Group: "azmonitoring.coreos.com", Version: "v1", Resource: resource,
	}).Namespace(namespace)
}

func MonitorObject(resource, namespace string, endpoint, labels map[string]interface{}) *unstructured.Unstructured {
	kind, endpoints := "ServiceMonitor", "endpoints"
	if resource == "podmonitors" {
		kind, endpoints = "PodMonitor", "podMetricsEndpoints"
	}
	metadata := map[string]interface{}{"generateName": "ama-e2e-access-", "namespace": namespace}
	if len(labels) > 0 {
		metadata["labels"] = labels
	}
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "azmonitoring.coreos.com/v1", "kind": kind,
		"metadata": metadata,
		"spec": map[string]interface{}{
			"selector": map[string]interface{}{"matchLabels": map[string]interface{}{"app": "basic-auth-ref-app"}},
			endpoints:  []interface{}{endpoint},
		},
	}}
}

func CreateMonitor(resource, namespace string, endpoint, labels map[string]interface{}) string {
	client := MonitorClient(resource, namespace)
	monitor, err := client.Create(context.Background(), MonitorObject(resource, namespace, endpoint, labels),
		metav1.CreateOptions{FieldValidation: "Strict"})
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(func() {
		Expect(client.Delete(context.Background(), monitor.GetName(), metav1.DeleteOptions{})).To(Succeed())
	})
	return strings.ToLower(monitor.GetKind()[:1]) + monitor.GetKind()[1:] + "/" + namespace + "/" + monitor.GetName()
}

func AllocatorInstance() (string, error) {
	pods, err := GetPodsWithLabel(K8sClient, "kube-system", "rsName", "ama-metrics-operator-targets")
	if err != nil {
		return "", err
	}
	if len(pods) != 1 {
		return "", fmt.Errorf("expected one target allocator pod, got %d", len(pods))
	}
	for _, status := range pods[0].Status.ContainerStatuses {
		if status.Name == "targetallocator" && status.Ready && status.ContainerID != "" {
			return string(pods[0].UID) + "/" + status.ContainerID, nil
		}
	}
	return "", fmt.Errorf("target allocator is not ready")
}

func AllocatorSecretWatchReady() error {
	pods, err := GetPodsWithLabel(K8sClient, "kube-system", "rsName", "ama-metrics-operator-targets")
	if err != nil {
		return err
	}
	if len(pods) != 1 {
		return fmt.Errorf("expected one target allocator pod, got %d", len(pods))
	}
	logs, err := K8sClient.CoreV1().Pods("kube-system").GetLogs(pods[0].Name,
		&corev1.PodLogOptions{Container: "targetallocator"}).DoRaw(context.Background())
	if err != nil {
		return err
	}
	if !strings.Contains(string(logs), "Using custom event handler for secrets informer") {
		return fmt.Errorf("target allocator secret watcher has not started")
	}
	return nil
}
