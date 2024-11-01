//go:build !ignore_autogenerated

/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CustomAlgorithm) DeepCopyInto(out *CustomAlgorithm) {
	*out = *in
	if in.Parameters != nil {
		in, out := &in.Parameters, &out.Parameters
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CustomAlgorithm.
func (in *CustomAlgorithm) DeepCopy() *CustomAlgorithm {
	if in == nil {
		return nil
	}
	out := new(CustomAlgorithm)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PowerCappingPolicy) DeepCopyInto(out *PowerCappingPolicy) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PowerCappingPolicy.
func (in *PowerCappingPolicy) DeepCopy() *PowerCappingPolicy {
	if in == nil {
		return nil
	}
	out := new(PowerCappingPolicy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PowerCappingPolicy) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PowerCappingPolicyList) DeepCopyInto(out *PowerCappingPolicyList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]PowerCappingPolicy, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PowerCappingPolicyList.
func (in *PowerCappingPolicyList) DeepCopy() *PowerCappingPolicyList {
	if in == nil {
		return nil
	}
	out := new(PowerCappingPolicyList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PowerCappingPolicyList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PowerCappingPolicySpec) DeepCopyInto(out *PowerCappingPolicySpec) {
	*out = *in
	in.Selector.DeepCopyInto(&out.Selector)
	if in.CustomAlgorithms != nil {
		in, out := &in.CustomAlgorithms, &out.CustomAlgorithms
		*out = make([]CustomAlgorithm, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PowerCappingPolicySpec.
func (in *PowerCappingPolicySpec) DeepCopy() *PowerCappingPolicySpec {
	if in == nil {
		return nil
	}
	out := new(PowerCappingPolicySpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PowerCappingPolicyStatus) DeepCopyInto(out *PowerCappingPolicyStatus) {
	*out = *in
	in.LastUpdated.DeepCopyInto(&out.LastUpdated)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PowerCappingPolicyStatus.
func (in *PowerCappingPolicyStatus) DeepCopy() *PowerCappingPolicyStatus {
	if in == nil {
		return nil
	}
	out := new(PowerCappingPolicyStatus)
	in.DeepCopyInto(out)
	return out
}
