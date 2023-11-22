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

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	promv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/prometheus-operator/prometheus-operator/pkg/assets"
	monitoringclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/prometheus-operator/prometheus-operator/pkg/informers"
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

	allocatorconfig "github.com/open-telemetry/opentelemetry-operator/cmd/otel-allocator/config"
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
		return nil, err
	}
	store := assets.NewStore(clientset.CoreV1(), clientset.CoreV1())
	promRegisterer := prometheusgoclient.NewRegistry()
	//promRegisterer = prometheusgoclient.WrapRegistererWith(prometheusgoclient.Labels{"controller": "targetallocator-prometheus"}, promRegisterer)
	operatorMetrics := operator.NewMetrics(promRegisterer)
	newNamespaceInformer := func(allowList map[string]struct{}) cache.SharedIndexInformer {
		// nsResyncPeriod is used to control how often the namespace informer
		// should resync. If the unprivileged ListerWatcher is used, then the
		// informer must resync more often because it cannot watch for
		// namespace changes.
		nsResyncPeriod := 15 * time.Second
		// If the only namespace is v1.NamespaceAll, then the client must be
		// privileged and a regular cache.ListWatch will be used. In this case
		// watching works and we do not need to resync so frequently.
		if listwatch.IsAllNamespaces(allowList) {
			nsResyncPeriod = resyncPeriod
		}
		nsInf := cache.NewSharedIndexInformer(
			operatorMetrics.NewInstrumentedListerWatcher(
				listwatch.NewUnprivilegedNamespaceListWatchFromClient(ctx, promOperatorLogger, clientset.CoreV1().RESTClient(), allowList, map[string]struct{}{}, fields.Everything()),
			),
			&v1.Namespace{}, nsResyncPeriod, cache.Indexers{},
		)

		return nsInf
	}
	nsMonInf := newNamespaceInformer(map[string]struct{}{v1.NamespaceAll: {}})

	resourceSelector := prometheus.NewResourceSelector(promOperatorLogger, prom, store, nsMonInf, operatorMetrics)

	// servMonSelector := getSelector(cfg.ServiceMonitorSelector)

	// podMonSelector := getSelector(cfg.PodMonitorSelector)

	return &PrometheusCRWatcher{
		logger:               logger,
		kubeMonitoringClient: mClient,
		k8sClient:            clientset,
		informers:            monitoringInformers,
		stopChannel:          make(chan struct{}),
		configGenerator:      generator,
		kubeConfigPath:       cliConfig.KubeConfigFilePath,
		// serviceMonitorSelector: servMonSelector,
		// podMonitorSelector:     podMonSelector,
		resourceSelector: resourceSelector,
		store:            store,
	}, nil
}

type PrometheusCRWatcher struct {
	logger               logr.Logger
	kubeMonitoringClient monitoringclient.Interface
	k8sClient            kubernetes.Interface
	informers            map[string]*informers.ForResource
	stopChannel          chan struct{}
	configGenerator      *prometheus.ConfigGenerator
	kubeConfigPath       string

	// serviceMonitorSelector labels.Selector
	// podMonitorSelector     labels.Selector
	resourceSelector *prometheus.ResourceSelector
	store            *assets.Store
}

// func getSelector(s map[string]string) labels.Selector {
// 	if s == nil {
// 		return labels.NewSelector()
// 	}
// 	return labels.SelectorFromSet(s)
// }

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
		return fmt.Errorf("failed to sync cache")
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
		w.logger.Error(err, "Failed getting SvcMonitor, skipping")
		return nil, err
	} else {
		w.logger.Info("Got svc monitors")
	}

	podMonitorInstances, err := w.resourceSelector.SelectPodMonitors(ctx, w.informers[monitoringv1.PodMonitorName].ListAllByNamespace)
	if err != nil {
		w.logger.Error(err, "Failed getting PodMonitor, skipping")
		return nil, err
	} else {
		w.logger.Info("Got pod monitors")
	}

	//store := assets.NewStore(w.k8sClient.CoreV1(), w.k8sClient.CoreV1())
	// //serviceMonitorInstances := make(map[string]*monitoringv1.ServiceMonitor)
	// smRetrieveErr := w.informers[monitoringv1.ServiceMonitorName].ListAll(w.serviceMonitorSelector, func(sm interface{}) {
	// 	monitor := sm.(*monitoringv1.ServiceMonitor)
	// 	key, _ := cache.DeletionHandlingMetaNamespaceKeyFunc(monitor)
	// 	validateError := w.addStoreAssetsForServiceMonitor(ctx, monitor.Name, monitor.Namespace, monitor.Spec.Endpoints, store)
	// 	if validateError != nil {
	// 		w.logger.Error(validateError, "Failed validating ServiceMonitor, skipping", "ServiceMonitor:", monitor.Name, "in namespace", monitor.Namespace)
	// 	} else {
	// 		serviceMonitorInstances[key] = monitor
	// 	}
	// })
	// if smRetrieveErr != nil {
	// 	return nil, smRetrieveErr
	// }

	//podMonitorInstances := make(map[string]*monitoringv1.PodMonitor)
	// pmRetrieveErr := w.informers[monitoringv1.PodMonitorName].ListAll(w.podMonitorSelector, func(pm interface{}) {
	// 	monitor := pm.(*monitoringv1.PodMonitor)
	// 	key, _ := cache.DeletionHandlingMetaNamespaceKeyFunc(monitor)
	// 	validateError := w.addStoreAssetsForPodMonitor(ctx, monitor.Name, monitor.Namespace, monitor.Spec.PodMetricsEndpoints, store)
	// 	if validateError != nil {
	// 		w.logger.Error(validateError, "Failed validating PodMonitor, skipping", "PodMonitor:", monitor.Name, "in namespace", monitor.Namespace)
	// 	} else {
	// 		podMonitorInstances[key] = monitor
	// 	}
	// })
	// if pmRetrieveErr != nil {
	// 	return nil, pmRetrieveErr
	// }

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

// addStoreAssetsForServiceMonitor adds authentication / authorization related information to the assets store,
// based on the service monitor and endpoints specs.
// This code borrows from
// https://github.com/prometheus-operator/prometheus-operator/blob/06b5c4189f3f72737766d86103d049115c3aff48/pkg/prometheus/resource_selector.go#L73.
// func (w *PrometheusCRWatcher) addStoreAssetsForServiceMonitor(
// 	ctx context.Context,
// 	smName, smNamespace string,
// 	endps []monitoringv1.Endpoint,
// 	store *assets.Store,
// ) error {
// 	var err error
// 	var validateErr error
// 	for i, endp := range endps {
// 		objKey := fmt.Sprintf("serviceMonitor/%s/%s/%d", smNamespace, smName, i)

// 		if err = store.AddBearerToken(ctx, smNamespace, endp.BearerTokenSecret, objKey); err != nil {
// 			break
// 		}

// 		if err = store.AddBasicAuth(ctx, smNamespace, endp.BasicAuth, objKey); err != nil {
// 			break
// 		}

// 		if endp.TLSConfig != nil {
// 			if err = store.AddTLSConfig(ctx, smNamespace, endp.TLSConfig); err != nil {
// 				break
// 			}
// 		}

// 		if err = store.AddOAuth2(ctx, smNamespace, endp.OAuth2, objKey); err != nil {
// 			break
// 		}

// 		smAuthKey := fmt.Sprintf("serviceMonitor/auth/%s/%s/%d", smNamespace, smName, i)
// 		if err = store.AddSafeAuthorizationCredentials(ctx, smNamespace, endp.Authorization, smAuthKey); err != nil {
// 			break
// 		}

// 		for _, rl := range endp.RelabelConfigs {
// 			if rl.Action != "" {
// 				if validateErr = validateRelabelConfig(*rl); validateErr != nil {
// 					break
// 				}
// 			}
// 		}

// 		for _, rl := range endp.MetricRelabelConfigs {
// 			if rl.Action != "" {
// 				if validateErr = validateRelabelConfig(*rl); validateErr != nil {
// 					break
// 				}
// 			}
// 		}
// 	}

// 	if err != nil {
// 		w.logger.Error(err, "Failed to obtain credentials for a ServiceMonitor", "serviceMonitor", smName)
// 	}

// 	if validateErr != nil {
// 		return validateErr
// 	}

// 	return nil
// }

// // addStoreAssetsForServiceMonitor adds authentication / authorization related information to the assets store,
// // based on the service monitor and pod metrics endpoints specs.
// // This code borrows from
// // https://github.com/prometheus-operator/prometheus-operator/blob/06b5c4189f3f72737766d86103d049115c3aff48/pkg/prometheus/resource_selector.go#L314.
// func (w *PrometheusCRWatcher) addStoreAssetsForPodMonitor(
// 	ctx context.Context,
// 	pmName, pmNamespace string,
// 	podMetricsEndps []monitoringv1.PodMetricsEndpoint,
// 	store *assets.Store,
// ) error {
// 	var err error
// 	var validateErr error
// 	for i, endp := range podMetricsEndps {
// 		objKey := fmt.Sprintf("podMonitor/%s/%s/%d", pmNamespace, pmName, i)

// 		if err = store.AddBearerToken(ctx, pmNamespace, endp.BearerTokenSecret, objKey); err != nil {
// 			break
// 		}

// 		if err = store.AddBasicAuth(ctx, pmNamespace, endp.BasicAuth, objKey); err != nil {
// 			break
// 		}

// 		if endp.TLSConfig != nil {
// 			if err = store.AddSafeTLSConfig(ctx, pmNamespace, &endp.TLSConfig.SafeTLSConfig); err != nil {
// 				break
// 			}
// 		}

// 		if err = store.AddOAuth2(ctx, pmNamespace, endp.OAuth2, objKey); err != nil {
// 			break
// 		}

// 		smAuthKey := fmt.Sprintf("podMonitor/auth/%s/%s/%d", pmNamespace, pmName, i)
// 		if err = store.AddSafeAuthorizationCredentials(ctx, pmNamespace, endp.Authorization, smAuthKey); err != nil {
// 			break
// 		}

// 		for _, rl := range endp.RelabelConfigs {
// 			if rl.Action != "" {
// 				if validateErr = validateRelabelConfig(*rl); validateErr != nil {
// 					break
// 				}
// 			}
// 		}

// 		for _, rl := range endp.MetricRelabelConfigs {
// 			if rl.Action != "" {
// 				if validateErr = validateRelabelConfig(*rl); validateErr != nil {
// 					break
// 				}
// 			}
// 		}
// 	}

// 	if err != nil {
// 		w.logger.Error(err, "Failed to obtain credentials for a PodMonitor", "podMonitor", pmName)
// 	}

// 	if validateErr != nil {
// 		return validateErr
// 	}

// 	return nil
// }

// func validateRelabelConfig(rc monitoringv1.RelabelConfig) error {
// 	relabelTarget := regexp.MustCompile(`^(?:(?:[a-zA-Z_]|\$(?:\{\w+\}|\w+))+\w*)+$`)
// 	// promVersion := operator.StringValOrDefault(p.GetCommonPrometheusFields().Version, operator.DefaultPrometheusVersion)
// 	// version, err := semver.ParseTolerant(promVersion)
// 	// if err != nil {
// 	// 	return errors.Wrap(err, "failed to parse Prometheus version")
// 	// }
// 	// minimumVersionCaseActions := version.GTE(semver.MustParse("2.36.0"))
// 	// minimumVersionEqualActions := version.GTE(semver.MustParse("2.41.0"))

// 	// if (rc.Action == string(relabel.Lowercase) || rc.Action == string(relabel.Uppercase)) && !minimumVersionCaseActions {
// 	// 	return errors.Errorf("%s relabel action is only supported from Prometheus version 2.36.0", rc.Action)
// 	// }

// 	// if (rc.Action == string(relabel.KeepEqual) || rc.Action == string(relabel.DropEqual)) && !minimumVersionEqualActions {
// 	// 	return errors.Errorf("%s relabel action is only supported from Prometheus version 2.41.0", rc.Action)
// 	// }

// 	if _, err := relabel.NewRegexp(rc.Regex); err != nil {
// 		return fmt.Errorf("invalid regex %s for relabel configuration", rc.Regex)
// 	}

// 	if rc.Modulus == 0 && rc.Action == string(relabel.HashMod) {
// 		return fmt.Errorf("relabel configuration for hashmod requires non-zero modulus")
// 	}

// 	if (rc.Action == string(relabel.Replace) || rc.Action == string(relabel.HashMod) || rc.Action == string(relabel.Lowercase) || rc.Action == string(relabel.Uppercase) || rc.Action == string(relabel.KeepEqual) || rc.Action == string(relabel.DropEqual)) && rc.TargetLabel == "" {
// 		return fmt.Errorf("relabel configuration for %s action needs targetLabel value", rc.Action)
// 	}

// 	if (rc.Action == string(relabel.Replace) || rc.Action == string(relabel.Lowercase) || rc.Action == string(relabel.Uppercase) || rc.Action == string(relabel.KeepEqual) || rc.Action == string(relabel.DropEqual)) && !relabelTarget.MatchString(rc.TargetLabel) {
// 		return fmt.Errorf("%q is invalid 'target_label' for %s action", rc.TargetLabel, rc.Action)
// 	}

// 	if (rc.Action == string(relabel.Lowercase) || rc.Action == string(relabel.Uppercase) || rc.Action == string(relabel.KeepEqual) || rc.Action == string(relabel.DropEqual)) && !(rc.Replacement == relabel.DefaultRelabelConfig.Replacement || rc.Replacement == "") {
// 		return fmt.Errorf("'replacement' can not be set for %s action", rc.Action)
// 	}

// 	if rc.Action == string(relabel.LabelMap) {
// 		if rc.Replacement != "" && !relabelTarget.MatchString(rc.Replacement) {
// 			return fmt.Errorf("%q is invalid 'replacement' for %s action", rc.Replacement, rc.Action)
// 		}
// 	}

// 	if rc.Action == string(relabel.HashMod) && !model.LabelName(rc.TargetLabel).IsValid() {
// 		return fmt.Errorf("%q is invalid 'target_label' for %s action", rc.TargetLabel, rc.Action)
// 	}

// 	if rc.Action == string(relabel.KeepEqual) || rc.Action == string(relabel.DropEqual) {
// 		if !(rc.Regex == "" || rc.Regex == relabel.DefaultRelabelConfig.Regex.String()) ||
// 			!(rc.Modulus == uint64(0) ||
// 				rc.Modulus == relabel.DefaultRelabelConfig.Modulus) ||
// 			!(rc.Separator == "" ||
// 				rc.Separator == relabel.DefaultRelabelConfig.Separator) ||
// 			!(rc.Replacement == relabel.DefaultRelabelConfig.Replacement ||
// 				rc.Replacement == "") {
// 			return fmt.Errorf("%s action requires only 'source_labels' and `target_label`, and no other fields", rc.Action)
// 		}
// 	}

// 	if rc.Action == string(relabel.LabelDrop) || rc.Action == string(relabel.LabelKeep) {
// 		if len(rc.SourceLabels) != 0 ||
// 			!(rc.TargetLabel == "" ||
// 				rc.TargetLabel == relabel.DefaultRelabelConfig.TargetLabel) ||
// 			!(rc.Modulus == uint64(0) ||
// 				rc.Modulus == relabel.DefaultRelabelConfig.Modulus) ||
// 			!(rc.Separator == "" ||
// 				rc.Separator == relabel.DefaultRelabelConfig.Separator) ||
// 			!(rc.Replacement == relabel.DefaultRelabelConfig.Replacement ||
// 				rc.Replacement == "") {
// 			return fmt.Errorf("%s action requires only 'regex', and no other fields", rc.Action)
// 		}
// 	}
// 	return nil
// }
