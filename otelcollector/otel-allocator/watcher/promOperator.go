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
	"fmt"
	"os"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/go-logr/logr"

	allocatorconfig "github.com/open-telemetry/opentelemetry-operator/cmd/otel-allocator/config"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	promv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/prometheus-operator/prometheus-operator/pkg/assets"
	monitoringclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/prometheus-operator/prometheus-operator/pkg/informers"

	"github.com/prometheus-operator/prometheus-operator/pkg/k8sutil"
	"github.com/prometheus-operator/prometheus-operator/pkg/listwatch"
	"github.com/prometheus-operator/prometheus-operator/pkg/operator"
	"github.com/prometheus-operator/prometheus-operator/pkg/prometheus"
	prometheusgoclient "github.com/prometheus/client_golang/prometheus"
	promconfig "github.com/prometheus/prometheus/config"
	kubeDiscovery "github.com/prometheus/prometheus/discovery/kubernetes"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const (
	resyncPeriod = 5 * time.Minute
)

func NewPrometheusCRWatcher(ctx context.Context, logger logr.Logger, cfg allocatorconfig.Config, cliConfig allocatorconfig.CLIConfig) (*PrometheusCRWatcher, error) {
	mClient, err := monitoringclient.NewForConfig(cliConfig.ClusterConfig)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(cliConfig.ClusterConfig)
	if err != nil {
		return nil, err
	}

	factory := informers.NewMonitoringInformerFactories(map[string]struct{}{v1.NamespaceAll: {}}, map[string]struct{}{}, mClient, allocatorconfig.DefaultResyncTime, nil) //TODO decide what strategy to use regarding namespaces

	monitoringInformers, err := getInformers(factory)
	if err != nil {
		return nil, err
	}

	// TODO: We should make these durations configurable
	prom := &monitoringv1.Prometheus{
		Spec: monitoringv1.PrometheusSpec{
			CommonPrometheusFields: monitoringv1.CommonPrometheusFields{
				ScrapeInterval: monitoringv1.Duration(cfg.PrometheusCR.ScrapeInterval.String()),
				ServiceMonitorSelector: &metav1.LabelSelector{
					MatchLabels: cfg.ServiceMonitorSelector,
				},
				PodMonitorSelector: &metav1.LabelSelector{
					MatchLabels: cfg.PodMonitorSelector,
					// MatchLabels: map[string]string{
					// 	"testpod": "testpod",
					// },
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
	promOperatorLogger := level.NewFilter(log.NewLogfmtLogger(os.Stderr), level.AllowWarn())

	generator, err := prometheus.NewConfigGenerator(promOperatorLogger, prom, true)
	if err != nil {
		return nil, err
	}
	store := assets.NewStore(clientset.CoreV1(), clientset.CoreV1())
	promRegisterer := prometheusgoclient.NewRegistry()
	operatorMetrics := operator.NewMetrics(promRegisterer)

	nsMonInf := getNamespaceInformer(ctx, map[string]struct{}{v1.NamespaceAll: {}}, promOperatorLogger, clientset, operatorMetrics)

	resourceSelector := prometheus.NewResourceSelector(promOperatorLogger, prom, store, nsMonInf, operatorMetrics)

	return &PrometheusCRWatcher{
		logger:               logger,
		kubeMonitoringClient: mClient,
		k8sClient:            clientset,
		informers:            monitoringInformers,
		nsInformer:           nsMonInf,
		stopChannel:          make(chan struct{}),
		configGenerator:      generator,
		kubeConfigPath:       cliConfig.KubeConfigFilePath,
		podMonitorNamespaceSelector: &metav1.LabelSelector{
			MatchLabels: cfg.PodMonitorNamespaceSelector,
		},
		serviceMonitorNamespaceSelector: &metav1.LabelSelector{
			MatchLabels: cfg.ServiceMonitorNamespaceSelector,
		},
		resourceSelector: resourceSelector,
		store:            store,
	}, nil
}

type PrometheusCRWatcher struct {
	logger                          logr.Logger
	kubeMonitoringClient            monitoringclient.Interface
	k8sClient                       kubernetes.Interface
	informers                       map[string]*informers.ForResource
	nsInformer                      cache.SharedIndexInformer
	stopChannel                     chan struct{}
	configGenerator                 *prometheus.ConfigGenerator
	kubeConfigPath                  string
	podMonitorNamespaceSelector     *metav1.LabelSelector
	serviceMonitorNamespaceSelector *metav1.LabelSelector
	resourceSelector                *prometheus.ResourceSelector
	store                           *assets.Store
}

func getNamespaceInformer(ctx context.Context, allowList map[string]struct{}, promOperatorLogger log.Logger, clientset kubernetes.Interface, operatorMetrics *operator.Metrics) cache.SharedIndexInformer {
	nsInf := cache.NewSharedIndexInformer(
		operatorMetrics.NewInstrumentedListerWatcher(
			listwatch.NewUnprivilegedNamespaceListWatchFromClient(ctx, promOperatorLogger, clientset.CoreV1().RESTClient(), allowList, map[string]struct{}{}, fields.Everything()),
		),
		&v1.Namespace{}, resyncPeriod, cache.Indexers{},
	)

	return nsInf
}

// getInformers returns a map of informers for the given resources.
func getInformers(factory informers.FactoriesForNamespaces) (map[string]*informers.ForResource, error) {
	serviceMonitorInformers, err := informers.NewInformersForResource(factory, monitoringv1.SchemeGroupVersion.WithResource(monitoringv1.ServiceMonitorName))
	if err != nil {
		return nil, err
	}

	podMonitorInformers, err := informers.NewInformersForResource(factory, monitoringv1.SchemeGroupVersion.WithResource(monitoringv1.PodMonitorName))
	if err != nil {
		return nil, err
	}

	return map[string]*informers.ForResource{
		monitoringv1.ServiceMonitorName: serviceMonitorInformers,
		monitoringv1.PodMonitorName:     podMonitorInformers,
	}, nil
}

// Watch wrapped informers and wait for an initial sync.
func (w *PrometheusCRWatcher) Watch(upstreamEvents chan Event, upstreamErrors chan error) error {
	event := Event{
		Source:  EventSourcePrometheusCR,
		Watcher: Watcher(w),
	}
	success := true

	go w.nsInformer.Run(w.stopChannel)
	if ok := cache.WaitForNamedCacheSync("namespace", w.stopChannel, w.nsInformer.HasSynced); !ok {
		success = false
	}
	w.nsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, newObj interface{}) {
			old := oldObj.(*v1.Namespace)
			cur := newObj.(*v1.Namespace)

			// Periodic resync may resend the Namespace without changes
			// in-between.
			if old.ResourceVersion == cur.ResourceVersion {
				return
			}

			for name, selector := range map[string]*metav1.LabelSelector{
				"PodMonitorNamespaceSelector":     w.podMonitorNamespaceSelector,
				"ServiceMonitorNamespaceSelector": w.serviceMonitorNamespaceSelector,
			} {

				sync, err := k8sutil.LabelSelectionHasChanged(old.Labels, cur.Labels, selector)
				if err != nil {
					w.logger.Error(err, "Failed to check label selection between namespaces while handling namespace updates", "selector", name)
					return
				}

				if sync {
					upstreamEvents <- event
					return
				}
			}
		},
	})

	for name, resource := range w.informers {
		resource.Start(w.stopChannel)

		if ok := cache.WaitForNamedCacheSync(name, w.stopChannel, resource.HasSynced); !ok {
			success = false
		}
		resource.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				upstreamEvents <- event
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				upstreamEvents <- event
			},
			DeleteFunc: func(obj interface{}) {
				upstreamEvents <- event
			},
		})
	}
	if !success {
		return fmt.Errorf("failed to sync one of the caches")
	}
	<-w.stopChannel
	return nil
}

func (w *PrometheusCRWatcher) Close() error {
	close(w.stopChannel)
	return nil
}

func (w *PrometheusCRWatcher) LoadConfig(ctx context.Context) (*promconfig.Config, error) {
	serviceMonitorInstances, err := w.resourceSelector.SelectServiceMonitors(ctx, w.informers[monitoringv1.ServiceMonitorName].ListAllByNamespace)
	if err != nil {
		return nil, err
	}

	podMonitorInstances, err := w.resourceSelector.SelectPodMonitors(ctx, w.informers[monitoringv1.PodMonitorName].ListAllByNamespace)
	if err != nil {
		return nil, err
	}

	generatedConfig, err := w.configGenerator.GenerateServerConfiguration(
		ctx,
		"30s",
		"",
		nil,
		nil,
		monitoringv1.TSDBSpec{},
		nil,
		nil,
		serviceMonitorInstances,
		podMonitorInstances,
		map[string]*monitoringv1.Probe{},
		map[string]*promv1alpha1.ScrapeConfig{},
		w.store,
		nil,
		nil,
		nil,
		[]string{})
	if err != nil {
		return nil, err
	}

	promCfg := &promconfig.Config{}
	unmarshalErr := yaml.Unmarshal(generatedConfig, promCfg)
	if unmarshalErr != nil {
		return nil, unmarshalErr
	}

	// set kubeconfig path to service discovery configs, else kubernetes_sd will always attempt in-cluster
	// authentication even if running with a detected kubeconfig
	for _, scrapeConfig := range promCfg.ScrapeConfigs {
		for _, serviceDiscoveryConfig := range scrapeConfig.ServiceDiscoveryConfigs {
			if serviceDiscoveryConfig.Name() == "kubernetes" {
				sdConfig := interface{}(serviceDiscoveryConfig).(*kubeDiscovery.SDConfig)
				sdConfig.KubeConfig = w.kubeConfigPath
			}
		}
	}
	return promCfg, nil
}
