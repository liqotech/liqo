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

// Package v1alpha1 contains API Schema definitions for the policy v1 API group
// +kubebuilder:object:generate=true
// +groupName=config.liqo.io
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion is group version used to register objects.
	GroupVersion = schema.GroupVersion{Group: "config.liqo.io", Version: "v1alpha1"}

	// ClusterConfigGroupResource is group resource used to register objects.
	ClusterConfigGroupResource = schema.GroupResource{Group: GroupVersion.Group, Resource: "clusterconfigs"}

	// ClusterConfigGroupVersionResource is group resource version used to register objects.
	ClusterConfigGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupVersion.Group,
		Resource: "clusterconfigs",
		Version:  GroupVersion.Version,
	}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)
