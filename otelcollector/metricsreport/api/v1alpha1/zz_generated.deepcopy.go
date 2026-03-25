package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// --- HealthCheckRequest ---

func (in *HealthCheckRequest) DeepCopyInto(out *HealthCheckRequest) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
}

func (in *HealthCheckRequest) DeepCopy() *HealthCheckRequest {
	if in == nil {
		return nil
	}
	out := new(HealthCheckRequest)
	in.DeepCopyInto(out)
	return out
}

func (in *HealthCheckRequest) DeepCopyObject() runtime.Object {
	return in.DeepCopy()
}

func (in *HealthCheckRequestList) DeepCopyInto(out *HealthCheckRequestList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		out.Items = make([]HealthCheckRequest, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}

func (in *HealthCheckRequestList) DeepCopy() *HealthCheckRequestList {
	if in == nil {
		return nil
	}
	out := new(HealthCheckRequestList)
	in.DeepCopyInto(out)
	return out
}

func (in *HealthCheckRequestList) DeepCopyObject() runtime.Object {
	return in.DeepCopy()
}

// --- HealthSignal ---

func (in *HealthSignal) DeepCopyInto(out *HealthSignal) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
}

func (in *HealthSignal) DeepCopy() *HealthSignal {
	if in == nil {
		return nil
	}
	out := new(HealthSignal)
	in.DeepCopyInto(out)
	return out
}

func (in *HealthSignal) DeepCopyObject() runtime.Object {
	return in.DeepCopy()
}

func (in *HealthSignalList) DeepCopyInto(out *HealthSignalList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		out.Items = make([]HealthSignal, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}

func (in *HealthSignalList) DeepCopy() *HealthSignalList {
	if in == nil {
		return nil
	}
	out := new(HealthSignalList)
	in.DeepCopyInto(out)
	return out
}

func (in *HealthSignalList) DeepCopyObject() runtime.Object {
	return in.DeepCopy()
}

func (in *HealthSignalStatus) DeepCopyInto(out *HealthSignalStatus) {
	*out = *in
	if in.Conditions != nil {
		out.Conditions = make([]metav1.Condition, len(in.Conditions))
		for i := range in.Conditions {
			in.Conditions[i].DeepCopyInto(&out.Conditions[i])
		}
	}
}
