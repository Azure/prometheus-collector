package utils_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"prometheus-collector/otelcollector/test/utils"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/apimachinery/pkg/util/httpstream/spdy"
	"k8s.io/apimachinery/pkg/util/remotecommand"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func TestQueryCollectorPromUIUsesOnePodList(t *testing.T) {
	first := []corev1.Pod{
		{ObjectMeta: metav1.ObjectMeta{Name: "collector-a"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "collector-b"}},
	}
	for _, test := range []struct {
		name string
		next []corev1.Pod
	}{
		{"reordered", []corev1.Pod{first[1], first[0]}},
		{"growing", append(slices.Clone(first), corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "collector-c"}})},
	} {
		t.Run(test.name, func(t *testing.T) {
			var lists atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v1/namespaces/kube-system/pods" {
					pods := first
					if lists.Add(1) > 1 {
						pods = test.next
					}
					w.Header().Set("Content-Type", "application/json")
					if err := json.NewEncoder(w).Encode(corev1.PodList{Items: pods}); err != nil {
						t.Error(err)
					}
					return
				}
				pod, ok := strings.CutPrefix(r.URL.Path, "/api/v1/namespaces/kube-system/pods/")
				if !ok || !strings.HasSuffix(pod, "/exec") {
					http.NotFound(w, r)
					return
				}
				pod = strings.TrimSuffix(pod, "/exec")
				if _, err := httpstream.Handshake(r, w, remotecommand.SupportedStreamingProtocols); err != nil {
					t.Error(err)
					return
				}
				conn := spdy.NewResponseUpgrader().UpgradeResponse(w, r, func(stream httpstream.Stream, replySent <-chan struct{}) error {
					go func() {
						<-replySent
						if stream.Headers().Get(corev1.StreamType) == corev1.StreamTypeStdout {
							if err := json.NewEncoder(stream).Encode(map[string]any{
								"status": "success", "data": map[string]string{"pod": pod},
							}); err != nil {
								t.Error(err)
							}
						}
						if err := stream.Close(); err != nil {
							t.Error(err)
						}
					}()
					return nil
				})
				if conn != nil {
					<-conn.CloseChan()
				}
			}))
			defer server.Close()
			cfg := &rest.Config{Host: server.URL, ContentConfig: rest.ContentConfig{ContentType: "application/json"}}
			client, err := kubernetes.NewForConfig(cfg)
			if err != nil {
				t.Fatal(err)
			}
			oldClient, oldCfg := utils.K8sClient, utils.Cfg
			utils.K8sClient, utils.Cfg = client, cfg
			defer func() { utils.K8sClient, utils.Cfg = oldClient, oldCfg }()

			responses, err := utils.QueryCollectorPromUI("/api/v1/targets")
			if err != nil {
				t.Fatal(err)
			}
			if got := lists.Load(); got != 1 {
				t.Errorf("listed collector pods %d times, want once", got)
			}
			if len(responses) != len(first) {
				t.Fatalf("got %d responses, want %d", len(responses), len(first))
			}
			for i, response := range responses {
				var data struct {
					Pod string `json:"pod"`
				}
				if err := json.Unmarshal(response.Data, &data); err != nil {
					t.Fatal(err)
				}
				if data.Pod != first[i].Name || response.PodName != first[i].Name {
					t.Errorf("response from %q attributed to %q, want %q", data.Pod, response.PodName, first[i].Name)
				}
			}
		})
	}
}

func TestWaitForCollectorsSettledWithoutFollowOnRestart(t *testing.T) {
	const childEnv = "AMA_TEST_COLLECTOR_SETTLING"
	if os.Getenv(childEnv) != "1" {
		// Ginkgo requires -count=1, so isolate it from repeated unit test runs.
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestWaitForCollectorsSettledWithoutFollowOnRestart$")
		cmd.Env = append(os.Environ(), childEnv+"=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("collector settling failed: %v\n%s", err, output)
		}
		return
	}

	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.Describe("collector settling", func() {
		ginkgo.It("does not wait for another restart", func() {
			since := time.Now().Add(-5 * time.Minute)
			started := metav1.NewTime(since.Add(time.Minute))
			var statuses []corev1.ContainerStatus
			for _, name := range []string{"prometheus-collector", "config-reader", "targetallocator"} {
				statuses = append(statuses, corev1.ContainerStatus{
					Name: name, Ready: true,
					State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: started}},
				})
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				var response any = corev1.PodList{
					TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "PodList"},
					Items:    []corev1.Pod{{Status: corev1.PodStatus{ContainerStatuses: statuses}}},
				}
				if r.URL.Path == "/api/v1/namespaces/kube-system/configmaps/ama-metrics-settings-configmap" {
					config := corev1.ConfigMap{
						TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
						ObjectMeta: metav1.ObjectMeta{Name: "ama-metrics-settings-configmap", Namespace: "kube-system"},
						Data:       map[string]string{"config-version": "old"},
					}
					if r.Method == http.MethodPut {
						if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
							http.Error(w, err.Error(), http.StatusBadRequest)
							return
						}
					}
					response = config
				}
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()

			client, err := kubernetes.NewForConfig(&rest.Config{
				Host: server.URL, ContentConfig: rest.ContentConfig{ContentType: "application/json"},
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			previous := utils.K8sClient
			utils.K8sClient = client
			defer func() { utils.K8sClient = previous }()
			utils.WaitForCollectorsSettled(since)
			utils.ReplaceSettings(map[string]string{"config-version": "changed"})
			utils.WaitForCollectorsSettled(time.Time{})
		})
	})
	ginkgo.RunSpecs(t, "Collector settling")
}

func TestCollectorSettleGuard(t *testing.T) {
	now := time.Now()
	since := now.Add(-5 * time.Minute)
	podStartedAt := func(startedAt time.Time, ready bool) corev1.Pod {
		return corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "ama-metrics-test"},
			Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{
				Name: "prometheus-collector", Ready: ready,
				State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(startedAt)}},
			}}},
		}
	}
	quiet := utils.CollectorSettleQuietPeriod
	if err := utils.ValidateContainerSettled(podStartedAt(since.Add(time.Minute), true), "prometheus-collector", since, now, quiet); err != nil {
		t.Fatalf("a collector reloaded after the change and quiet for 4m must be settled: %v", err)
	}
	if err := utils.ValidateContainerSettled(podStartedAt(now.Add(-time.Minute), true), "prometheus-collector", since, now, quiet); err == nil {
		t.Fatal("a collector that restarted within the quiet period (follow-on liveness restart) must not be settled")
	}
	if err := utils.ValidateContainerSettled(podStartedAt(since.Add(-time.Minute), true), "prometheus-collector", since, now, quiet); err == nil {
		t.Fatal("a collector that has not reloaded since the settings change must not be settled")
	}
	if err := utils.ValidateContainerSettled(podStartedAt(since.Add(-time.Minute), true), "prometheus-collector", time.Time{}, now, quiet); err != nil {
		t.Fatalf("without a settings change only the quiet period applies: %v", err)
	}
	if err := utils.ValidateContainerSettled(podStartedAt(since.Add(time.Minute), false), "prometheus-collector", since, now, quiet); err == nil {
		t.Fatal("an unready collector must not be settled")
	}
	if err := utils.ValidateContainerSettled(podStartedAt(since.Add(time.Minute), true), "config-reader", since, now, quiet); err == nil {
		t.Fatal("a missing container status must not be settled")
	}
}

func TestAnnotationTargetSet(t *testing.T) {
	now := time.Now()
	target := func(namespace, pod string) utils.TransitionTarget {
		return utils.TransitionTarget{
			ScrapePool: "kubernetes-pods", Health: "up", LastScrape: now,
			Labels:           map[string]string{"kubernetes_namespace": namespace, "kubernetes_pod_name": pod},
			DiscoveredLabels: map[string]string{"__meta_kubernetes_namespace": namespace, "__meta_kubernetes_pod_name": pod},
		}
	}
	allowed := []string{"ama-settings-included", "ama-settings-excluded"}
	valid := []utils.TransitionTarget{target(allowed[0], "annotation-target"), target(allowed[1], "annotation-target")}
	if err := utils.ValidateAnnotationTargets(valid, allowed, now); err != nil {
		t.Fatal(err)
	}
	wildcard := append(slices.Clone(valid), target("ama-settings-included-similar", "annotation-target"),
		target("unrelated-workload", "annotation-target"))
	if err := utils.ValidateAnnotationTargets(wildcard, append(slices.Clone(allowed), "ama-settings-included-similar"), now); err != nil {
		t.Fatal(err)
	}
	unhealthy := slices.Clone(valid)
	unhealthy[1].Health = "down"
	stale := slices.Clone(valid)
	stale[1].LastScrape = now.Add(-time.Second)
	missingIdentity := slices.Clone(valid)
	missingIdentity[1].Labels = nil
	for _, test := range []struct {
		name    string
		targets []utils.TransitionTarget
		allowed []string
	}{
		{"missing-all", nil, allowed},
		{"missing-second-alternative", valid[:1], allowed},
		{"similar-prefix", wildcard, allowed},
		{"unannotated", append(slices.Clone(valid), target(allowed[0], "unannotated-target")), allowed},
		{"unhealthy-second", unhealthy, allowed},
		{"stale-second", stale, allowed},
		{"missing-exported-identity", missingIdentity, allowed},
		{"missing-exported-identity-disabled", missingIdentity[1:], nil},
		{"retained-after-disabled", valid, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := utils.ValidateAnnotationTargets(test.targets, test.allowed, now); err == nil {
				t.Fatal("wrong fixture target set must fail")
			}
		})
	}
	if err := utils.ValidateAnnotationTargets(nil, nil, now); err != nil {
		t.Fatal(err)
	}
}
