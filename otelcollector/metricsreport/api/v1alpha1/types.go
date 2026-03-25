// Package v1alpha1 contains API types for the health.aks.io v1alpha1 API group.
//
// These types mirror the AKS Health Signal CRDs:
//   - HealthCheckRequest: Created by AKS RP to request health monitoring during upgrades.
//   - HealthSignal: Created by monitoring apps (this controller) in response.
//
// +kubebuilder:object:generate=true
// +groupName=health.aks.io
package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// --- HealthCheckRequest ---

// HealthCheckRequestScope defines the level of the health check.
// +kubebuilder:validation:Enum=Node;NodePool;Cluster
type HealthCheckRequestScope string

const (
	HealthCheckRequestScopeNode     HealthCheckRequestScope = "Node"
	HealthCheckRequestScopeNodePool HealthCheckRequestScope = "NodePool"
	HealthCheckRequestScopeCluster  HealthCheckRequestScope = "Cluster"
)

// HealthCheckRequestSpec defines the desired state of a HealthCheckRequest.
type HealthCheckRequestSpec struct {
	// Scope is the level of the health check: Node, NodePool, or Cluster.
	// +kubebuilder:validation:Required
	Scope HealthCheckRequestScope `json:"scope"`
	// TargetName is the name of the target (node name, node pool name, or cluster name).
	// +kubebuilder:validation:Required
	TargetName string `json:"targetName"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName=hcr

// HealthCheckRequest is created by the AKS RP to request health monitoring.
type HealthCheckRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec HealthCheckRequestSpec `json:"spec"`
}

// +kubebuilder:object:root=true

// HealthCheckRequestList contains a list of HealthCheckRequest.
type HealthCheckRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HealthCheckRequest `json:"items"`
}

// --- HealthSignal ---

// HealthSignalType defines the type of health signal.
// +kubebuilder:validation:Enum=NodeHealth;ClusterHealth
type HealthSignalType string

const (
	NodeHealth    HealthSignalType = "NodeHealth"
	ClusterHealth HealthSignalType = "ClusterHealth"
)

// Condition status constants matching the AKS Health Signal spec.
const (
	ConditionHealthy   = "True"
	ConditionUnhealthy = "False"
	ConditionOngoing   = "Unknown"
)

// HealthSignalSpec defines the desired state of a HealthSignal.
type HealthSignalSpec struct {
	// Type is the health signal type (NodeHealth or ClusterHealth).
	// +kubebuilder:validation:Required
	Type HealthSignalType `json:"type"`
	// TargetRef identifies the Kubernetes object this health signal targets.
	TargetRef corev1.ObjectReference `json:"targetRef"`
}

// HealthSignalStatus contains observed health conditions.
type HealthSignalStatus struct {
	// Conditions is the list of health conditions.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=hs

// HealthSignal is created by monitoring apps in response to a HealthCheckRequest.
// Each HealthSignal MUST set an ownerReference to its HealthCheckRequest.
type HealthSignal struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HealthSignalSpec   `json:"spec"`
	Status HealthSignalStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// HealthSignalList contains a list of HealthSignal.
type HealthSignalList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HealthSignal `json:"items"`
}
