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

// Package v1alpha1 contains API Schema definitions for the discovery v1alpha1 API group
// +kubebuilder:object:generate=true
// +groupName=discovery.liqo.io
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion is group version used to register these objects.
	GroupVersion = schema.GroupVersion{Group: "discovery.liqo.io", Version: "v1alpha1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme

	// ForeignClusterResource is the resource name used to register the ForeignCluster CRD.
	ForeignClusterResource = "foreignclusters"

	// ForeignClusterGroupVersionResource is the group version resource used to register the ForeignCluster CRD.
	ForeignClusterGroupVersionResource = GroupVersion.WithResource(ForeignClusterResource)

	// ForeignClusterGroupResource is the group resource used to register the ForeignCluster CRD.
	ForeignClusterGroupResource = schema.GroupResource{Group: GroupVersion.Group, Resource: ForeignClusterResource}

	// ResourceRequestResource is the resource name used to register the ResourceRequest CRD.
	ResourceRequestResource = "resourcerequests"

	// ResourceRequestGroupVersionResource is the group version resource used to register ResourceRequest CRD.
	ResourceRequestGroupVersionResource = GroupVersion.WithResource(ResourceRequestResource)

	// ResourceRequestGroupResource is the group resource used to register ResourceRequest CRD.
	ResourceRequestGroupResource = schema.GroupResource{Group: GroupVersion.Group, Resource: ResourceRequestResource}

	// SearchDomainGroupResource is the group resource used to register SearchDomain CRD.
	SearchDomainGroupResource = schema.GroupResource{Group: GroupVersion.Group, Resource: "searchdomains"}
)
