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

// Package resources contains information about the resources to replicate through the CRD replicator.
package resources

import (
	"k8s.io/apimachinery/pkg/runtime/schema"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
)

// Resource contains a list of resources identified by their GVR.
type Resource struct {
	// GroupVersionResource contains the GVR of the resource to replicate.
	GroupVersionResource schema.GroupVersionResource
	// Ownership indicates the ownership over this resource.
	Ownership consts.OwnershipType
}

// GetResourcesToReplicate returns the list of resources to be replicated through the CRD replicator.
func GetResourcesToReplicate() []Resource {
	return []Resource{
		{
			GroupVersionResource: offloadingv1beta1.NamespaceMapGroupVersionResource,
			Ownership:            consts.OwnershipShared,
		},
		{
			GroupVersionResource: authv1beta1.ResourceSliceGroupVersionResource,
			Ownership:            consts.OwnershipShared,
		},
		{
			GroupVersionResource: authv1beta1.RenewGroupVersionResource,
			Ownership:            consts.OwnershipShared,
		},
	}
}
