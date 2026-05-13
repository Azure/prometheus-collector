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

// StartHealthSignalController starts the controller manager in a background goroutine.
// It watches HealthCheckRequest CRs and creates/updates HealthSignal CRs based
// on Prometheus metrics from the local collector.
func StartHealthSignalController(ctx context.Context) error {
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

	// Metrics cache retains 1 hour of data points. The 15-second dedup TTL
	// prevents redundant Prometheus API calls within the same reconciliation cycle.
	metricsCache := NewMetricsCache(1*time.Hour, 15*time.Second)

	reconciler := &HealthSignalReconciler{
		Client:           mgr.GetClient(),
		Scheme:           mgr.GetScheme(),
		PrometheusAPIURL: prometheusURL,
		Cache:            metricsCache,
		UpgradeGate:      NewUpgradeGate(mgr.GetClient()),
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
