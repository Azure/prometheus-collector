package controller

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	healthv1alpha1 "prometheus-collector/metricsreport/api/v1alpha1"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
)

const defaultPrometheusAPIURL = "http://localhost:9092"

// activeUpgradeGate holds a reference to the UpgradeGate created during
// controller startup, so the collector exporter can access it for loading
// customer-defined rule metric names.
var activeUpgradeGate *UpgradeGate

// GetUpgradeGate returns the UpgradeGate created during controller startup.
func GetUpgradeGate() *UpgradeGate {
	return activeUpgradeGate
}

// StartHealthSignalController starts the controller with a new internal cache.
// Used when running in a separate process from the collector.
func StartHealthSignalController(ctx context.Context) error {
	cache := NewMetricsCache(1*time.Hour, 15*time.Second)
	return StartHealthSignalControllerWithCache(ctx, cache)
}

// StartHealthSignalControllerWithCache starts the controller using a provided
// MetricsCache. Used when the controller runs inside the collector process so
// the cache can be shared with the health_cache exporter.
func StartHealthSignalControllerWithCache(ctx context.Context, cache *MetricsCache) error {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(healthv1alpha1.AddToScheme(scheme))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
	})
	if err != nil {
		return fmt.Errorf("creating controller manager: %w", err)
	}

	prometheusURL := os.Getenv("PROMETHEUS_API_URL")
	if prometheusURL == "" {
		prometheusURL = defaultPrometheusAPIURL
	}
	log.Printf("HealthSignal controller using Prometheus API at %s", prometheusURL)

	upgradeGate := NewUpgradeGate(mgr.GetClient())
	activeUpgradeGate = upgradeGate

	reconciler := &HealthSignalReconciler{
		Client:           mgr.GetClient(),
		Scheme:           mgr.GetScheme(),
		PrometheusAPIURL: prometheusURL,
		Cache:            cache,
		UpgradeGate:      upgradeGate,
	}

	if err := reconciler.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("setting up HealthSignal controller: %w", err)
	}

	go func() {
		log.Println("Starting HealthSignal controller manager")
		if err := mgr.Start(ctx); err != nil {
			log.Printf("HealthSignal controller manager error: %v", err)
		}
	}()

	return nil
}
