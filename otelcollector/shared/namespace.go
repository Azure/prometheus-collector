package shared

import "os"

// GetNamespace returns the namespace the pod is running in.
// Reads POD_NAMESPACE (set via Kubernetes downward API), defaulting to "kube-system".
func GetNamespace() string {
	if ns := os.Getenv("POD_NAMESPACE"); ns != "" {
		return ns
	}
	return "kube-system"
}
