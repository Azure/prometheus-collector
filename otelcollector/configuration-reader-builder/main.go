package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"os"

	yaml "gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetes "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Config struct {
	LabelSelector      map[string]string      `yaml:"label_selector,omitempty"`
	Config             map[string]interface{} `yaml:"config"`
	AllocationStrategy string                 `yaml:"allocation_strategy,omitempty"`
}

type OtelConfig struct {
	Exporters  interface{} `yaml:"exporters"`
	Processors interface{} `yaml:"processors"`
	Extensions interface{} `yaml:"extensions"`
	Receivers  struct {
		Prometheus struct {
			Config          map[string]interface{} `yaml:"config"`
			TargetAllocator interface{}            `yaml:"target_allocator"`
		} `yaml:"prometheus"`
	} `yaml:"receivers"`
	Service struct {
		Extensions interface{} `yaml:"extensions"`
		Pipelines  struct {
			Metrics struct {
				Exporters  interface{} `yaml:"exporters"`
				Processors interface{} `yaml:"processors"`
				Receivers  interface{} `yaml:"receivers"`
			} `yaml:"metrics"`
		} `yaml:"pipelines"`
		Telemetry struct {
			Logs struct {
				Level    interface{} `yaml:"level"`
				Encoding interface{} `yaml:"encoding"`
			} `yaml:"logs"`
		} `yaml:"telemetry"`
	} `yaml:"service"`
}

var RESET = "\033[0m"
var RED = "\033[31m"

var taConfigFilePath = "/ta-configuration/targetallocator.yaml"

func logFatalError(message string) {
	// Always log the full message
	log.Fatalf("%s%s%s", RED, message, RESET)
}

func connectToK8s() *kubernetes.Clientset {
	// home, exists := os.LookupEnv("HOME")
	// if !exists {
	// 	home = "/root"
	// }

	// configPath := filepath.Join(home, ".kube", "config")

	// config, err := clientcmd.BuildConfigFromFlags("", configPath)
	// config, err := clientcmd.BuildConfigFromFlags("", "config")
	config, err := rest.InClusterConfig()

	if err != nil {
		log.Fatalln("failed to create K8s config")
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalln("Failed to create K8s clientset")
	}

	return clientset
}

func updateConfigMap(clientset *kubernetes.Clientset, configFilePath string) {
	// func updateTAConfigFile(configFilePath string) {
	targetAllocatorConfigmap := "ama-metrics-otelcollector-targetallocator"
	configMaps, err := clientset.CoreV1().ConfigMaps("kube-system").Get(context.TODO(), targetAllocatorConfigmap, metav1.GetOptions{})
	defaultsMergedConfigFileContents, err := os.ReadFile(configFilePath)
	if err != nil {
		logFatalError(fmt.Sprintf("config-reader::Unable to read file contents from: %s - %v\n", configFilePath, err))
		os.Exit(1)
	}
	log.Println("Got configmaps successfully - Name: ", configMaps.Name)
	configMapClient := clientset.CoreV1().ConfigMaps("kube-system")
	var promScrapeConfig map[string]interface{}
	var otelConfig OtelConfig
	err = yaml.Unmarshal([]byte(defaultsMergedConfigFileContents), &otelConfig)
	if err != nil {
		logFatalError(fmt.Sprintf("config-reader::Unable to unmarshal merged otel configuration from: %s - %v\n", configFilePath, err))
		os.Exit(1)
	}

	promScrapeConfig = otelConfig.Receivers.Prometheus.Config
	targetAllocatorConfig := Config{
		AllocationStrategy: "consistent-hashing",
		LabelSelector: map[string]string{
			"ama-metrics.component":          "ama-metrics-targetallocator",
			"ama-metrics.component/instance": "ama-metrics-targetallocator-ta-container",
		},
		Config: promScrapeConfig,
	}

	targetAllocatorConfigYaml, _ := yaml.Marshal(targetAllocatorConfig)
	newConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: targetAllocatorConfigmap,
		},
		Data: map[string]string{
			"targetallocator.yaml": string(targetAllocatorConfigYaml),
		},
	}

	result, err := configMapClient.Update(context.TODO(), newConfigMap, metav1.UpdateOptions{})
	log.Println("Updated configmap - ", result.GetObjectMeta().GetName())
	// if err := os.WriteFile(taConfigFilePath, targetAllocatorConfigYaml, 0644); err != nil {
	// 	logFatalError(fmt.Sprintf("config-reader::Unable to write to: %s - %v\n", taConfigFilePath, err))
	// 	os.Exit(1)
	// }

	log.Println("Updated file - targetallocator.yaml for the TargetAllocator to pick up new config changes")
}

func main() {
	configFilePtr := flag.String("config", "", "Config file to read")
	flag.Parse()
	otelConfigFilePath := *configFilePtr
	clientset := connectToK8s()
	updateConfigMap(clientset, otelConfigFilePath)
	// updateTAConfigFile(otelConfigFilePath)
}
