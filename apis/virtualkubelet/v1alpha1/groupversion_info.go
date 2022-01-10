// Copyright 2019-2022 The Liqo Authors
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

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion is group version used to register these objects.
	SchemeGroupVersion = schema.GroupVersion{Group: "virtualkubelet.liqo.io", Version: "v1alpha1"}

	// NamespaceMapResource is the resource name used to register the NamespaceMap CRD.
	NamespaceMapResource = "namespacemaps"

	// NamespaceMapGroupResource is group resource used to register these objects.
	NamespaceMapGroupResource = schema.GroupResource{Group: SchemeGroupVersion.Group, Resource: NamespaceMapResource}

	// NamespaceMapGroupVersionResource is groupResourceVersion used to register these objects.
	NamespaceMapGroupVersionResource = SchemeGroupVersion.WithResource(NamespaceMapResource)

	// ShadowPodResource is the resource name used to register the ShadowPod CRD.
	ShadowPodResource = "shadowpods"

	// ShadowPodGroupResource is group resource used to register these objects.
	ShadowPodGroupResource = schema.GroupResource{Group: SchemeGroupVersion.Group, Resource: ShadowPodResource}

	// ShadowPodGroupVersionResource is groupResourceVersion used to register these objects.
	ShadowPodGroupVersionResource = SchemeGroupVersion.WithResource(ShadowPodResource)

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	SchemeBuilder = &scheme.Builder{GroupVersion: SchemeGroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

// Resource takes an unqualified resource and returns a Group qualified GroupResource.
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}
