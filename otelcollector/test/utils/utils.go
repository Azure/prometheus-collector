package utils

import (
	"context"
	"errors"
	"flag"
	"path/filepath"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/util/homedir"

	"bytes"
	"fmt"
	"io"
)

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
		return nil, nil, err
	}
	
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, nil, err
	}

	return client, cfg, nil
}

// CheckContainerLogsForErrors checks the logs of containers in pods with a specific label for any errors.
// It takes a Kubernetes clientset, namespace, label name, and label value as input parameters.
// The function retrieves all pods with the given label and then checks the logs of each container in each pod.
// If any error is found in the logs, the function returns an error message indicating the container and pod name.
// If no errors are found, the function returns nil.

// Example usage:
// clientset, _ := kubernetes.NewForConfig(config)
// namespace := "default"
// labelName := "app"
// labelValue := "myapp"
// err := CheckContainerLogsForErrors(clientset, namespace, labelName, labelValue)
// if err != nil {
//     fmt.Println("Error:", err)
// }
func CheckContainerLogsForErrors(clientset *kubernetes.Clientset, namespace, labelName, labelValue string) error {
	// Get all pods with the given label
	pods, err := GetPodsWithLabel(clientset, namespace, labelName, labelValue)
	if err != nil {
		return err
	}

	// Check the logs of each container in each pod for errors
	for _, pod := range pods {
		for _, container := range pod.Spec.Containers {
			logs, err := getContainerLogs(clientset, pod.Namespace, pod.Name, container.Name)
			if err != nil {
				return err
			}

			if strings.Contains(logs, "error") || strings.Contains(logs, "Error") {

				// Get the exact log line of the error
				for _, line := range strings.Split(logs, "\n") {
					if strings.Contains(line, "error") || strings.Contains(line, "Error") {
						return fmt.Errorf("Logs for container %s in pod %s contain errors:\n %s", container.Name, pod.Name, line)
					}
				}
				return fmt.Errorf("Logs for container %s in pod %s contain errors", container.Name, pod.Name)
			}
		}
	}
	return nil
}


func GetPodsWithLabel(clientset *kubernetes.Clientset, namespace string, labelKey string, labelValue string) ([]corev1.Pod, error) {
	podList, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelKey + "=" + labelValue,
	})
	if err != nil {
		return nil, err
	}
	if podList == nil {
		return nil, fmt.Errorf("no pods found with label %s=%s", labelKey, labelValue)
	}

	return podList.Items, nil
}

func getContainerLogs(clientset *kubernetes.Clientset, namespace string, podName string, containerName string) (string, error) {
	req := clientset.CoreV1().RESTClient().Get().
		Namespace(namespace).
		Name(podName).
		Resource("pods").
		SubResource("log").
		Param("container", containerName).
		Param("timestamps", "true")

	readCloser, err := req.Stream(context.Background())
	if err != nil {
		return "", fmt.Errorf("failed to get logs for container %s in pod %s: %v", containerName, podName, err)
	}
	defer readCloser.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, readCloser)
	if err != nil {
		return "", fmt.Errorf("failed to read logs for container %s in pod %s: %v", containerName, podName, err)
	}

	return buf.String(), nil
}

// ExecCmd exec command on specific pod and wait the command's output.
func ExecCmd(client *kubernetes.Clientset, config *rest.Config, podName string, containerName string, namespace string, command []string) (stdout string, stderr string, err error) {
	req := client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec")
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		return "", "", errors.New(fmt.Sprintf("Error setting up exec request: %v", err))
	}

	parameterCodec := runtime.NewParameterCodec(scheme)
	req.VersionedParams(&corev1.PodExecOptions{
		Command:   command,
		Container: containerName,
		Stdin:     false,
		Stdout:    true,
		Stderr:    true,
		TTY:       false,
	}, parameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return "", "", errors.New(fmt.Sprintf("Error while creating command executor: %v", err))
	}

	var stdoutB, stderrB bytes.Buffer
	if err := exec.Stream(remotecommand.StreamOptions{
		Stdout: &stdoutB,
		Stderr: &stderrB,
	}); err != nil {
		return stdoutB.String(), stderrB.String(), errors.New(fmt.Sprintf("Error when running command %v in the container: %v", command, err))
	}

	return stdoutB.String(), stderrB.String(), nil
}

func CheckContainerStatus(K8sClient *kubernetes.Clientset, Cfg *rest.Config, labelName, labelValue, namespace, containerName, terminatedMessage, processName string, timeout int64) error {
	pods, err := GetPodsWithLabel(K8sClient, namespace, labelName, labelValue)
	if err != nil {
		return err
	}

	for _, pod := range pods {
		// Execute the command and capture the output
		command := []string{"sh", "-c", fmt.Sprintf("kill -9 $(ps ax | grep \"%s\" | fgrep -v grep | awk '{ print $1 }')", processName)}
		_, stderr, err := ExecCmd(K8sClient, Cfg, pod.Name, containerName, namespace, command)
		if err != nil {
			return err
		}

		// Check the output for any errors
		if stderr != "" {
			return fmt.Errorf("stderr: %s", stderr)
		}

		watcher, err := K8sClient.CoreV1().Pods(namespace).Watch(context.Background(), metav1.ListOptions{
			LabelSelector:   fmt.Sprintf("%s=%s", labelName, labelValue),
			TimeoutSeconds: &timeout,
		})
		if err != nil {
			return err
		}
		defer watcher.Stop()

		for {
			select {
			case event, ok := <-watcher.ResultChan():
				if !ok {
					return fmt.Errorf("watcher.ResultChan() closed unexpectedly")
				}
				if event.Type != "MODIFIED" {
					continue
				}

				p, ok := event.Object.(*corev1.Pod)
				if !ok {
					return fmt.Errorf("event.Object is not of type *corev1.Pod")
				}

				// Check ContainerStateTerminated for prometheus-collector container
				for _, containerStatus := range p.Status.ContainerStatuses {
					if containerStatus.Name == containerName && containerStatus.LastTerminationState.Terminated != nil {
						if containerStatus.LastTerminationState.Terminated.Reason == "Error" && strings.Contains(containerStatus.LastTerminationState.Terminated.Message, terminatedMessage) {
							return nil
						}
					}
				}
			}
		}
		break
	}

	return nil
}

func CheckAllProcessesRunning(K8sClient *kubernetes.Clientset, Cfg *rest.Config, labelName, labelValue, namespace, containerName string, processes []string) error {
	var processesGrepStringBuilder strings.Builder
	for _, process := range processes {
		processesGrepStringBuilder.WriteString(fmt.Sprintf("ps | grep \"%s\" | grep -v grep && ", process))
	}
	processesGrepString := strings.TrimSuffix(processesGrepStringBuilder.String(), " && ")

	command := []string{"bash", "-c", processesGrepString}
	pods, err := GetPodsWithLabel(K8sClient, namespace, labelName, labelValue)
	if err != nil {
		return errors.New(fmt.Sprintf("Error when getting pods with label %s=%s: %v", labelName, labelValue, err))
	}

	for _, pod := range pods {
		_, _, err := ExecCmd(K8sClient, Cfg, pod.Name, containerName, namespace, command)
		if err != nil {
			return errors.New(fmt.Sprintf("Error when running command %v in the container: %v", command, err))
		}
	}
	return nil
}