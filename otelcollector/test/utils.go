package e2e

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"

	"bytes"
	"fmt"
	"io"
)

func CheckContainerLogsForErrors(namespace, labelName, labelValue string) error {
	// Get all pods with the given label
	pods, err := getPodsWithLabel(K8sClient, namespace, labelName, labelValue)
	if err != nil {
		return err
	}

	// Check the logs of each container in each pod for errors
	for _, pod := range pods {
		for _, container := range pod.Spec.Containers {
			logs, err := getContainerLogs(K8sClient, pod.Namespace, pod.Name, container.Name)
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

func getPodsWithLabel(clientset *kubernetes.Clientset, namespace string, labelKey string, labelValue string) ([]corev1.Pod, error) {
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
/*func ExecCmdExample(client *kubernetes.Clientset, podName string, namespace, string, command string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	cmd := []string{
			"sh",
			"-c",
			command,
	}
	req := client.CoreV1().RESTClient().Post().Resource("pods").Name(podName).
			Namespace(namespace).SubResource("exec")
	option := &v1.PodExecOptions{
			Command: cmd,
			Stdin:   true,
			Stdout:  true,
			Stderr:  true,
			TTY:     true,
	}
	if stdin == nil {
			option.Stdin = false
	}
	req.VersionedParams(
			option,
			scheme.ParameterCodec,
	)
	exec, err := remotecommand.NewSPDYExecutor(nil, "POST", req.URL())
	if err != nil {
			return err
	}
	err = exec.Stream(remotecommand.StreamOptions{
			Stdin:  stdin,
			Stdout: stdout,
			Stderr: stderr,
	})
	if err != nil {
			return err
	}

	return nil
}
*/