// Copyright 2019-2025 The Liqo Authors
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	corev1apply "k8s.io/client-go/applyconfigurations/core/v1"

	"github.com/liqotech/liqo/pkg/utils/maps"
)

const (
	// LiqoOriginClusterIDKey is the key of a label identifying the origin cluster of a reflected resource.
	LiqoOriginClusterIDKey = "offloading.liqo.io/origin"
	// LiqoDestinationClusterIDKey is the key of a label identifying the destination cluster of a reflected resource.
	LiqoDestinationClusterIDKey = "offloading.liqo.io/destination"
	// LiqoOriginClusterNodeName is the name of the node on the origin cluster referenced by the virtual-kubelet.
	LiqoOriginClusterNodeName = "offloading.liqo.io/nodename"
)

// ReflectionLabels returns the labels assigned to the objects reflected from the local to the remote cluster.
func ReflectionLabels() labels.Set {
	return map[string]string{
		LiqoOriginClusterIDKey:      string(LocalCluster),
		LiqoDestinationClusterIDKey: string(RemoteCluster),
	}
}

// ReflectionLabelsWithNodeName returns the labels assigned to the objects reflected from the local to the remote cluster with the given node name.
func ReflectionLabelsWithNodeName(nodeName string) labels.Set {
	l := ReflectionLabels()
	l[LiqoOriginClusterNodeName] = nodeName
	return l
}

// ReflectedLabelSelector returns a label selector matching the objects reflected from the local to the remote cluster.
func ReflectedLabelSelector() labels.Selector {
	return ReflectionLabels().AsSelectorPreValidated()
}

// IsReflected returns whether the current object has been reflected from the local to the remote cluster.
func IsReflected(obj metav1.Object) bool {
	return ReflectedLabelSelector().Matches(labels.Set(obj.GetLabels()))
}

// RemoteObjectMeta forges the local ObjectMeta for a reflected object.
func RemoteObjectMeta(local, remote *metav1.ObjectMeta) metav1.ObjectMeta {
	output := remote.DeepCopy()
	output.SetLabels(labels.Merge(local.GetLabels(), ReflectionLabels()))
	output.SetAnnotations(local.GetAnnotations())
	return *output
}

// RemoteObjectReference forges the apply patch for a reflected RemoteObjectReference.
func RemoteObjectReference(ref *corev1.ObjectReference) *corev1apply.ObjectReferenceApplyConfiguration {
	if ref == nil {
		return nil
	}

	return corev1apply.ObjectReference().
		WithAPIVersion(ref.APIVersion).WithFieldPath(ref.FieldPath).
		WithKind(RemoteKind(ref.Kind)).WithName(ref.Name).WithNamespace(ref.Namespace).
		WithResourceVersion(ref.ResourceVersion).WithUID(ref.UID)
}

// RemoteTypedLocalObjectReference forges the apply patch for a reflected TypedLocalObjectReference.
func RemoteTypedLocalObjectReference(local *corev1.TypedLocalObjectReference) *corev1apply.TypedLocalObjectReferenceApplyConfiguration {
	if local == nil {
		return nil
	}
	res := corev1apply.TypedLocalObjectReference().
		WithKind(local.Kind).
		WithName(local.Name)
	res.APIGroup = local.APIGroup
	return res
}

// RemoteKind prepends "Remote" to a kind name, to identify remote objects.
func RemoteKind(kind string) string {
	return "Remote" + kind
}

// FilterNotReflected returns a map filtering out entries that match the given keys.
func FilterNotReflected(m map[string]string, blackListedKeys []string) map[string]string {
	filter := maps.FilterBlacklist(blackListedKeys...)
	return maps.Filter(m, filter)
}
