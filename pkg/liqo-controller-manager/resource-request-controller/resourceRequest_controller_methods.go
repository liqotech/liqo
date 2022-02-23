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

package resourcerequestoperator

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
)

func (r *ResourceRequestReconciler) checkOfferState(ctx context.Context,
	resourceRequest *discoveryv1alpha1.ResourceRequest) error {

	var resourceOffer sharingv1alpha1.ResourceOffer
	err := r.Client.Get(ctx, types.NamespacedName{
		Name:      getOfferName(r.HomeCluster),
		Namespace: resourceRequest.GetNamespace(),
	}, &resourceOffer)
	if err != nil && !apierrors.IsNotFound(err) {
		klog.Error(err)
		return err
	}

	if apierrors.IsNotFound(err) {
		resourceRequest.Status.OfferState = discoveryv1alpha1.OfferStateNone
	} else {
		resourceRequest.Status.OfferState = discoveryv1alpha1.OfferStateCreated
	}

	return nil
}

// getOfferName returns the name of the ResourceOffer coming from the given cluster.
func getOfferName(cluster discoveryv1alpha1.ClusterIdentity) string {
	return cluster.ClusterName
}

// GetTenantName returns the name of the Tenant for the given cluster.
func GetTenantName(cluster discoveryv1alpha1.ClusterIdentity) string {
	return cluster.ClusterName
}
