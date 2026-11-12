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
		if empty {
			namespace = "ama-secret-watch-test"
			for _, verb := range []string{"get", "list", "watch"} {
				review, err := K8sClient.AuthorizationV1().SubjectAccessReviews().Create(context.Background(),
					&authv1.SubjectAccessReview{Spec: authv1.SubjectAccessReviewSpec{
						User:   "system:serviceaccount:kube-system:ama-metrics-serviceaccount",
						Groups: []string{"system:serviceaccounts", "system:serviceaccounts:kube-system", "system:authenticated"},
						ResourceAttributes: &authv1.ResourceAttributes{
							Namespace: namespace, Verb: verb, Resource: "secrets", Name: "ama-e2e-denied-secret",
						},
					}}, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(review.Status.Allowed).To(BeTrue(), "denial must come from the empty allowlist, not missing %s RBAC", verb)
			}
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
			ObjectMeta: metav1.ObjectMeta{Name: "ama-e2e-denied-secret"},
			StringData: map[string]string{"username": "synthetic-user", "password": "synthetic-not-a-credential"},
		}, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() {
			Expect(K8sClient.CoreV1().Secrets(namespace).Delete(context.Background(), secret.Name, metav1.DeleteOptions{})).To(Succeed())
		})
		allowed, denied := []string{}, []string{}
		for _, resource := range []string{"servicemonitors", "podmonitors"} {
			basic := func(name string) map[string]interface{} {
				return map[string]interface{}{"port": "ba-metrics", "path": "/httpsmetrics", "basicAuth": map[string]interface{}{
					"username": map[string]interface{}{"name": name, "key": "username"},
					"password": map[string]interface{}{"name": name, "key": "password"},
				}}
			}
			denied = append(denied, createMonitor(resource, namespace, basic(secret.Name)))
			allowed = append(allowed, createMonitor(resource, namespace, map[string]interface{}{"port": "ba-metrics"}))
			if !empty {
				allowed = append(allowed, createMonitor(resource, "basic-auth-test", basic("basic-auth-creds")))
			}
		}
		expectMonitorJobs(allowed, denied)
	}
	It("resolves allowlisted ServiceMonitor and PodMonitor secrets but rejects a nonallowlisted namespace",
		Label(utils.ConfigProcessingBasicAuthSmon, utils.ConfigProcessingBasicAuthSmonV2), func() { check(false) })
	It("watches no secrets with an empty namespace allowlist",
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
