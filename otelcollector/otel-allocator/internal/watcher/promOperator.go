// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package watcher

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/blang/semver/v4"
	"github.com/go-logr/logr"
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
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"

	allocatorconfig "github.com/open-telemetry/opentelemetry-operator/cmd/otel-allocator/internal/config"
)

const (
	resyncPeriod     = 5 * time.Minute
	minEventInterval = time.Second * 5
)

var DefaultScrapeProtocols = []monitoringv1.ScrapeProtocol{
	monitoringv1.OpenMetricsText1_0_0,
	monitoringv1.OpenMetricsText0_0_1,
	monitoringv1.PrometheusText0_0_4,
}

func NewPrometheusCRWatcher(ctx context.Context, logger logr.Logger, cfg allocatorconfig.Config) (*PrometheusCRWatcher, error) {
	promLogger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	slogger := slog.New(logr.ToSlogHandler(logger))
	var resourceSelector *prometheus.ResourceSelector
	mClient, err := monitoringclient.NewForConfig(cfg.ClusterConfig)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(cfg.ClusterConfig)
	if err != nil {
		return nil, err
	}

	mdClient, err := metadata.NewForConfig(cfg.ClusterConfig)
	if err != nil {
		return nil, err
	}

	probeGroupVersion := monitoringv1.SchemeGroupVersion
	probeResource := monitoringv1.ProbeName
	probeCRDInstalled, err := k8sutil.IsAPIGroupVersionResourceSupported(clientset.Discovery(), probeGroupVersion, probeResource)
	if err != nil {
		return nil, err
	}

	if !probeCRDInstalled {
		fmt.Printf("resource %q (group: %q) not installed in the cluster\n", probeResource, probeGroupVersion)
	} else {
		fmt.Printf("resource %q (group: %q) is installed in the cluster\n", probeResource, probeGroupVersion)
	}

	scrapeConfigGroupVersion := promv1alpha1.SchemeGroupVersion
	scrapeConfigResource := promv1alpha1.ScrapeConfigName
	scrapeConfigCRDInstalled, err := k8sutil.IsAPIGroupVersionResourceSupported(clientset.Discovery(), scrapeConfigGroupVersion, scrapeConfigResource)
	if err != nil {
		return nil, err
	}
	if !scrapeConfigCRDInstalled {
		fmt.Printf("resource %q (group: %q) not installed in the cluster\n", scrapeConfigResource, scrapeConfigGroupVersion)
	} else {
		fmt.Printf("resource %q (group: %q) is installed in the cluster\n", scrapeConfigResource, scrapeConfigGroupVersion)
	}

	// use above instead like in prom operator- https://github.com/prometheus-operator/prometheus-operator/issues/7459
	// https://github.com/prometheus-operator/prometheus-operator/blob/c4ebc762d0d2263541c67ebfe1ba7f2b419ed547/cmd/operator/main.go#L74
	// probeCRDExists, err := crdExists(ctx, slogger, crdClientSet, "probes.monitoring.coreos.com")
	// if err != nil {
	// 	return nil, err
	// }

	// scrapeConfigCRDExists, err := crdExists(ctx, slogger, crdClientSet, "scrapeconfigs.monitoring.coreos.com")
	// if err != nil {
	// 	return nil, err
	// }

	allowList, denyList := cfg.PrometheusCR.GetAllowDenyLists()

	monitoringInformerFactory := informers.NewMonitoringInformerFactories(allowList, denyList, mClient, allocatorconfig.DefaultResyncTime, nil)
	metaDataInformerFactory := informers.NewMetadataInformerFactory(allowList, denyList, mdClient, allocatorconfig.DefaultResyncTime, nil)
	monitoringInformers, err := getInformers(monitoringInformerFactory, metaDataInformerFactory, probeCRDInstalled, scrapeConfigCRDInstalled)
	// monitoringInformers, err := getInformers(monitoringInformerFactory, metaDataInformerFactory)
	if err != nil {
		return nil, err
	}

	// we want to use endpointslices by default
	serviceDiscoveryRole := monitoringv1.ServiceDiscoveryRole("EndpointSlice")

	// TODO: We should make these durations configurable
	prom := &monitoringv1.Prometheus{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cfg.CollectorNamespace,
		},
		Spec: monitoringv1.PrometheusSpec{
			CommonPrometheusFields: monitoringv1.CommonPrometheusFields{
				ScrapeInterval:                  monitoringv1.Duration(cfg.PrometheusCR.ScrapeInterval.String()),
				PodMonitorSelector:              cfg.PrometheusCR.PodMonitorSelector,
				PodMonitorNamespaceSelector:     cfg.PrometheusCR.PodMonitorNamespaceSelector,
				ServiceMonitorSelector:          cfg.PrometheusCR.ServiceMonitorSelector,
				ServiceMonitorNamespaceSelector: cfg.PrometheusCR.ServiceMonitorNamespaceSelector,
				ScrapeConfigSelector:            cfg.PrometheusCR.ScrapeConfigSelector,
				ScrapeConfigNamespaceSelector:   cfg.PrometheusCR.ScrapeConfigNamespaceSelector,
				ProbeSelector:                   cfg.PrometheusCR.ProbeSelector,
				ProbeNamespaceSelector:          cfg.PrometheusCR.ProbeNamespaceSelector,
				ServiceDiscoveryRole:            &serviceDiscoveryRole,
				Version:                         "2.55.1", // fix Prometheus version 2 to avoid generating incompatible config
				ScrapeProtocols:                 DefaultScrapeProtocols,
			},
			EvaluationInterval: monitoringv1.Duration("30s"),
		},
	}

	generator, err := prometheus.NewConfigGenerator(promLogger, prom, prometheus.WithEndpointSliceSupport())

	if err != nil {
		return nil, err
	}

	store := assets.NewStoreBuilder(clientset.CoreV1(), clientset.CoreV1())
	promRegisterer := prometheusgoclient.NewRegistry()
	operatorMetrics := operator.NewMetrics(promRegisterer)
	eventRecorderFactory := operator.NewEventRecorderFactory(false)
	eventRecorder := eventRecorderFactory(clientset, "target-allocator")

	var nsMonInf cache.SharedIndexInformer
	getNamespaceInformerErr := retry.OnError(retry.DefaultRetry,
		func(err error) bool {
			logger.Error(err, "Retrying namespace informer creation in promOperator CRD watcher")
			return true
		}, func() error {
			nsMonInf, err = getNamespaceInformer(ctx, allowList, denyList, promLogger, clientset, operatorMetrics)
			return err
		})
	if getNamespaceInformerErr != nil {
		logger.Error(getNamespaceInformerErr, "Failed to create namespace informer in promOperator CRD watcher")
		return nil, getNamespaceInformerErr
	}

	resourceSelector, err = prometheus.NewResourceSelector(slogger, prom, store, nsMonInf, operatorMetrics, eventRecorder)
	if err != nil {
		logger.Error(err, "Failed to create resource selector in promOperator CRD watcher")
	}

	return &PrometheusCRWatcher{
		logger:                          slogger,
		kubeMonitoringClient:            mClient,
		k8sClient:                       clientset,
		informers:                       monitoringInformers,
		nsInformer:                      nsMonInf,
		stopChannel:                     make(chan struct{}),
		eventInterval:                   minEventInterval,
		configGenerator:                 generator,
		kubeConfigPath:                  cfg.KubeConfigFilePath,
		podMonitorNamespaceSelector:     cfg.PrometheusCR.PodMonitorNamespaceSelector,
		serviceMonitorNamespaceSelector: cfg.PrometheusCR.ServiceMonitorNamespaceSelector,
		scrapeConfigNamespaceSelector:   cfg.PrometheusCR.ScrapeConfigNamespaceSelector,
		probeNamespaceSelector:          cfg.PrometheusCR.ProbeNamespaceSelector,
		resourceSelector:                resourceSelector,
		store:                           store,
		prometheusCR:                    prom,
		probeCRDInstalled:               probeCRDInstalled,
		scrapeConfigCRDInstalled:        scrapeConfigCRDInstalled,
	}, nil
}

type PrometheusCRWatcher struct {
	logger                          *slog.Logger
	kubeMonitoringClient            monitoringclient.Interface
	k8sClient                       kubernetes.Interface
	informers                       map[string]*informers.ForResource
	nsInformer                      cache.SharedIndexInformer
	eventInterval                   time.Duration
	stopChannel                     chan struct{}
	configGenerator                 *prometheus.ConfigGenerator
	kubeConfigPath                  string
	podMonitorNamespaceSelector     *metav1.LabelSelector
	serviceMonitorNamespaceSelector *metav1.LabelSelector
	scrapeConfigNamespaceSelector   *metav1.LabelSelector
	probeNamespaceSelector          *metav1.LabelSelector
	resourceSelector                *prometheus.ResourceSelector
	store                           *assets.StoreBuilder
	prometheusCR                    *monitoringv1.Prometheus
	probeCRDInstalled               bool // indicates if the Probe CRD exists in the cluster
	scrapeConfigCRDInstalled        bool // indicates if the ScrapeConfig CRD exists in the cluster
}

func getNamespaceInformer(ctx context.Context, allowList, denyList map[string]struct{}, promOperatorLogger *slog.Logger, clientset kubernetes.Interface, operatorMetrics *operator.Metrics) (cache.SharedIndexInformer, error) {
	kubernetesVersion, err := clientset.Discovery().ServerVersion()
	if err != nil {
		return nil, err
	}
	kubernetesSemverVersion, err := semver.ParseTolerant(kubernetesVersion.String())
	if err != nil {
		return nil, err
	}
	lw, _, err := listwatch.NewNamespaceListWatchFromClient(
		ctx,
		promOperatorLogger,
		kubernetesSemverVersion,
		clientset.CoreV1(),
		clientset.AuthorizationV1().SelfSubjectAccessReviews(),
		allowList,
		denyList,
	)
	if err != nil {
		return nil, err
	}

	return cache.NewSharedIndexInformer(
		operatorMetrics.NewInstrumentedListerWatcher(lw),
		&v1.Namespace{}, resyncPeriod, cache.Indexers{},
	), nil

}

// getInformers returns a map of informers for the given resources.
func getInformers(factory informers.FactoriesForNamespaces, metaDataInformerFactory informers.FactoriesForNamespaces, probeCRDInstalled bool, scrapeConfigCRDInstalled bool) (map[string]*informers.ForResource, error) {
	// func getInformers(factory informers.FactoriesForNamespaces, metaDataInformerFactory informers.FactoriesForNamespaces) (map[string]*informers.ForResource, error) {
	serviceMonitorInformers, err := informers.NewInformersForResource(factory, monitoringv1.SchemeGroupVersion.WithResource(monitoringv1.ServiceMonitorName))
	if err != nil {
		return nil, err
	}

	podMonitorInformers, err := informers.NewInformersForResource(factory, monitoringv1.SchemeGroupVersion.WithResource(monitoringv1.PodMonitorName))
	if err != nil {
		return nil, err
	}

	probeInformers := &informers.ForResource{}
	if probeCRDInstalled {
		probeInformers, err = informers.NewInformersForResource(factory, monitoringv1.SchemeGroupVersion.WithResource(monitoringv1.ProbeName))
		if err != nil {
			return nil, err
		}
	}

	scrapeConfigInformers := &informers.ForResource{}
	if scrapeConfigCRDInstalled {
		scrapeConfigInformers, err = informers.NewInformersForResource(factory, promv1alpha1.SchemeGroupVersion.WithResource(promv1alpha1.ScrapeConfigName))
		if err != nil {
			return nil, err
		}
	}

	secretInformers, err := informers.NewInformersForResourceWithTransform(metaDataInformerFactory, v1.SchemeGroupVersion.WithResource(string(v1.ResourceSecrets)), informers.PartialObjectMetadataStrip)
	if err != nil {
		return nil, err
	}

	return map[string]*informers.ForResource{
		monitoringv1.ServiceMonitorName: serviceMonitorInformers,
		monitoringv1.PodMonitorName:     podMonitorInformers,
		monitoringv1.ProbeName:          probeInformers,
		promv1alpha1.ScrapeConfigName:   scrapeConfigInformers,
		string(v1.ResourceSecrets):      secretInformers,
	}, nil
}

// Watch wrapped informers and wait for an initial sync.
func (w *PrometheusCRWatcher) Watch(ctx context.Context, upstreamEvents chan Event, upstreamErrors chan error) error {
	success := true
	// this channel needs to be buffered because notifications are asynchronous and neither producers nor consumers wait
	notifyEvents := make(chan struct{}, 1)

	if w.nsInformer != nil {
		go w.nsInformer.Run(w.stopChannel)
		if ok := w.WaitForNamedCacheSync("namespace", w.nsInformer.HasSynced); !ok {
			success = false
		}

		_, _ = w.nsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			UpdateFunc: func(oldObj, newObj interface{}) {
				w.logger.Info("rashmi-logs: Inside nsinformer UpdateFunc")
				old := oldObj.(*v1.Namespace)
				cur := newObj.(*v1.Namespace)

				// Periodic resync may resend the Namespace without changes
				// in-between.
				if old.ResourceVersion == cur.ResourceVersion {
					w.logger.Info("rashmi-logs: Skipping namespace update event as resource version has not changed")
					return
				}

				for name, selector := range map[string]*metav1.LabelSelector{
					"PodMonitorNamespaceSelector":     w.podMonitorNamespaceSelector,
					"ServiceMonitorNamespaceSelector": w.serviceMonitorNamespaceSelector,
					"ProbeNamespaceSelector":          w.probeNamespaceSelector,
					"ScrapeConfigNamespaceSelector":   w.scrapeConfigNamespaceSelector,
				} {
					w.logger.Info("rashmi-logs: Namespace update detected", "oldResourceName", old.Name, "newResourceName", cur.Name)
					sync, err := k8sutil.LabelSelectionHasChanged(old.Labels, cur.Labels, selector)
					if err != nil {
						w.logger.Error("Failed to check label selection between namespaces while handling namespace updates", "selector", name, "error", err)
						return
					}

					if sync {
						select {
						case notifyEvents <- struct{}{}:
						default:
						}
						return
					}
				}
			},
		})
	} else {
		w.logger.Info("Unable to watch namespaces since namespace informer is nil")
	}

	for name, resource := range w.informers {
		resource.Start(w.stopChannel)

		if ok := w.WaitForNamedCacheSync(name, resource.HasSynced); !ok {
			w.logger.Info("skipping informer", "informer", name)
			continue
		}
		// Use a custom event handler for secrets since secret update requires asset store to be updated so that CRs can pick up updated secrets.
		if name == string(v1.ResourceSecrets) {
			w.logger.Info("rashmi-logs: Using custom event handler for secrets informer", "informer", name)
			// only send an event notification if there isn't one already
			resource.AddEventHandler(cache.ResourceEventHandlerFuncs{
				// these functions only write to the notification channel if it's empty to avoid blocking
				// if scrape config updates are being rate-limited
				AddFunc: func(obj interface{}) {
					select {
					case notifyEvents <- struct{}{}:
					default:
					}
				},
				UpdateFunc: func(oldObj, newObj interface{}) {
					w.logger.Info("rashmi-logs: Inside secret informer UpdateFunc")
					// Periodic resync may resend the Namespace without changes
					// in-between.
					// oldResource := oldObj.(*v1.Secret)
					// newResource := newObj.(*v1.Secret)
					oldMeta, _ := oldObj.(metav1.ObjectMetaAccessor)
					newMeta, _ := newObj.(metav1.ObjectMetaAccessor)
					// if oldResource.ResourceVersion == newResource.ResourceVersion {
					// 	w.logger.Info("rashmi-logs: Skipping secret update event as resource version has not changed")
					// 	return
					// }
					secretName := newMeta.GetObjectMeta().GetName()
					secretNamespace := newMeta.GetObjectMeta().GetNamespace()
					_, exists, err := w.store.GetObject(&v1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      secretName,
							Namespace: secretNamespace,
						},
					})
					if !exists || err != nil {
						if err != nil {
							w.logger.Error("unexpected store error when checking if secret exists, skipping update", secretName, "error", err)
							return
						}
						// if the secret does not exist in the store, we skip the update
						w.logger.Info(
							"rashmi-logs: Secret does not exist in store, skipping update",
							"newObjName", secretName,
							"newobjnamespace", secretNamespace,
						)
						return
					}

					newSecret, err := w.store.SecretClient().Secrets(secretNamespace).Get(ctx, secretName, metav1.GetOptions{})

					if err != nil {
						w.logger.Error("unexpected store error when getting updated secret - ", secretName, "error", err)
						return
					}

					w.logger.Info("rashmi-logs: Updating secret in store", "newObjName", newMeta.GetObjectMeta().GetName(), "newobjnamespace", newMeta.GetObjectMeta().GetNamespace())
					if err := w.store.UpdateObject(newSecret); err != nil {
						w.logger.Error("unexpected store error when updating secret  - ", newMeta.GetObjectMeta().GetName(), "error", err)
						//return
					} else {
						w.logger.Info(
							"rashmi-logs:Successfully updated store, sending update event to notifyEvents channel",
							"oldObjName", oldMeta.GetObjectMeta().GetName(),
							"oldobjnamespace", oldMeta.GetObjectMeta().GetNamespace(),
							"newObjName", newMeta.GetObjectMeta().GetName(),
							"newobjnamespace", newMeta.GetObjectMeta().GetNamespace(),
						)
						select {
						case notifyEvents <- struct{}{}:
						default:
						}
					}
				},
				DeleteFunc: func(obj interface{}) {
					// Periodic resync may resend the Namespace without changes
					// in-between.
					//secretResource := obj.(*v1.Secret)
					secretMeta, _ := obj.(metav1.ObjectMetaAccessor)

					w.logger.Info("rashmi-logs: Inside secret informer Delete Func")
					secretName := secretMeta.GetObjectMeta().GetName()
					secretNamespace := secretMeta.GetObjectMeta().GetNamespace()

					// check if the secret exists in the store
					w.logger.Info("rashmi-logs: Checking if secret exists in store", "objName", secretMeta.GetObjectMeta().GetName(), "objnamespace", secretMeta.GetObjectMeta().GetNamespace())
					secretObj := &v1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      secretName,
							Namespace: secretNamespace,
						},
					}
					_, exists, err := w.store.GetObject(secretObj)
					// if the secret does not exist in the store, we skip the delete
					if !exists || err != nil {
						if err != nil {
							w.logger.Error("unexpected store error when checking if secret exists, skipping delete", secretMeta.GetObjectMeta().GetName(), "error", err)
							return
						}
						// if the secret does not exist in the store, we skip the delete
						w.logger.Info(
							"rashmi-logs: Secret does not exist in store, skipping delete",
							"objName", secretMeta.GetObjectMeta().GetName(),
							"objnamespace", secretMeta.GetObjectMeta().GetNamespace(),
						)
						return
					}
					w.logger.Info("rashmi-logs: Deleting secret from store", "objName", secretMeta.GetObjectMeta().GetName(), "objnamespace", secretMeta.GetObjectMeta().GetNamespace())
					// if the secret exists in the store, we delete it
					// and send an event notification to the notifyEvents channel
					if err := w.store.DeleteObject(secretObj); err != nil {
						w.logger.Error("unexpected store error when deleting secret - ", secretMeta.GetObjectMeta().GetName(), "error", err)
						//return
					} else {
						w.logger.Info(
							"rashmi-logs:Successfully removed secret from store, sending update event to notifyEvents channel",
							"objName", secretMeta.GetObjectMeta().GetName(),
							"objnamespace", secretMeta.GetObjectMeta().GetNamespace(),
						)
						select {
						case notifyEvents <- struct{}{}:
						default:
						}
					}
				},
			})
		} else {
			w.logger.Info("rashmi-logs: Using default event handler for informer", "informer", name)
			// only send an event notification if there isn't one already
			resource.AddEventHandler(cache.ResourceEventHandlerFuncs{
				// these functions only write to the notification channel if it's empty to avoid blocking
				// if scrape config updates are being rate-limited
				AddFunc: func(obj interface{}) {
					select {
					case notifyEvents <- struct{}{}:
					default:
					}
				},
				UpdateFunc: func(oldObj, newObj interface{}) {
					select {
					case notifyEvents <- struct{}{}:
					default:
					}
				},
				DeleteFunc: func(obj interface{}) {
					select {
					case notifyEvents <- struct{}{}:
					default:
					}
				},
			})
		}
	}
	if !success {
		return fmt.Errorf("failed to sync one of the caches")
	}

	// limit the rate of outgoing events
	w.rateLimitedEventSender(upstreamEvents, notifyEvents)

	<-w.stopChannel
	return nil
}

// rateLimitedEventSender sends events to the upstreamEvents channel whenever it gets a notification on the notifyEvents channel,
// but not more frequently than once per w.eventPeriod.
func (w *PrometheusCRWatcher) rateLimitedEventSender(upstreamEvents chan Event, notifyEvents chan struct{}) {
	ticker := time.NewTicker(w.eventInterval)
	defer ticker.Stop()

	event := Event{
		Source:  EventSourcePrometheusCR,
		Watcher: Watcher(w),
	}

	for {
		select {
		case <-w.stopChannel:
			return
		case <-ticker.C: // throttle events to avoid excessive updates
			select {
			case <-notifyEvents:
				w.logger.Info("rashmi: New event received, sending upstream", "event", event.Source.String())
				select {
				case upstreamEvents <- event:
				default: // put the notification back in the queue if we can't send it upstream
					w.logger.Info("rashmi: Upstream channel full, re-queueing event", "event", event.Source.String())
					select {
					case notifyEvents <- struct{}{}:
					default:
					}
				}
			default:
			}
		}
	}
}

func (w *PrometheusCRWatcher) Close() error {
	close(w.stopChannel)
	return nil
}

func (w *PrometheusCRWatcher) LoadConfig(ctx context.Context) (*promconfig.Config, error) {
	promCfg := &promconfig.Config{}

	if w.resourceSelector != nil {
		serviceMonitorInstances, err := w.resourceSelector.SelectServiceMonitors(ctx, w.informers[monitoringv1.ServiceMonitorName].ListAllByNamespace)
		if err != nil {
			return nil, err
		}

		podMonitorInstances, err := w.resourceSelector.SelectPodMonitors(ctx, w.informers[monitoringv1.PodMonitorName].ListAllByNamespace)
		if err != nil {
			return nil, err
		}

		var probeInstances map[string]*monitoringv1.Probe
		if w.probeCRDInstalled {
			probeInstances, err = w.resourceSelector.SelectProbes(ctx, w.informers[monitoringv1.ProbeName].ListAllByNamespace)
			if err != nil {
				return nil, err
			}
		} else {
			probeInstances = make(map[string]*monitoringv1.Probe)
		}

		var scrapeConfigInstances map[string]*promv1alpha1.ScrapeConfig
		if w.scrapeConfigCRDInstalled {
			scrapeConfigInstances, err = w.resourceSelector.SelectScrapeConfigs(ctx, w.informers[promv1alpha1.ScrapeConfigName].ListAllByNamespace)
			if err != nil {
				return nil, err
			}
		} else {
			scrapeConfigInstances = make(map[string]*promv1alpha1.ScrapeConfig)
		}

		generatedConfig, err := w.configGenerator.GenerateServerConfiguration(
			w.prometheusCR,
			serviceMonitorInstances,
			podMonitorInstances,
			probeInstances,
			scrapeConfigInstances,
			w.store,
			nil,
			nil,
			nil,
			[]string{})
		if err != nil {
			return nil, err
		}

		generatedConfig, err = applyPromConfigDefaults(generatedConfig)
		if err != nil {
			return nil, err
		}

		unmarshalErr := yaml.Unmarshal(generatedConfig, promCfg)
		if unmarshalErr != nil {
			return nil, unmarshalErr
		}

		// // set kubeconfig path to service discovery configs, else kubernetes_sd will always attempt in-cluster
		// // authentication even if running with a detected kubeconfig
		for _, scrapeConfig := range promCfg.ScrapeConfigs {
			for _, serviceDiscoveryConfig := range scrapeConfig.ServiceDiscoveryConfigs {
				if serviceDiscoveryConfig.Name() == "kubernetes" {
					sdConfig := interface{}(serviceDiscoveryConfig).(*kubeDiscovery.SDConfig)
					sdConfig.KubeConfig = w.kubeConfigPath
				}
			}
		}
		return promCfg, nil
	} else {
		w.logger.Info("Unable to load config since resource selector is nil, returning empty prometheus config")
		return promCfg, nil
	}
}

// WaitForNamedCacheSync adds a timeout to the informer's wait for the cache to be ready.
// If the PrometheusCRWatcher is unable to load an informer within 15 seconds, the method is
// cancelled and returns false. A successful informer load will return true. This method also
// will be cancelled if the target allocator's stopChannel is called before it returns.
//
// This method is inspired by the upstream prometheus-operator implementation, with a shorter timeout
// and support for the PrometheusCRWatcher's stopChannel.
// https://github.com/prometheus-operator/prometheus-operator/blob/293c16c854ce69d1da9fdc8f0705de2d67bfdbfa/pkg/operator/operator.go#L433
func (w *PrometheusCRWatcher) WaitForNamedCacheSync(controllerName string, inf cache.InformerSynced) bool {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	t := time.NewTicker(time.Second * 5)
	defer t.Stop()

	go func() {
		for {
			select {
			case <-t.C:
				w.logger.Debug("cache sync not yet completed")
			case <-ctx.Done():
				return
			case <-w.stopChannel:
				w.logger.Warn("stop received, shutting down cache syncing")
				cancel()
				return
			}
		}
	}()

	ok := cache.WaitForNamedCacheSync(controllerName, ctx.Done(), inf)
	if !ok {
		w.logger.Error("failed to sync cache")
	} else {
		w.logger.Debug("successfully synced cache")
	}

	return ok
}

// applyPromConfigDefaults applies our own defaults to the Prometheus configuration. The unmarshalling process for
// Prometheus config is quite involved, and as a result, we need to apply our own defaults before it happens.
func applyPromConfigDefaults(configBytes []byte) ([]byte, error) {
	var configMap map[any]any
	err := yaml.Unmarshal(configBytes, &configMap)
	if err != nil {
		return nil, err
	}
	err = allocatorconfig.ApplyPromConfigDefaults(configMap)
	if err != nil {
		return nil, err
	}
	return yaml.Marshal(configMap)
}
