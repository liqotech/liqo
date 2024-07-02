// Copyright 2019-2024 The Liqo Authors
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

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion is group version used to register these objects.
	GroupVersion = schema.GroupVersion{Group: "offloading.liqo.io", Version: "v1alpha1"}

	// NamespaceOffloadingResource is the resource name used to register the NamespaceOffloading CRD.
	NamespaceOffloadingResource = "namespaceoffloadings"

	// NamespaceOffloadingGroupResource is group and resource used to register these objects.
	NamespaceOffloadingGroupResource = schema.GroupResource{Group: GroupVersion.Group, Resource: NamespaceOffloadingResource}

	// NamespaceOffloadingGroupVersionResource is the group version resource used to register the NamespaceOffloading CRD.
	NamespaceOffloadingGroupVersionResource = GroupVersion.WithResource(NamespaceOffloadingResource)

	// QuotaResource is the resource name used to register the Quota CRD.
	QuotaResource = "quotas"

	// QuotaGroupResource is group and resource used to register these objects.
	QuotaGroupResource = schema.GroupResource{Group: GroupVersion.Group, Resource: QuotaResource}

	// QuotaGroupVersionResource is the group version resource used to register the Quota CRD.
	QuotaGroupVersionResource = GroupVersion.WithResource(QuotaResource)

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)
