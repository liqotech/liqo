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
// +groupName=core.liqo.io

package v1beta1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion is group version used to register these objects.
	GroupVersion = schema.GroupVersion{Group: "core.liqo.io", Version: "v1beta1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme

	// ForeignClusterKind is the resource name used to register the ForeignCluster CRD.
	ForeignClusterKind = "ForeignCluster"

	// ForeignClusterResource is the resource name used to register the ForeignCluster CRD.
	ForeignClusterResource = "foreignclusters"

	// ForeignClusterGroupVersionResource is the group version resource used to register the ForeignCluster CRD.
	ForeignClusterGroupVersionResource = GroupVersion.WithResource(ForeignClusterResource)

	// ForeignClusterGroupResource is the group resource used to register the ForeignCluster CRD.
	ForeignClusterGroupResource = schema.GroupResource{Group: GroupVersion.Group, Resource: ForeignClusterResource}
)
