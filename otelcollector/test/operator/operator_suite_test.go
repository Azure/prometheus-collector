package operator

import (
	//"prometheus-collector/otelcollector/test/utils"
	"flag"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	//promOperatorClient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

//var K8sClient 	*kubernetes.Clientset
//var Cfg       	*rest.Config
//var PromClient 	promOperatorClient.Interface

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
  var err error
  //K8sClient, Cfg, err = utils.SetupKubernetesClient()
	_, _, err = SetupKubernetesClient()
  Expect(err).NotTo(HaveOccurred())
	//PromClient, err = promOperatorClient.NewForConfig(Cfg)
  //Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
  By("tearing down the test environment")
})

/*
 * Returns the Kubernetes API client and cluster configuration.
 * The function will first check if a kubeconfig file is present in the user's home directory, for running the tests locally.
 * If the file is not found, it will assume the tests are running in a Kubernetes cluster and use the in-cluster configuration.
 */
 func SetupKubernetesClient() (*kubernetes.Clientset, *rest.Config, error) {
  var kubeconfig *string
  if home := homedir.HomeDir(); home != "" {
    kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
  } else {
    kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
  }
  flag.Parse()

  cfg, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
  if err != nil {
    cfg, err = rest.InClusterConfig()
    if err != nil {
      return nil, nil, err
    }
  }
  
  client, err := kubernetes.NewForConfig(cfg)
  if err != nil {
    return nil, nil, err
  }

  return client, cfg, nil
}
