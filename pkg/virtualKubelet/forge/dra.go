// Copyright 2019-2026 The Liqo Authors
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

package forge

import (
	corev1 "k8s.io/api/core/v1"
	resourcev1 "k8s.io/api/resource/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/ptr"
)

// LocalResourceSlice builds the local ResourceSlice from the given remote slice,
// setting its OwnerReference to the local virtual node. The caller is responsible for
// having checked that remote.Spec.NodeName is set and matches a local node.
// Important: this function is based on the assumption that the local node name is the same as the remote one.
func LocalResourceSlice(
	remote *resourcev1.ResourceSlice,
	localNode *corev1.Node,
	labelsNotReflected, annotationsNotReflected []string,
) *resourcev1.ResourceSlice {
	return &resourcev1.ResourceSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name: remote.Name,
			Labels: labels.Merge(
				FilterNotReflected(remote.GetLabels(), labelsNotReflected),
				ReflectionLabels(),
			),
			Annotations: FilterNotReflected(remote.GetAnnotations(), annotationsNotReflected),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "v1",
					Kind:               "Node",
					Name:               localNode.Name,
					UID:                localNode.UID,
					Controller:         ptr.To(true),
					BlockOwnerDeletion: ptr.To(false),
				},
			},
		},
		Spec: *remote.Spec.DeepCopy(),
	}
}

// RemoteResourceClaim builds a remote ResourceClaim from a local one, dropping
// status and server-managed metadata.
func RemoteResourceClaim(local *resourcev1.ResourceClaim, remoteNamespace string,
	labelsNotReflected, annotationsNotReflected []string) *resourcev1.ResourceClaim {
	return &resourcev1.ResourceClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      local.Name,
			Namespace: remoteNamespace,
			Labels: labels.Merge(
				FilterNotReflected(local.GetLabels(), labelsNotReflected),
				ReflectionLabels(),
			),
			Annotations: FilterNotReflected(local.GetAnnotations(), annotationsNotReflected),
		},
		Spec: *local.Spec.DeepCopy(),
	}
}

// RemoteDeviceClass builds a remote DeviceClass from a local one. DeviceClass is
// cluster-scoped; we keep the same name and copy the spec verbatim.
func RemoteDeviceClass(local *resourcev1.DeviceClass, labelsNotReflected, annotationsNotReflected []string) *resourcev1.DeviceClass {
	return &resourcev1.DeviceClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: local.Name,
			Labels: labels.Merge(
				FilterNotReflected(local.GetLabels(), labelsNotReflected),
				ReflectionLabels(),
			),
			Annotations: FilterNotReflected(local.GetAnnotations(), annotationsNotReflected),
		},
		Spec: *local.Spec.DeepCopy(),
	}
}

// ReferencedDeviceClasses extracts the set of DeviceClass names referenced by a
// ResourceClaim, walking both Exactly and FirstAvailable sub-requests.
func ReferencedDeviceClasses(claim *resourcev1.ResourceClaim) []string {
	seen := map[string]struct{}{}
	out := []string{}
	add := func(name string) {
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	for i := range claim.Spec.Devices.Requests {
		req := &claim.Spec.Devices.Requests[i]
		if req.Exactly != nil {
			add(req.Exactly.DeviceClassName)
		}
		for j := range req.FirstAvailable {
			add(req.FirstAvailable[j].DeviceClassName)
		}
	}
	return out
}
