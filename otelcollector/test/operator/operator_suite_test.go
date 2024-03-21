package operator

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

/*
 * These tests MUST be run with the flag:
 * -ldflags="-s -X github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring.GroupName=azmonitoring.coreos.com"
 * in order for the prometheus-operator package to get CRs using our custom API group name.
 */
func TestOperator(t *testing.T) {
  RegisterFailHandler(Fail)

  RunSpecs(t, "Operator Test Suite")
}

var _ = BeforeSuite(func() {
})

var _ = AfterSuite(func() {
  By("tearing down the test environment")
})
