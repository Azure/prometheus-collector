package controller

import (
	"context"
	"fmt"
	"log"

	healthv1alpha1 "prometheus-collector/metricsreport/api/v1alpha1"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
)

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

	reconciler := &HealthSignalReconciler{
		Client:           mgr.GetClient(),
		Scheme:           mgr.GetScheme(),
		PrometheusAPIURL: "http://localhost:9092",
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
