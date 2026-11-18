package configprocessing

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/prometheus/config"
	authv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilversion "k8s.io/apimachinery/pkg/util/version"
	"k8s.io/client-go/dynamic"

	"prometheus-collector/otelcollector/test/utils"
)

func monitorClient(resource, namespace string) dynamic.ResourceInterface {
	client, err := dynamic.NewForConfig(Cfg)
	Expect(err).NotTo(HaveOccurred())
	return client.Resource(schema.GroupVersionResource{
		Group: "azmonitoring.coreos.com", Version: "v1", Resource: resource,
	}).Namespace(namespace)
}

func monitorObject(resource, namespace string, endpoint map[string]interface{}) *unstructured.Unstructured {
	kind, endpoints := "ServiceMonitor", "endpoints"
	if resource == "podmonitors" {
		kind, endpoints = "PodMonitor", "podMetricsEndpoints"
	}
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "azmonitoring.coreos.com/v1", "kind": kind,
		"metadata": map[string]interface{}{"generateName": "ama-e2e-access-", "namespace": namespace},
		"spec": map[string]interface{}{
			"selector": map[string]interface{}{"matchLabels": map[string]interface{}{"app": "basic-auth-ref-app"}},
			endpoints:  []interface{}{endpoint},
		},
	}}
}

func createMonitor(resource, namespace string, endpoint map[string]interface{}) string {
	client := monitorClient(resource, namespace)
	monitor, err := client.Create(context.Background(), monitorObject(resource, namespace, endpoint),
		metav1.CreateOptions{FieldValidation: "Strict"})
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(func() {
		Expect(client.Delete(context.Background(), monitor.GetName(), metav1.DeleteOptions{})).To(Succeed())
	})
	return strings.ToLower(monitor.GetKind()[:1]) + monitor.GetKind()[1:] + "/" + namespace + "/" + monitor.GetName()
}

func validateMonitorJobs(jobs, allowed, denied []string) error {
	found := func(prefix string) bool {
		for _, job := range jobs {
			if strings.HasPrefix(job, prefix+"/") {
				return true
			}
		}
		return false
	}
	for _, prefix := range allowed {
		if !found(prefix) {
			return fmt.Errorf("allowed control %s was not reconciled", prefix)
		}
	}
	for _, prefix := range denied {
		if found(prefix) {
			return fmt.Errorf("disallowed monitor %s was generated", prefix)
		}
	}
	return nil
}

func expectMonitorJobs(allowed, denied []string) {
	check := func() error {
		responses, err := utils.QueryPromUIFromPods(K8sClient, Cfg, "kube-system", "rsName", "ama-metrics",
			"prometheus-collector", "/api/v1/status/config", true)
		if err != nil {
			return err
		}
		for _, response := range responses {
			var data promv1.ConfigResult
			if err := json.Unmarshal(response.Data, &data); err != nil {
				return err
			}
			cfg, err := config.Load(data.YAML, nil)
			if err != nil {
				return err
			}
			jobs := make([]string, 0, len(cfg.ScrapeConfigs))
			for _, job := range cfg.ScrapeConfigs {
				jobs = append(jobs, job.JobName)
			}
			if err := validateMonitorJobs(jobs, allowed, denied); err != nil {
				return fmt.Errorf("pod %s: %w", response.PodName, err)
			}
		}
		return nil
	}
	Eventually(check, 5*time.Minute, 15*time.Second).Should(Succeed())
	Consistently(check, time.Minute, 15*time.Second).Should(Succeed())
}

func allocatorInstance() (string, error) {
	pods, err := utils.GetPodsWithLabel(K8sClient, "kube-system", "rsName", "ama-metrics-operator-targets")
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

func allocatorSecretWatchReady() error {
	pods, err := utils.GetPodsWithLabel(K8sClient, "kube-system", "rsName", "ama-metrics-operator-targets")
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

var _ = Describe("Monitor filesystem access enforcement", Label(utils.ConfigProcessingDenyFSAccess), func() {
	It("drops ServiceMonitor file references while retaining a file-free control", func() {
		denied := []string{}
		for _, endpoint := range []map[string]interface{}{
			{"port": "ba-metrics", "bearerTokenFile": "/ama-e2e-nonexistent/token"},
			{"port": "ba-metrics", "tlsConfig": map[string]interface{}{"caFile": "/ama-e2e-nonexistent/ca"}},
			{"port": "ba-metrics", "tlsConfig": map[string]interface{}{
				"certFile": "/ama-e2e-nonexistent/cert", "keyFile": "/ama-e2e-nonexistent/key"}},
		} {
			denied = append(denied, createMonitor("servicemonitors", "default", endpoint))
		}
		allowed := createMonitor("servicemonitors", "default", map[string]interface{}{"port": "ba-metrics"})
		expectMonitorJobs([]string{allowed}, denied)
	})

	It("rejects unsupported PodMonitor file references at strict CRD admission", func() {
		client := monitorClient("podmonitors", "default")
		monitor, err := client.Create(context.Background(),
			monitorObject("podmonitors", "default", map[string]interface{}{
				"port": "ba-metrics", "bearerTokenFile": "/ama-e2e-nonexistent/token",
			}), metav1.CreateOptions{FieldValidation: "Strict"})
		if err == nil {
			DeferCleanup(func() {
				Expect(client.Delete(context.Background(), monitor.GetName(), metav1.DeleteOptions{})).To(Succeed())
			})
		}
		Expect(apierrors.IsBadRequest(err) || apierrors.IsInvalid(err)).To(BeTrue(), "PodMonitor schema must reject, not silently prune, unsupported local-file fields: %v", err)
		Expect(err.Error()).To(ContainSubstring("bearerTokenFile"))
	})
})

var _ = Describe("Monitor secret namespace enforcement", func() {
	check := func(empty bool) {
		version, err := K8sClient.Discovery().ServerVersion()
		Expect(err).NotTo(HaveOccurred())
		parsed, err := utilversion.ParseSemantic(version.GitVersion)
		Expect(err).NotTo(HaveOccurred())
		if !parsed.AtLeast(utilversion.MustParseSemantic("v1.37.0")) {
			Skip("scoped secret watching is supported on Kubernetes 1.37+; use the current auto-upgrading lane")
		}
		namespace := "default"
		secretName := "ama-e2e-denied-secret"
		if empty {
			namespace = "ama-secret-watch-test"
			secretName = "settings-auth-creds"
		}
		verbs := []string{"get"}
		if empty {
			verbs = []string{"get", "list", "watch"}
		}
		for _, verb := range verbs {
			review, err := K8sClient.AuthorizationV1().SubjectAccessReviews().Create(context.Background(),
				&authv1.SubjectAccessReview{Spec: authv1.SubjectAccessReviewSpec{
					User:   "system:serviceaccount:kube-system:ama-metrics-serviceaccount",
					Groups: []string{"system:serviceaccounts", "system:serviceaccounts:kube-system", "system:authenticated"},
					ResourceAttributes: &authv1.ResourceAttributes{
						Namespace: namespace, Verb: verb, Resource: "secrets", Name: secretName,
					},
				}}, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(review.Status.Allowed).To(Equal(empty), "unexpected %s secret RBAC in %s", verb, namespace)
		}
		cm, err := K8sClient.CoreV1().ConfigMaps("kube-system").Get(context.Background(),
			"ama-metrics-settings-configmap", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		match := regexp.MustCompile(`(?m)^\s*secrets_access_namespaces\s*=\s*"([^"]*)"`).
			FindStringSubmatch(cm.Data["prometheus-collector-settings"])
		namespaces := ""
		if len(match) > 1 {
			namespaces = strings.TrimSpace(match[1])
		}
		if empty {
			Expect(namespaces).To(BeEmpty(), "empty-allowlist fixture required")
		} else {
			Expect(namespaces).To(Equal("basic-auth-test"), "restricted-allowlist fixture required")
		}
		secret, err := K8sClient.CoreV1().Secrets(namespace).Create(context.Background(), &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: secretName},
			StringData: map[string]string{"username": "synthetic-user", "password": "synthetic-not-a-credential"},
		}, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
		if !empty {
			DeferCleanup(func() {
				Expect(K8sClient.CoreV1().Secrets(namespace).Delete(context.Background(), secret.Name, metav1.DeleteOptions{})).To(Succeed())
			})
		}
		// The empty-case secret is removed with its owned fixture namespace.
		allowed, denied := []string{}, []string{}
		secretJobs := []string{}
		for _, resource := range []string{"servicemonitors", "podmonitors"} {
			basic := func(name string) map[string]interface{} {
				return map[string]interface{}{"port": "ba-metrics", "path": "/httpsmetrics", "basicAuth": map[string]interface{}{
					"username": map[string]interface{}{"name": name, "key": "username"},
					"password": map[string]interface{}{"name": name, "key": "password"},
				}}
			}
			secretJob := createMonitor(resource, namespace, basic(secret.Name))
			if empty {
				allowed = append(allowed, secretJob)
				secretJobs = append(secretJobs, secretJob+"/0")
			} else {
				denied = append(denied, secretJob)
			}
			allowed = append(allowed, createMonitor(resource, namespace, map[string]interface{}{"port": "ba-metrics"}))
			if !empty {
				allowed = append(allowed, createMonitor(resource, "basic-auth-test", basic("basic-auth-creds")))
			}
		}
		expectMonitorJobs(allowed, denied)
		if !empty {
			return
		}
		Eventually(func() error { return monitorTransitionState(secretJobs, "synthetic-user", false) },
			5*time.Minute, 15*time.Second).Should(Succeed())
		instance, err := allocatorInstance()
		Expect(err).NotTo(HaveOccurred())
		secret.StringData = nil
		secret.Data = map[string][]byte{"username": []byte("synthetic-rotated"), "password": []byte("synthetic-not-a-credential")}
		_, err = K8sClient.CoreV1().Secrets(namespace).Update(context.Background(), secret, metav1.UpdateOptions{})
		Expect(err).NotTo(HaveOccurred())
		Consistently(func() error {
			current, err := allocatorInstance()
			if err != nil {
				return err
			}
			if current != instance {
				return fmt.Errorf("target allocator restarted during watch observation")
			}
			return monitorTransitionState(secretJobs, "synthetic-user", false)
		}, 5*time.Minute, 15*time.Second).Should(Succeed())

		restoreSettingsAfterTransition()
		cm.Data["prometheus-collector-settings"] = regexp.MustCompile(`(?m)^\s*secrets_access_namespaces\s*=.*\n?`).
			ReplaceAllString(cm.Data["prometheus-collector-settings"], "")
		cm.Data["prometheus-collector-settings"] += "\nsecrets_access_namespaces = \"" + namespace + "\"\n"
		replaceSettings(cm.Data)
		Eventually(func() error {
			current, err := allocatorInstance()
			if err != nil {
				return err
			}
			if current == instance {
				return fmt.Errorf("target allocator has not reloaded the allowlist")
			}
			return monitorTransitionState(secretJobs, "synthetic-rotated", false)
		}, 8*time.Minute, 15*time.Second).Should(Succeed())
		instance, err = allocatorInstance()
		Expect(err).NotTo(HaveOccurred())
		Eventually(allocatorSecretWatchReady, 5*time.Minute, 15*time.Second).Should(Succeed())
		secret, err = K8sClient.CoreV1().Secrets(namespace).Get(context.Background(), secret.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		secret.Data["username"] = []byte("synthetic-allowed")
		_, err = K8sClient.CoreV1().Secrets(namespace).Update(context.Background(), secret, metav1.UpdateOptions{})
		Expect(err).NotTo(HaveOccurred())
		Eventually(func() error {
			current, err := allocatorInstance()
			if err != nil {
				return err
			}
			if current != instance {
				return fmt.Errorf("target allocator restarted during positive watch control")
			}
			return monitorTransitionState(secretJobs, "synthetic-allowed", false)
		}, 5*time.Minute, 15*time.Second).Should(Succeed())
	}
	It("resolves allowlisted ServiceMonitor and PodMonitor secrets but rejects a nonallowlisted namespace",
		Label(utils.ConfigProcessingBasicAuthSmon, utils.ConfigProcessingBasicAuthSmonV2), func() { check(false) })
	It("does not watch secret updates with an empty namespace allowlist, but watches an allowlisted namespace",
		Label("config-processing-empty-secret-namespaces"), func() { check(true) })
})

func TestMonitorAccessAssertions(t *testing.T) {
	allowed, denied := []string{"serviceMonitor/default/allowed"}, []string{"podMonitor/default/denied"}
	if err := validateMonitorJobs([]string{allowed[0] + "/0"}, allowed, denied); err != nil {
		t.Fatal(err)
	}
	for _, jobs := range [][]string{nil, {allowed[0] + "/0", denied[0] + "/0"}, {allowed[0] + "-different/0"}} {
		if err := validateMonitorJobs(jobs, allowed, denied); err == nil {
			t.Fatalf("missing control or admitted forbidden monitor must fail: %v", jobs)
		}
	}
}
