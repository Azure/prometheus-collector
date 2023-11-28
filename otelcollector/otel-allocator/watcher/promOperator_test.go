// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package watcher

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	allocatorconfig "github.com/open-telemetry/opentelemetry-operator/cmd/otel-allocator/config"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus-operator/prometheus-operator/pkg/assets"
	fakemonitoringclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/prometheus-operator/prometheus-operator/pkg/informers"
	"github.com/prometheus-operator/prometheus-operator/pkg/operator"
	"github.com/prometheus-operator/prometheus-operator/pkg/prometheus"
	prometheusgoclient "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	promconfig "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	kubeDiscovery "github.com/prometheus/prometheus/discovery/kubernetes"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name           string
		serviceMonitor *monitoringv1.ServiceMonitor
		podMonitor     *monitoringv1.PodMonitor
		want           *promconfig.Config
		wantErr        bool
	}{
		{
			name: "simple test",
			serviceMonitor: &monitoringv1.ServiceMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "simple",
					Namespace: "test",
				},
				Spec: monitoringv1.ServiceMonitorSpec{
					JobLabel: "test",
					Endpoints: []monitoringv1.Endpoint{
						{
							Port: "web",
						},
					},
				},
			},
			podMonitor: &monitoringv1.PodMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "simple",
					Namespace: "test",
				},
				Spec: monitoringv1.PodMonitorSpec{
					JobLabel: "test",
					PodMetricsEndpoints: []monitoringv1.PodMetricsEndpoint{
						{
							Port: "web",
						},
					},
				},
			},
			want: &promconfig.Config{
				ScrapeConfigs: []*promconfig.ScrapeConfig{
					{
						JobName:         "serviceMonitor/test/simple/0",
						ScrapeInterval:  model.Duration(30 * time.Second),
						ScrapeTimeout:   model.Duration(10 * time.Second),
						HonorTimestamps: true,
						HonorLabels:     false,
						Scheme:          "http",
						MetricsPath:     "/metrics",
						ServiceDiscoveryConfigs: []discovery.Config{
							&kubeDiscovery.SDConfig{
								Role: "endpointslice",
								NamespaceDiscovery: kubeDiscovery.NamespaceDiscovery{
									Names:               []string{"test"},
									IncludeOwnNamespace: false,
								},
								HTTPClientConfig: config.DefaultHTTPClientConfig,
							},
						},
						HTTPClientConfig: config.DefaultHTTPClientConfig,
					},
					{
						JobName:         "podMonitor/test/simple/0",
						ScrapeInterval:  model.Duration(30 * time.Second),
						ScrapeTimeout:   model.Duration(10 * time.Second),
						HonorTimestamps: true,
						HonorLabels:     false,
						Scheme:          "http",
						MetricsPath:     "/metrics",
						ServiceDiscoveryConfigs: []discovery.Config{
							&kubeDiscovery.SDConfig{
								Role: "pod",
								NamespaceDiscovery: kubeDiscovery.NamespaceDiscovery{
									Names:               []string{"test"},
									IncludeOwnNamespace: false,
								},
								HTTPClientConfig: config.DefaultHTTPClientConfig,
							},
						},
						HTTPClientConfig: config.DefaultHTTPClientConfig,
					},
				},
			},
		},
		{
			name: "basic auth (serviceMonitor)",
			serviceMonitor: &monitoringv1.ServiceMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "auth",
					Namespace: "test",
				},
				Spec: monitoringv1.ServiceMonitorSpec{
					JobLabel: "auth",
					Endpoints: []monitoringv1.Endpoint{
						{
							Port: "web",
							BasicAuth: &monitoringv1.BasicAuth{
								Username: v1.SecretKeySelector{
									LocalObjectReference: v1.LocalObjectReference{
										Name: "basic-auth",
									},
									Key: "username",
								},
								Password: v1.SecretKeySelector{
									LocalObjectReference: v1.LocalObjectReference{
										Name: "basic-auth",
									},
									Key: "password",
								},
							},
						},
					},
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "auth",
						},
					},
				},
			},
			want: &promconfig.Config{
				GlobalConfig: promconfig.GlobalConfig{},
				ScrapeConfigs: []*promconfig.ScrapeConfig{
					{
						JobName:         "serviceMonitor/test/auth/0",
						ScrapeInterval:  model.Duration(30 * time.Second),
						ScrapeTimeout:   model.Duration(10 * time.Second),
						HonorTimestamps: true,
						HonorLabels:     false,
						Scheme:          "http",
						MetricsPath:     "/metrics",
						ServiceDiscoveryConfigs: []discovery.Config{
							&kubeDiscovery.SDConfig{
								Role: "endpointslice",
								NamespaceDiscovery: kubeDiscovery.NamespaceDiscovery{
									Names:               []string{"test"},
									IncludeOwnNamespace: false,
								},
								HTTPClientConfig: config.DefaultHTTPClientConfig,
							},
						},
						HTTPClientConfig: config.HTTPClientConfig{
							FollowRedirects: true,
							EnableHTTP2:     true,
							BasicAuth: &config.BasicAuth{
								Username: "admin",
								Password: "password",
							},
						},
					},
				},
			},
		},
		{
			name: "bearer token (podMonitor)",
			podMonitor: &monitoringv1.PodMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bearer",
					Namespace: "test",
				},
				Spec: monitoringv1.PodMonitorSpec{
					JobLabel: "bearer",
					PodMetricsEndpoints: []monitoringv1.PodMetricsEndpoint{
						{
							Port: "web",
							BearerTokenSecret: v1.SecretKeySelector{
								LocalObjectReference: v1.LocalObjectReference{
									Name: "bearer",
								},
								Key: "token",
							},
						},
					},
				},
			},
			want: &promconfig.Config{
				GlobalConfig: promconfig.GlobalConfig{},
				ScrapeConfigs: []*promconfig.ScrapeConfig{
					{
						JobName:         "podMonitor/test/bearer/0",
						ScrapeInterval:  model.Duration(30 * time.Second),
						ScrapeTimeout:   model.Duration(10 * time.Second),
						HonorTimestamps: true,
						HonorLabels:     false,
						Scheme:          "http",
						MetricsPath:     "/metrics",
						ServiceDiscoveryConfigs: []discovery.Config{
							&kubeDiscovery.SDConfig{
								Role: "pod",
								NamespaceDiscovery: kubeDiscovery.NamespaceDiscovery{
									Names:               []string{"test"},
									IncludeOwnNamespace: false,
								},
								HTTPClientConfig: config.DefaultHTTPClientConfig,
							},
						},
						HTTPClientConfig: config.HTTPClientConfig{
							FollowRedirects: true,
							EnableHTTP2:     true,
							Authorization: &config.Authorization{
								Type:        "Bearer",
								Credentials: "bearer-token",
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := getTestPrometheuCRWatcher(t, tt.serviceMonitor, tt.podMonitor)

			// Start namespace informers in order to populate cache.
			w.nsInformer.Run(w.stopChannel)
			// for !w.nsInformer.HasSynced() {
			// 	time.Sleep(50 * time.Millisecond)
			// }

			for _, informer := range w.informers {
				// Start informers in order to populate cache.
				informer.Start(w.stopChannel)
			}

			// Wait for informers to sync.
			for _, informer := range w.informers {
				for !informer.HasSynced() {
					time.Sleep(50 * time.Millisecond)
				}
			}

			got, err := w.LoadConfig(context.Background())
			assert.NoError(t, err)

			sanitizeScrapeConfigsForTest(got.ScrapeConfigs)
			assert.Equal(t, tt.want.ScrapeConfigs, got.ScrapeConfigs)
		})
	}
}

// getTestPrometheuCRWatcher creates a test instance of PrometheusCRWatcher with fake clients
// and test secrets.
func getTestPrometheuCRWatcher(t *testing.T, sm *monitoringv1.ServiceMonitor, pm *monitoringv1.PodMonitor) *PrometheusCRWatcher {
	mClient := fakemonitoringclient.NewSimpleClientset()
	if sm != nil {
		_, err := mClient.MonitoringV1().ServiceMonitors("test").Create(context.Background(), sm, metav1.CreateOptions{})
		if err != nil {
			t.Fatal(t, err)
		}
	}
	if pm != nil {
		_, err := mClient.MonitoringV1().PodMonitors("test").Create(context.Background(), pm, metav1.CreateOptions{})
		if err != nil {
			t.Fatal(t, err)
		}
	}

	k8sClient := fake.NewSimpleClientset()
	_, err := k8sClient.CoreV1().Secrets("test").Create(context.Background(), &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "basic-auth",
			Namespace: "test",
		},
		Data: map[string][]byte{"username": []byte("admin"), "password": []byte("password")},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(t, err)
	}
	_, err = k8sClient.CoreV1().Secrets("test").Create(context.Background(), &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bearer",
			Namespace: "test",
		},
		Data: map[string][]byte{"token": []byte("bearer-token")},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(t, err)
	}

	_, err = k8sClient.CoreV1().Namespaces().Create(context.Background(), &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace",
		},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(t, err)
	}

	factory := informers.NewMonitoringInformerFactories(map[string]struct{}{v1.NamespaceAll: {}}, map[string]struct{}{}, mClient, 0, nil)
	informers, err := getInformers(factory)
	if err != nil {
		t.Fatal(t, err)
	}

	cfg := allocatorconfig.Config{}
	// 	// AllocationStrategy: &allocationStrategy,
	// 	LabelSelector: map[string]string{
	// 		"rsName":                         "ama-metrics",
	// 		"kubernetes.azure.com/managedby": "aks",
	// 	},
	// }

	prom := &monitoringv1.Prometheus{
		Spec: monitoringv1.PrometheusSpec{
			CommonPrometheusFields: monitoringv1.CommonPrometheusFields{
				ScrapeInterval: monitoringv1.Duration("30s"),
				ServiceMonitorSelector: &metav1.LabelSelector{
					MatchLabels: cfg.ServiceMonitorSelector,
				},
				PodMonitorSelector: &metav1.LabelSelector{
					MatchLabels: cfg.PodMonitorSelector,
				},
				ServiceMonitorNamespaceSelector: &metav1.LabelSelector{
					MatchLabels: cfg.ServiceMonitorNamespaceSelector,
				},
				PodMonitorNamespaceSelector: &metav1.LabelSelector{
					MatchLabels: cfg.PodMonitorNamespaceSelector,
				},
			},
		},
	}

	promOperatorLogger := level.NewFilter(log.NewLogfmtLogger(os.Stderr), level.AllowDebug())

	generator, err := prometheus.NewConfigGenerator(promOperatorLogger, prom, true)
	if err != nil {
		t.Fatal(t, err)
	}

	// c := fake.NewSimpleClientset(
	// 	&v1.Secret{
	// 		ObjectMeta: metav1.ObjectMeta{
	// 			Name:      "secret",
	// 			Namespace: "ns1",
	// 		},
	// 		Data: map[string][]byte{
	// 			"key1": []byte("val1"),
	// 		},
	// 	},
	// )

	store := assets.NewStore(k8sClient.CoreV1(), k8sClient.CoreV1())
	promRegisterer := prometheusgoclient.NewRegistry()
	// promRegisterer = prometheusgoclient.WrapRegistererWith(prometheusgoclient.Labels{"controller": "targetallocator-prometheus"}, promRegisterer)
	operatorMetrics := operator.NewMetrics(promRegisterer)

	nsMonInf := getNamespaceInformer(context.Background(), map[string]struct{}{v1.NamespaceAll: {}}, promOperatorLogger, k8sClient, operatorMetrics)

	resourceSelector := prometheus.NewResourceSelector(promOperatorLogger, prom, store, nsMonInf, operatorMetrics)

	// servMonSelector := getSelector(cfg.ServiceMonitorSelector)

	// podMonSelector := getSelector(cfg.PodMonitorSelector)

	return &PrometheusCRWatcher{
		kubeMonitoringClient: mClient,
		k8sClient:            k8sClient,
		informers:            informers,
		nsInformer:           nsMonInf,
		stopChannel:          make(chan struct{}),
		configGenerator:      generator,
		// serviceMonitorSelector: servMonSelector,
		// podMonitorSelector:     podMonSelector,
		resourceSelector: resourceSelector,
		// store:            store,
	}

	// generator, err := prometheus.NewConfigGenerator(log.NewNopLogger(), prom, true)
	// if err != nil {
	// 	t.Fatal(t, err)
	// }

	// return &PrometheusCRWatcher{
	// 	kubeMonitoringClient: mClient,
	// 	k8sClient:            k8sClient,
	// 	informers:            informers,
	// 	configGenerator:      generator,
	// 	// serviceMonitorSelector: getSelector(nil),
	// 	// podMonitorSelector:     getSelector(nil),
	// 	stopChannel: make(chan struct{}),
	// }
}

// Remove relable configs fields from scrape configs for testing,
// since these are mutated and tested down the line with the hook(s).
func sanitizeScrapeConfigsForTest(scs []*promconfig.ScrapeConfig) {
	for _, sc := range scs {
		sc.RelabelConfigs = nil
		sc.MetricRelabelConfigs = nil
	}
}
