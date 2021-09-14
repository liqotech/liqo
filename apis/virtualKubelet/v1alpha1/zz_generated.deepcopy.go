//go:build !ignore_autogenerated
// +build !ignore_autogenerated

// Copyright 2019-2021 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NamespaceMap) DeepCopyInto(out *NamespaceMap) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NamespaceMap.
func (in *NamespaceMap) DeepCopy() *NamespaceMap {
	if in == nil {
		return nil
	}
	out := new(NamespaceMap)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NamespaceMap) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NamespaceMapList) DeepCopyInto(out *NamespaceMapList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]NamespaceMap, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NamespaceMapList.
func (in *NamespaceMapList) DeepCopy() *NamespaceMapList {
	if in == nil {
		return nil
	}
	out := new(NamespaceMapList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NamespaceMapList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NamespaceMapSpec) DeepCopyInto(out *NamespaceMapSpec) {
	*out = *in
	if in.DesiredMapping != nil {
		in, out := &in.DesiredMapping, &out.DesiredMapping
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NamespaceMapSpec.
func (in *NamespaceMapSpec) DeepCopy() *NamespaceMapSpec {
	if in == nil {
		return nil
	}
	out := new(NamespaceMapSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NamespaceMapStatus) DeepCopyInto(out *NamespaceMapStatus) {
	*out = *in
	if in.CurrentMapping != nil {
		in, out := &in.CurrentMapping, &out.CurrentMapping
		*out = make(map[string]RemoteNamespaceStatus, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NamespaceMapStatus.
func (in *NamespaceMapStatus) DeepCopy() *NamespaceMapStatus {
	if in == nil {
		return nil
	}
	out := new(NamespaceMapStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RemoteNamespaceStatus) DeepCopyInto(out *RemoteNamespaceStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RemoteNamespaceStatus.
func (in *RemoteNamespaceStatus) DeepCopy() *RemoteNamespaceStatus {
	if in == nil {
		return nil
	}
	out := new(RemoteNamespaceStatus)
	in.DeepCopyInto(out)
	return out
}
