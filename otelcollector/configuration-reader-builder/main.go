package main

import (
	"context"
	// "flag"
	"io/ioutil"
	"log"

	// "os"
	// "path/filepath"
	// "strings"

	// batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	// v1 "k8s.io/api/core/v1"
	promconfig "github.com/prometheus/prometheus/config"
	yaml "gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetes "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	// clientcmd "k8s.io/client-go/tools/clientcmd"
)

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

type Config struct {
	LabelSelector      map[string]string  `yaml:"label_selector,omitempty"`
	Config             *promconfig.Config `yaml:"config"`
	AllocationStrategy string             `yaml:"allocation_strategy,omitempty"`
	// FilterStrategy         *string            `yaml:"filter_strategy,omitempty"`
	// PodMonitorSelector     map[string]string  `yaml:"pod_monitor_selector,omitempty"`
	// ServiceMonitorSelector map[string]string  `yaml:"service_monitor_selector,omitempty"`
}

func updateConfigMap(clientset *kubernetes.Clientset) {
	targetAllocatorConfigmap := "ama-metrics-otelcollector-targetallocator"
	configMaps, err := clientset.CoreV1().ConfigMaps("kube-system").Get(context.TODO(), targetAllocatorConfigmap, metav1.GetOptions{})
	// jobs := clientset.BatchV1().Jobs("default")
	// var backOffLimit int32 = 0

	// _, err := jobs.Create(context.TODO(), jobSpec, metav1.CreateOptions{})
	if err != nil {
		log.Fatalln("Failed to get configmpap")
	}

	//print job details
	log.Println("Got configmaps successfully - Name: ", configMaps.Name)
	configMapClient := clientset.CoreV1().ConfigMaps("kube-system")

	defaultsMergedConfigFileContents, err := ioutil.ReadFile("/opt/defaultsMergedConfig.yml")
	if err != nil {
		panic(err)
	}
	var promScrapeConfig *promconfig.Config
	err = yaml.Unmarshal([]byte(defaultsMergedConfigFileContents), &promScrapeConfig)
	if err != nil {
		panic(err)
	}

	targetAllocatorConfig := Config{
		AllocationStrategy: "consistent-hashing",
		LabelSelector: map[string]string{
			"app.kubernetes.io/component": "opentelemetry-collector",
			"app.kubernetes.io/instance":  "kube-system.ama-metrics-otelcollector",
		},
		Config: promScrapeConfig,
	}

	// defaultTargetsFileContents, err := ioutil.ReadFile("defaultTargets.yaml")
	// if err != nil {
	// 	panic(err)
	// }
	// defaultTargetsFileContentsString := string(defaultTargetsFileContents)

	targetAllocatorConfigYaml, _ := yaml.Marshal(targetAllocatorConfig)
	newConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "demo-configmap",
		},
		Data: map[string]string{
			"targetallocator.yaml": string(targetAllocatorConfigYaml),
		},
	}

	// result, err := configMapClient.Create(context.TODO(), newConfigMap, metav1.CreateOptions{})
	result, err := configMapClient.Update(context.TODO(), newConfigMap, metav1.UpdateOptions{})

	if err != nil {
		panic(err)
	}
	log.Println("Updated configmap - ", result.GetObjectMeta().GetName())
}

// log.Println("Got configmaps successfully - Data: ", configMaps.Data["targetallocator.yaml"](string))

// }

func main() {
	// jobName := flag.String("jobname", "test-job", "The name of the job")
	// containerImage := flag.String("image", "ubuntu:latest", "Name of the container image")
	// entryCommand := flag.String("command", "ls", "The command to run inside the container")

	// flag.Parse()

	clientset := connectToK8s()
	updateConfigMap(clientset)
	//launchK8sJob(clientset, jobName, containerImage, entryCommand)
}
