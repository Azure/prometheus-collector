package cmd_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"go.goms.io/aks/configmap-watcher/cmd"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	testclient "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	k8stesting "k8s.io/client-go/testing"

	"github.com/stretchr/testify/assert"
)

func TestSuccessCommandNoConfigmap(t *testing.T) {
	var cli cmd.KubeClient = &KubectlMock{}
	_, err := cli.CreateClientSet("kubeconfig-file", "user-agent")
	if err != nil {
		t.Fatalf("Error creating fake client: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tmpDir := t.TempDir()

	rootCmd := cmd.NewKubeCommand(cli)
	rootCmd.SetArgs([]string{
		"--kubeconfig-file=/config/fake/kubeconfig",
		"--settings-volume=" + tmpDir,
		"--configmap-name=ama-metrics-settings-configmap",
		"--configmap-namespace=kube-system",
	})

	go func() {
		if err := rootCmd.ExecuteContext(ctx); err != nil && !errors.Is(ctx.Err(), context.DeadlineExceeded) {
			t.Error("Command execution failed: %w", err)
			return
		}
	}()

	// Wait for the context to be done (either by command completion or timeout)
	<-ctx.Done()
}

func TestInvalidParameters(t *testing.T) {
	testCases := []struct {
		name     string
		args     []string
		testFunc func(t *testing.T, args []string)
	}{
		{
			name: "kubeconfig-file invalid parameter",
			args: []string{
				"--settings-volume=" + t.TempDir(),
				"--configmap-name=ama-metrics-settings-configmap",
				"--configmap-namespace=kube-system",
			},
			testFunc: func(t *testing.T, args []string) {
				var cli cmd.KubeClient = &KubectlMock{}
				rootCmd := cmd.NewKubeCommand(cli)
				rootCmd.SetArgs(args)

				err := rootCmd.Execute()
				assert.EqualError(t, err, "invalid parameter: --kubeconfig-file is required")
			},
		},
		{
			name: "settings-volume invalid parameter",
			args: []string{
				"--kubeconfig-file=/config/fake/kubeconfig",
				"--configmap-name=ama-metrics-settings-configmap",
				"--configmap-namespace=kube-system",
			},
			testFunc: func(t *testing.T, args []string) {
				var cli cmd.KubeClient = &KubectlMock{}
				rootCmd := cmd.NewKubeCommand(cli)
				rootCmd.SetArgs(args)

				err := rootCmd.Execute()
				assert.EqualError(t, err, "invalid parameter: --settings-volume is required")
			},
		},
		{
			name: "configmap-name invalid parameter",
			args: []string{
				"--kubeconfig-file=/config/fake/kubeconfig",
				"--settings-volume=" + t.TempDir(),
				"--configmap-namespace=kube-system",
			},
			testFunc: func(t *testing.T, args []string) {
				var cli cmd.KubeClient = &KubectlMock{}
				rootCmd := cmd.NewKubeCommand(cli)
				rootCmd.SetArgs(args)

				err := rootCmd.Execute()
				assert.EqualError(t, err, "invalid parameter: --configmap-name is required")
			},
		},
		{
			name: "configmap-namespace invalid parameter",
			args: []string{
				"--kubeconfig-file=/config/fake/kubeconfig",
				"--settings-volume=" + t.TempDir(),
				"--configmap-name=ama-metrics-settings-configmap",
			},
			testFunc: func(t *testing.T, args []string) {
				var cli cmd.KubeClient = &KubectlMock{}
				rootCmd := cmd.NewKubeCommand(cli)
				rootCmd.SetArgs(args)

				err := rootCmd.Execute()
				assert.EqualError(t, err, "invalid parameter: --configmap-namespace is required")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.testFunc(t, tc.args)
		})
	}
}

func TestSuccessCommandWhenConfigmapExists(t *testing.T) {
	data := loadConfigmapFromFile(t, "../tests/settings-configmap-create.yaml")
	fakeClient, watchInterface := createFakeClient()
	// the command runs indefinitely in a loop, therefore we need to cancel it after a while
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Simulate watch event
	tmpDir := executeConfigmapWatch(ctx, t, fakeClient)
	watchInterface.Add(data)
	time.Sleep(1 * time.Second)

	// Wait for the context to be done
	<-ctx.Done()

	// Assert result
	files, _ := os.ReadDir(tmpDir)
	assert.Equal(t, 9, len(files))
}

func loadConfigmapFromFile(t *testing.T, configmapFile string) *corev1.ConfigMap {
	// Read file content
	fileContent, err := os.ReadFile(configmapFile)
	if err != nil {
		t.Fatalf("Error reading configmap test file: %s", err)
	}

	// Decode the YAML into a ConfigMap object
	decode := serializer.NewCodecFactory(scheme.Scheme).UniversalDeserializer().Decode
	obj, _, err := decode(fileContent, nil, nil)
	if err != nil {
		t.Fatalf("Error decoding YAML to ConfigMap: %s", err)
	}

	configMap, ok := obj.(*corev1.ConfigMap)
	if !ok {
		t.Fatalf("Decoded object is not a ConfigMap, it is a %T", obj)
	}

	return configMap
}

func createFakeClient() (kubernetes.Interface, *watch.RaceFreeFakeWatcher) {
	fakeClient := testclient.NewSimpleClientset()
	watchInterface := watch.NewRaceFreeFake()
	fakeClient.PrependWatchReactor("configmaps", k8stesting.DefaultWatchReactor(watchInterface, nil))
	return fakeClient, watchInterface
}

// KubectlMock is a mock implementation of the KubeClient interface
type KubectlMock struct {
	kubeconfig string
	userAgent  string
	clientSet  kubernetes.Interface
}

func (cli *KubectlMock) CreateClientSet(kubeconfigFile, userAgent string) (kubernetes.Interface, error) {
	if cli.clientSet == nil {
		cli.clientSet = testclient.NewSimpleClientset()
		cli.userAgent = userAgent
		cli.kubeconfig = kubeconfigFile
	}

	return cli.clientSet, nil
}
