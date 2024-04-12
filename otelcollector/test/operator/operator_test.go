package operator

import (
	"context"
	"fmt"

	"prometheus-collector/otelcollector/test/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Calling the API server using the Prometheus Operator client", Label(utils.OperatorLabel), func() {
  It("should get the pod monitor custom resources", func() {
    podMonitors, err := PromClient.MonitoringV1().PodMonitors("").List(context.Background(), metav1.ListOptions{})
    Expect(err).NotTo(HaveOccurred())
		Expect(len(podMonitors.Items)).To(BeNumerically(">", 0))

    fmt.Printf("Found %d Pod Monitors:\n", len(podMonitors.Items))
    for _, pm := range podMonitors.Items {
      fmt.Printf("- Name: %s\n", pm.Name)
      fmt.Printf("  Namespace: %s\n", pm.Namespace)
      fmt.Printf("  Labels: %v\n", pm.Labels)
      fmt.Printf("  Spec: %v\n", pm.Spec)
    }
  })

  It("should get the service monitor custom resources", func() {
    serviceMonitors, err := PromClient.MonitoringV1().ServiceMonitors("").List(context.Background(), metav1.ListOptions{})
    Expect(err).NotTo(HaveOccurred())
		Expect(len(serviceMonitors.Items)).To(BeNumerically(">", 0))

    fmt.Printf("Found %d Service Monitors:\n", len(serviceMonitors.Items))
    for _, sm := range serviceMonitors.Items {
      fmt.Printf("- Name: %s\n", sm.Name)
      fmt.Printf("  Namespace: %s\n", sm.Namespace)
      fmt.Printf("  Labels: %v\n", sm.Labels)
      fmt.Printf("  Spec: %v\n", sm.Spec)
    }
  })
})