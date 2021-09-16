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

package resourcerequestoperator

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	crdreplicator "github.com/liqotech/liqo/internal/crdReplicator"
)

// ResourceRequestReconciler reconciles a ResourceRequest object.
type ResourceRequestReconciler struct {
	client.Client
	Scheme                *runtime.Scheme
	ClusterID             string
	Broadcaster           *Broadcaster
	EnableIncomingPeering bool
}

const (
	offerPrefix = "resourceoffer-"
)

// +kubebuilder:rbac:groups=sharing.liqo.io,resources=resourceoffers,verbs=get;list;watch;create;update;patch;
// +kubebuilder:rbac:groups=sharing.liqo.io,resources=resourceoffers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=resourcerequests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=resourcerequests/status;resourcerequests/finalizers,verbs=get;update;patch
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters/status;foreignclusters/finalizers,verbs=get;update;patch

// +kubebuilder:rbac:groups=capsule.clastix.io,resources=tenants,verbs=get;list;watch;create;update;patch;delete;

// Reconcile is the main function of the controller which reconciles ResourceRequest resources.
func (r *ResourceRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	var resourceRequest discoveryv1alpha1.ResourceRequest
	err = r.Get(ctx, req.NamespacedName, &resourceRequest)
	if err != nil {
		klog.Errorf("unable to get resourceRequest %s: %s", req.NamespacedName, err)
		return ctrl.Result{}, nil
	}

	remoteClusterID := resourceRequest.Spec.ClusterIdentity.ClusterID

	// ensure the ForeignCluster existence, if not exists we have to add a new one
	// with IncomingPeering discovery method.
	foreignCluster, err := r.ensureForeignCluster(ctx, &resourceRequest)
	if err != nil {
		klog.Errorf("%s -> Error generating resourceOffer: %s", remoteClusterID, err)
		return ctrl.Result{}, err
	}

	// ensure that the ResourceRequest is controlled by a ForeignCluster
	requireSpecUpdate, err := r.ensureControllerReference(foreignCluster, &resourceRequest)
	if err != nil {
		klog.Errorf("%s -> Error ensuring the controller reference presence: %s", remoteClusterID, err)
		return ctrl.Result{}, err
	}

	var resourceReqPhase resourceRequestPhase
	resourceReqPhase, err = r.getResourceRequestPhase(foreignCluster, &resourceRequest)
	if err != nil {
		klog.Errorf("%s -> Error getting the ResourceRequest Phase: %s", remoteClusterID, err)
		return ctrl.Result{}, err
	}

	newRequireSpecUpdate := false
	// ensure creation and deletion of the Capsule Tenant for the remote cluster
	switch resourceReqPhase {
	case deletingResourceRequestPhase, denyResourceRequestPhase:
		// the local cluster does not allow the peering, ensure the Tenant deletion
		if newRequireSpecUpdate, err = r.ensureTenantDeletion(ctx, &resourceRequest); err != nil {
			klog.Errorf("%s -> Error deleting Tenant: %s", remoteClusterID, err)
			return ctrl.Result{}, err
		}
	case allowResourceRequestPhase:
		// the local cluster allows the peering, ensure the Tenant creation
		if newRequireSpecUpdate, err = r.ensureTenant(ctx, &resourceRequest); err != nil {
			klog.Errorf("%s -> Error creating Tenant: %s", remoteClusterID, err)
			return ctrl.Result{}, err
		}
	}
	requireSpecUpdate = requireSpecUpdate || newRequireSpecUpdate

	if requireSpecUpdate {
		if err = r.Client.Update(ctx, &resourceRequest); err != nil {
			klog.Error(err)
			return ctrl.Result{}, err
		}
		// always requeue after a spec update
		return ctrl.Result{}, err
	}

	defer func() {
		newErr := r.Client.Status().Update(ctx, &resourceRequest)
		if newErr != nil {
			klog.Error(newErr)
			err = newErr
		}
	}()

	// ensure creation, update and deletion of the related ResourceOffer
	switch resourceReqPhase {
	case allowResourceRequestPhase:
		// ensure that we are offering resources to this remote cluster
		r.Broadcaster.enqueueForCreationOrUpdate(remoteClusterID)
		resourceRequest.Status.OfferWithdrawalTimestamp = nil
	case denyResourceRequestPhase, deletingResourceRequestPhase:
		// ensure to invalidate any resource offered to the remote cluster
		err = r.invalidateResourceOffer(ctx, &resourceRequest)
		if err != nil {
			klog.Errorf("%s -> Error invalidating resourceOffer: %s", remoteClusterID, err)
			return ctrl.Result{}, err
		}
	}

	// check the state of the related ResourceOffer
	if err = r.checkOfferState(ctx, &resourceRequest); err != nil {
		klog.Error(err)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager is the setup function of the controller.
func (r *ResourceRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// generate the predicate to filter just the ResourceRequest created by the remote cluster checking crdReplicator labels
	p, err := predicate.LabelSelectorPredicate(crdreplicator.ReplicatedResourcesLabelSelector)
	if err != nil {
		klog.Error(err)
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&discoveryv1alpha1.ResourceRequest{}, builder.WithPredicates(p)).
		Owns(&sharingv1alpha1.ResourceOffer{}).
		Watches(&source.Kind{Type: &discoveryv1alpha1.ForeignCluster{}}, getForeignClusterEventHandler(
			r.Client,
		)).
		Complete(r)
}
