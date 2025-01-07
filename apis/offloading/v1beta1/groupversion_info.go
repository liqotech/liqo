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

// +kubebuilder:object:generate=true
// +groupName=offloading.liqo.io

package v1beta1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// SchemeGroupVersion is group version used to register these objects.
	SchemeGroupVersion = schema.GroupVersion{Group: "offloading.liqo.io", Version: "v1beta1"}

	// NamespaceOffloadingResource is the resource name used to register the NamespaceOffloading CRD.
	NamespaceOffloadingResource = "namespaceoffloadings"

	// NamespaceOffloadingGroupResource is group and resource used to register these objects.
	NamespaceOffloadingGroupResource = schema.GroupResource{Group: SchemeGroupVersion.Group, Resource: NamespaceOffloadingResource}

	// NamespaceOffloadingGroupVersionResource is the group version resource used to register the NamespaceOffloading CRD.
	NamespaceOffloadingGroupVersionResource = SchemeGroupVersion.WithResource(NamespaceOffloadingResource)

	// QuotaResource is the resource name used to register the Quota CRD.
	QuotaResource = "quotas"

	// QuotaGroupResource is group and resource used to register these objects.
	QuotaGroupResource = schema.GroupResource{Group: SchemeGroupVersion.Group, Resource: QuotaResource}

	// QuotaGroupVersionResource is the group version resource used to register the Quota CRD.
	QuotaGroupVersionResource = SchemeGroupVersion.WithResource(QuotaResource)

	// NamespaceMapResource is the resource name used to register the NamespaceMap CRD.
	NamespaceMapResource = "namespacemaps"

	// NamespaceMapGroupResource is group resource used to register these objects.
	NamespaceMapGroupResource = schema.GroupResource{Group: SchemeGroupVersion.Group, Resource: NamespaceMapResource}

	// NamespaceMapGroupVersionResource is groupResourceVersion used to register these objects.
	NamespaceMapGroupVersionResource = SchemeGroupVersion.WithResource(NamespaceMapResource)

	// VirtualNodeKind is the kind name used to register the VirtualNode CRD.
	VirtualNodeKind = "VirtualNode"

	// VirtualNodeResource is the resource name used to register the VirtualNode CRD.
	VirtualNodeResource = "virtualnodes"

	// VirtualNodeGroupResource is group resource used to register these objects.
	VirtualNodeGroupResource = schema.GroupResource{Group: SchemeGroupVersion.Group, Resource: VirtualNodeResource}

	// VirtualNodeGroupVersionResource is groupResourceVersion used to register these objects.
	VirtualNodeGroupVersionResource = SchemeGroupVersion.WithResource(VirtualNodeResource)

	// ShadowPodResource is the resource name used to register the ShadowPod CRD.
	ShadowPodResource = "shadowpods"

	// ShadowPodGroupResource is group resource used to register these objects.
	ShadowPodGroupResource = schema.GroupResource{Group: SchemeGroupVersion.Group, Resource: ShadowPodResource}

	// ShadowPodGroupVersionResource is groupResourceVersion used to register these objects.
	ShadowPodGroupVersionResource = SchemeGroupVersion.WithResource(ShadowPodResource)

	// ShadowEndpointSliceResource is the resource name used to register the ShadowEndpointSlice CRD.
	ShadowEndpointSliceResource = "shadowendpointslices"

	// ShadowEndpointSliceGroupResource is group resource used to register these objects.
	ShadowEndpointSliceGroupResource = schema.GroupResource{Group: SchemeGroupVersion.Group, Resource: ShadowEndpointSliceResource}

	// ShadowEndpointSliceGroupVersionResource is groupResourceVersion used to register these objects.
	ShadowEndpointSliceGroupVersionResource = SchemeGroupVersion.WithResource(ShadowEndpointSliceResource)

	// VkOptionsTemplateResource is the resource name used to register the VkOptionsTemplate CRD.
	VkOptionsTemplateResource = "vkoptionstemplates"

	// VkOptionsTemplateGroupResource is group resource used to register these objects.
	VkOptionsTemplateGroupResource = schema.GroupResource{Group: SchemeGroupVersion.Group, Resource: VkOptionsTemplateResource}

	// VkOptionsTemplateGroupVersionResource is groupResourceVersion used to register these objects.
	VkOptionsTemplateGroupVersionResource = SchemeGroupVersion.WithResource(VkOptionsTemplateResource)

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	SchemeBuilder = &scheme.Builder{GroupVersion: SchemeGroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

// Resource takes an unqualified resource and returns a Group qualified GroupResource.
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}
