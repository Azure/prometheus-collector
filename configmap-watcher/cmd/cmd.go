// +gocover:ignore:file - main function
package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	lgr "go.goms.io/aks/configmap-watcher/logger"
	"go.uber.org/zap"

	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// ConfigMapSync holds the configmap information
type ConfigMapSync struct {
	namespace      string
	configmapName  string
	settingsVolume string
}

// KubeClient interface to create client set
type KubeClient interface {
	CreateClientSet(kubeconfigFile, userAgent string) (kubernetes.Interface, error)
}

// Kubectl implements KubeClient interface
type Kubectl struct{}

var (
	// ExitSignal 143=128+SIGTERM, https://tldp.org/LDP/abs/html/exitcodes.html
	ExitSignal         = 143
	kubeconfigFile     string
	settingsVolume     string
	configmapNamespace string
	configmapName      string
)

func run(ctx context.Context, cli KubeClient) error {
	logger := lgr.SetupLogger(os.Stdout, "configmap-watcher")
	defer logger.Sync() //nolint:errcheck

	configmapInfo, err := validateParameters()
	if err != nil {
		logger.Error("Invalid parameter.", zap.Error(err))
		return fmt.Errorf("invalid parameter: %w", err)
	}

	// TODO: Find a way to get the version, commit and date from the build
	userAgent := fmt.Sprintf("configmap-watcher/%s %s/%s", "Version", "Commit", "Date")
	overlayClient, err := cli.CreateClientSet(kubeconfigFile, userAgent)
	if err != nil {
		logger.Error("failed to create overlay clientset", zap.Error(err))
		return err
	}

	err = WatchForChanges(ctx, overlayClient, logger, configmapInfo)
	if err != nil {
		logger.Error("failed to watch configmap changes", zap.Error(err))
		return err
	}

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		logger.Info("os interrupt SIGTERM, exiting...")
		os.Exit(ExitSignal)
	}()

	return nil
}

// NewKubeCommand creates root cobra command
func NewKubeCommand(cli KubeClient) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "configmap-watcher",
		Short: "This binary will watch a configmap and load the values in a pod volume",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd.Context(), cli)
		},
	}

	rootCmd.Flags().StringVar(&kubeconfigFile, "kubeconfig-file", "", "Path to the kubeconfig")
	rootCmd.Flags().StringVar(&configmapNamespace, "configmap-namespace", "", "The configmap namespace")
	rootCmd.Flags().StringVar(&configmapName, "configmap-name", "", "The configmap name")
	rootCmd.Flags().StringVar(&settingsVolume, "settings-volume", "", "Directory where the settings files are stored")

	return rootCmd
}

// CreateClientSet createOverlayKubeClient constructs a kube client instance for current overlay cluster.
func (cli *Kubectl) CreateClientSet(kubeconfigFile, userAgent string) (kubernetes.Interface, error) {
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigFile)
	if err != nil {
		return nil, fmt.Errorf("error creating clientset from kubeconfig: %w", err)
	}
	cfg.UserAgent = userAgent
	return kubernetes.NewForConfig(cfg)
}

func validateParameters() (*ConfigMapSync, error) {
	if kubeconfigFile == "" {
		return nil, errors.New("--kubeconfig-file is required")
	}

	if settingsVolume == "" {
		return nil, errors.New("--settings-volume is required")
	}

	if configmapName == "" {
		return nil, errors.New("--configmap-name is required")
	}

	if configmapNamespace == "" {
		return nil, errors.New("--configmap-namespace is required")
	}

	return &ConfigMapSync{
		namespace:      configmapNamespace,
		configmapName:  configmapName,
		settingsVolume: settingsVolume,
	}, nil
}
