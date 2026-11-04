package utils

import (
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestQueryEveryCollectorPod(t *testing.T) {
	for _, size := range []int{1, 2, 5} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			pods := make([]corev1.Pod, size)
			for i := range pods {
				pods[i].Name = fmt.Sprintf("collector-%d", i)
			}
			queried := make(map[string]int)
			responses, err := queryPromUIPods(pods, func(name string) (*APIResponse, error) {
				queried[name]++
				return &APIResponse{PodName: name}, nil
			})
			if err != nil || len(responses) != size {
				t.Fatalf("want %d responses, got %d: %v", size, len(responses), err)
			}
			for i, pod := range pods {
				if queried[pod.Name] != 1 || responses[i].PodName != pod.Name {
					t.Fatalf("pod %s was omitted, duplicated or overwritten: %v", pod.Name, queried)
				}
			}
		})
	}
}

func TestQueryCollectorPodErrorsFailTheWholeSelection(t *testing.T) {
	pods := []corev1.Pod{{ObjectMeta: metav1.ObjectMeta{Name: "first"}}, {ObjectMeta: metav1.ObjectMeta{Name: "second"}}}
	for _, absent := range []bool{false, true} {
		responses, err := queryPromUIPods(pods, func(name string) (*APIResponse, error) {
			if name == "second" {
				if absent {
					return nil, nil
				}
				return nil, fmt.Errorf("request failed")
			}
			return &APIResponse{PodName: name}, nil
		})
		if err == nil || responses != nil {
			t.Fatalf("second pod failure must not become partial success: %v %v", responses, err)
		}
	}
	if _, err := queryPromUIPods(nil, nil); err == nil {
		t.Fatal("empty selection must fail")
	}
}
