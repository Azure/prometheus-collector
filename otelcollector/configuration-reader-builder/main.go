package main

import (
	"flag"

	"io/ioutil"
	"log"

	// "os"
	// "path/filepath"
	// "strings"

	// batchv1 "k8s.io/api/batch/v1"

	// v1 "k8s.io/api/core/v1"
	// promconfig "github.com/prometheus/prometheus/config"
	yaml "gopkg.in/yaml.v2"
	// clientcmd "k8s.io/client-go/tools/clientcmd"
)

// func connectToK8s() *kubernetes.Clientset {
// 	config, err := rest.InClusterConfig()

// 	if err != nil {
// 		log.Fatalln("failed to create K8s config")
// 	}

// 	clientset, err := kubernetes.NewForConfig(config)
// 	if err != nil {
// 		log.Fatalln("Failed to create K8s clientset")
// 	}

// 	return clientset
// }

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

var taConfigFilePath = "/ta-configuration/targetallocator.yaml"

// func updateConfigMap(clientset *kubernetes.Clientset, configFilePath string) {
func updateTAConfigFile(configFilePath string) {
	// targetAllocatorConfigmap := "ama-metrics-otelcollector-targetallocator"
	// configMapClient := clientset.CoreV1().ConfigMaps("kube-system")

	defaultsMergedConfigFileContents, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		panic(err)
	}
	// var promScrapeConfig *promconfig.Config
	var promScrapeConfig map[string]interface{}
	var otelConfig OtelConfig
	err = yaml.Unmarshal([]byte(defaultsMergedConfigFileContents), &otelConfig)
	if err != nil {
		panic(err)
	}

	promScrapeConfig = otelConfig.Receivers.Prometheus.Config
	targetAllocatorConfig := Config{
		AllocationStrategy: "consistent-hashing",
		LabelSelector: map[string]string{
			"app.kubernetes.io/component": "opentelemetry-collector",
			"app.kubernetes.io/instance":  "kube-system.ama-metrics-otelcollector",
		},
		Config: promScrapeConfig,
	}

	targetAllocatorConfigYaml, _ := yaml.Marshal(targetAllocatorConfig)
	// newConfigMap := &corev1.ConfigMap{
	// 	ObjectMeta: metav1.ObjectMeta{
	// 		Name: targetAllocatorConfigmap,
	// 	},
	// 	Data: map[string]string{
	// 		"targetallocator.yaml": string(targetAllocatorConfigYaml),
	// 	},
	// }

	// result, err := configMapClient.Update(context.TODO(), newConfigMap, metav1.UpdateOptions{})
	if err := ioutil.WriteFile(taConfigFilePath, targetAllocatorConfigYaml, 0644); err != nil {
		panic(err)
	}
	// if err != nil {
	// 	panic(err)
	// }
	// log.Println("Updated configmap - ", result.GetObjectMeta().GetName())
	log.Println("Updated file - targetallocator.yaml for the TargetAllocator to pick up new config changes")
}

func main() {
	configFilePtr := flag.String("config", "", "Config file to read")
	flag.Parse()
	otelConfigFilePath := *configFilePtr
	// clientset := connectToK8s()
	// updateConfigMap(clientset, otelConfigFilePath)
	updateTAConfigFile(otelConfigFilePath)
}
