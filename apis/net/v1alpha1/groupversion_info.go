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

// Package v1alpha1 contains API Schema definitions for the liqonetliqoio v1alpha1 API group
// +kubebuilder:object:generate=true
// +groupName=net.liqo.io
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion is group version used to register these objects.
	GroupVersion = schema.GroupVersion{Group: "net.liqo.io", Version: "v1alpha1"}

	// TunnelEndpointGroupResource is group resource used to register tunnel endpoints.
	TunnelEndpointGroupResource = schema.GroupResource{Group: GroupVersion.Group,
		Resource: "tunnelendpoints"}

	// TunnelEndpointGroupVersionResource is group resource version used to tunnelEndpoint objects.
	TunnelEndpointGroupVersionResource = schema.GroupVersionResource{Group: GroupVersion.Group,
		Version:  GroupVersion.Version,
		Resource: "tunnelendpoints"}

	// NetworkConfigGroupVersionResource is group resource version used to networkConfig objects.
	NetworkConfigGroupVersionResource = schema.GroupVersionResource{Group: GroupVersion.Group,
		Version:  GroupVersion.Version,
		Resource: "networkconfigs"}

	// IpamGroupResource is group resource used to register ipamstorages.
	IpamGroupResource = schema.GroupVersionResource{Group: GroupVersion.Group, Version: GroupVersion.Version,
		Resource: "ipamstorages"}

	// NatMappingGroupResource is group resource used to register natmappings.
	NatMappingGroupResource = schema.GroupVersionResource{Group: GroupVersion.Group, Version: GroupVersion.Version,
		Resource: "natmappings"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)
