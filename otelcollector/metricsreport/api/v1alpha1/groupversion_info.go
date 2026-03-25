// Package v1alpha1 contains the SchemeBuilder for health.aks.io types.
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// GroupVersion is the API group and version for AKS Health Signal CRs.
	GroupVersion = schema.GroupVersion{Group: "health.aks.io", Version: "v1alpha1"}

	// SchemeBuilder is used to add Go types to the GroupVersionKind scheme.
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(GroupVersion,
		&HealthCheckRequest{},
		&HealthCheckRequestList{},
		&HealthSignal{},
		&HealthSignalList{},
	)
	return nil
}
