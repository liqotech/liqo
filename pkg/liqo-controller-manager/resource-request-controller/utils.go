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
	"errors"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	"github.com/liqotech/liqo/pkg/discovery"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreignCluster"
)

// ensureForeignCluster ensures the ForeignCluster existence, if not exists we have to add a new one
// with IncomingPeering discovery method.
func (r *ResourceRequestReconciler) ensureForeignCluster(ctx context.Context,
	resourceRequest *discoveryv1alpha1.ResourceRequest) (*discoveryv1alpha1.ForeignCluster, error) {
	remoteClusterID := resourceRequest.Spec.ClusterIdentity.ClusterID

	// Check if a foreignCluster with the desired ClusterID already exists.
	if foreignCluster, err := foreignclusterutils.GetForeignClusterByID(ctx, r.Client, remoteClusterID); err == nil {
		// A valid ForeignCluster already exists.
		return foreignCluster, nil
	} else if !apierrors.IsNotFound(err) {
		// Something went wrong while retrieving the foreign cluster.
		return nil, err
	}

	// If the resource request had already been withdrawn by the local cluster, avoid creating a new foreign cluster.
	if !resourceRequest.Status.OfferWithdrawalTimestamp.IsZero() {
		return nil, errors.New("the resource request has already been withdrawn")
	}

	// Otherwise, create a new ForeignCluster
	rrIdentity := resourceRequest.Spec.ClusterIdentity
	cluster, err := r.createForeignCluster(ctx, rrIdentity, resourceRequest.Spec.AuthURL)
	if apierrors.IsAlreadyExists(err) {
		newIdentity := discoveryv1alpha1.ClusterIdentity{
			ClusterID:   rrIdentity.ClusterID,
			ClusterName: foreignclusterutils.UniqueName(&rrIdentity),
		}
		klog.Warningf("Cluster name %s is already taken: retrying with %s",
			rrIdentity.ClusterName, newIdentity.ClusterName)
		cluster, err = r.createForeignCluster(ctx, newIdentity, resourceRequest.Spec.AuthURL)
	}
	return cluster, err
}

// ensureControllerReference ensures that the ForeignCluster is the owner of the ResourceRequest, to make it able
// to correctly monitor the incoming peering status.
func (r *ResourceRequestReconciler) ensureControllerReference(foreignCluster *discoveryv1alpha1.ForeignCluster,
	resourceRequest *discoveryv1alpha1.ResourceRequest) (requireSpecUpdate bool, err error) {
	if metav1.GetControllerOfNoCopy(resourceRequest) != nil {
		return false, nil
	}

	return true, controllerutil.SetControllerReference(foreignCluster, resourceRequest, r.Scheme)
}

// createForeignCluster creates a foreign cluster.
func (r *ResourceRequestReconciler) createForeignCluster(ctx context.Context,
	identity discoveryv1alpha1.ClusterIdentity, authURL string) (*discoveryv1alpha1.ForeignCluster, error) {
	foreignCluster := &discoveryv1alpha1.ForeignCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: identity.ClusterName,
			Labels: map[string]string{
				discovery.DiscoveryTypeLabel: string(discovery.IncomingPeeringDiscovery),
				discovery.ClusterIDLabel:     identity.ClusterID,
			},
		},
		Spec: discoveryv1alpha1.ForeignClusterSpec{
			ClusterIdentity:        identity,
			OutgoingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
			IncomingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
			ForeignAuthURL:         authURL,
			InsecureSkipTLSVerify:  pointer.BoolPtr(true),
		},
	}

	if err := r.Client.Create(ctx, foreignCluster); err != nil {
		klog.Errorf("%s -> unable to Create foreignCluster: %s", identity.ClusterName, err)
		return nil, err
	}

	klog.Infof("%s -> Created ForeignCluster %s with IncomingPeering discovery type",
		identity.ClusterName, foreignCluster.Name)
	return foreignCluster, nil
}

func (r *ResourceRequestReconciler) invalidateResourceOffer(ctx context.Context, request *discoveryv1alpha1.ResourceRequest) error {
	var offer sharingv1alpha1.ResourceOffer
	err := r.Client.Get(ctx, types.NamespacedName{
		Namespace: request.GetNamespace(),
		Name:      getOfferName(r.HomeCluster),
	}, &offer)
	if apierrors.IsNotFound(err) {
		// ignore not found errors
		return nil
	}
	if err != nil {
		return err
	}

	switch offer.Status.VirtualKubeletStatus {
	case sharingv1alpha1.VirtualKubeletStatusDeleting, sharingv1alpha1.VirtualKubeletStatusCreated:
		if offer.Spec.WithdrawalTimestamp.IsZero() {
			now := metav1.Now()
			offer.Spec.WithdrawalTimestamp = &now
		}
		err = client.IgnoreNotFound(r.Client.Update(ctx, &offer))
		if err != nil {
			return err
		}
		klog.Infof("%s -> Offer: %s/%s", r.HomeCluster.ClusterName, offer.Namespace, offer.Name)
		return nil
	case sharingv1alpha1.VirtualKubeletStatusNone:
		err = client.IgnoreNotFound(r.Client.Delete(ctx, &offer))
		if err != nil {
			return err
		}
		if request.Status.OfferWithdrawalTimestamp.IsZero() {
			now := metav1.Now()
			request.Status.OfferWithdrawalTimestamp = &now
		}
		klog.Infof("%s -> Deleted Offer: %s/%s", r.HomeCluster.ClusterName, offer.Namespace, offer.Name)
		return nil
	default:
		return fmt.Errorf("unknown VirtualKubeletStatus %v", offer.Status.VirtualKubeletStatus)
	}
}
