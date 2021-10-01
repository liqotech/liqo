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

package forge

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	// LiqoOutgoingKey is a label to set on all offloaded resources (deprecated).
	LiqoOutgoingKey = "virtualkubelet.liqo.io/outgoing"
	// LiqoOriginClusterIDKey is the key of a label identifying the origin cluster of a reflected resource.
	LiqoOriginClusterIDKey = "virtualkubelet.liqo.io/origin"
	// LiqoDestinationClusterIDKey is the key of a label identifying the destination cluster of a reflected resource.
	LiqoDestinationClusterIDKey = "virtualkubelet.liqo.io/destination"
)

var (
	LiqoNodeName = func() string {
		if forger.virtualNodeName == nil {
			return ""
		}
		return forger.virtualNodeName.Value().ToString()
	}
)

// ReflectionLabels returns the labels assigned to the objects reflected from the local to the remote cluster.
func ReflectionLabels() labels.Set {
	return map[string]string{
		LiqoOriginClusterIDKey:      LocalClusterID,
		LiqoDestinationClusterIDKey: RemoteClusterID,
	}
}

// ReflectedLabelSelector returns a label selector matching the objects reflected from the local to the remote cluster.
func ReflectedLabelSelector() labels.Selector {
	return ReflectionLabels().AsSelectorPreValidated()
}

// IsReflected returns whether the current object has been reflected from the local to the remote cluster.
func IsReflected(obj metav1.Object) bool {
	return ReflectedLabelSelector().Matches(labels.Set(obj.GetLabels()))
}

func (f *apiForger) forgeForeignMeta(homeMeta, foreignMeta *metav1.ObjectMeta, foreignNamespace, reflectionType string) {
	forgeObjectMeta(homeMeta, foreignMeta)

	foreignMeta.Namespace = foreignNamespace
	foreignMeta.Labels[LiqoOriginClusterIDKey] = LocalClusterID
	foreignMeta.Labels[reflectionType] = LiqoNodeName()
}

func (f *apiForger) forgeHomeMeta(foreignMeta, homeMeta *metav1.ObjectMeta, homeNamespace, reflectionType string) {
	forgeObjectMeta(foreignMeta, homeMeta)

	homeMeta.Namespace = homeNamespace
	homeMeta.Labels[reflectionType] = LiqoNodeName()
}

func forgeObjectMeta(inMeta, outMeta *metav1.ObjectMeta) {
	outMeta.Name = inMeta.Name

	if outMeta.Annotations == nil {
		outMeta.Annotations = make(map[string]string)
	}
	for k, v := range inMeta.Annotations {
		outMeta.Annotations[k] = v
	}

	if outMeta.Labels == nil {
		outMeta.Labels = make(map[string]string)
	}
	for k, v := range inMeta.Labels {
		outMeta.Labels[k] = v
	}
}
