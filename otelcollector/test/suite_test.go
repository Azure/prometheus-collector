package e2e

import (
	"context"
	"testing"

	"k8s.io/client-go/kubernetes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/rest"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	cfg       		*rest.Config
	ctx       		context.Context
	cancel    		context.CancelFunc
)
var K8sClient 		*kubernetes.Clientset

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "E2E Test Suite")
}

var _ = BeforeSuite(func() {
	// creates the in-cluster config
	cfg, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	/*var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// use the current context in kubeconfig
	cfg, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}*/
	
	// creates the clientset
	K8sClient, err = kubernetes.NewForConfig(cfg)
	if err != nil {
		panic(err.Error())
	}

	ctx, cancel = context.WithCancel(context.TODO())
})

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
})