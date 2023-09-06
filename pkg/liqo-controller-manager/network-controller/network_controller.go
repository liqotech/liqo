// Copyright 2019-2023 The Liqo Authors
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

package networkctrl

import (
	"context"
	"fmt"
	"net"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqonet/ipam"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreignCluster"
)

const (
	ipamNetworkFinalizer = "network.ipam.liqo.io"
)

// NetworkReconciler reconciles a Network object.
type NetworkReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	IpamClient ipam.IpamClient
}

// +kubebuilder:rbac:groups=ipam.liqo.io,resources=networks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ipam.liqo.io,resources=networks/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ipam.liqo.io,resources=networks/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters,verbs=get;list;watch

// Reconcile Network objects.
func (r *NetworkReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	klog.Infof("Reconcilg Network %q", req.NamespacedName) // TODO:: delete
	var nw ipamv1alpha1.Network
	var desiredCIDR, remappedCIDR string

	// Fetch the Network instance
	if err := r.Get(ctx, req.NamespacedName, &nw); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("Network %q not found", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		klog.Errorf("an error occurred while getting Network %q: %v", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	// Retrieve the remote cluster ID from the labels.
	remoteclusterID, found := nw.Labels[consts.RemoteClusterID] // it should always be present thanks to validating webhook
	if !found {
		err := fmt.Errorf("missing label %q on Network %q (webhook disabled or misconfigured)", consts.RemoteClusterID, req.NamespacedName)
		klog.Error(err)
		return ctrl.Result{}, err
	}

	_, err := foreignclusterutils.CheckForeignClusterExistence(ctx, r.Client, remoteclusterID)
	if err != nil {
		return ctrl.Result{}, err
	}

	desiredCIDR = nw.Spec.CIDR

	if nw.GetDeletionTimestamp().IsZero() {
		if !controllerutil.ContainsFinalizer(&nw, ipamNetworkFinalizer) {
			// Add finalizer to prevent deletion without unmapping the Network.
			controllerutil.AddFinalizer(&nw, ipamNetworkFinalizer)

			// Update the Network object
			if err := r.Update(ctx, &nw); err != nil {
				klog.Errorf("error while adding finalizers to Network %q: %v", req.NamespacedName, err)
				return ctrl.Result{}, err
			}
			klog.Infof("finalizer %q correctly added to Network %q", ipamNetworkFinalizer, req.NamespacedName)

			// We return immediately and wait for the next reconcile to eventually update the status.
			return ctrl.Result{}, nil
		}

		// Update Network status if it is not set yet
		// The IPAM MapNetworkCIDR() function is not idempotent, so we avoid to call it
		// multiple times by checking if the status is already set.
		if nw.Status.CIDR == "" {
			remappedCIDR, err = getRemappedCIDR(ctx, r.IpamClient, desiredCIDR)
			if err != nil {
				return ctrl.Result{}, err
			}

			// Update status
			nw.Status.CIDR = remappedCIDR
			if err := r.Client.Status().Update(ctx, &nw); err != nil {
				klog.Errorf("error while updating Network %q status: %v", req.NamespacedName, err)
				return ctrl.Result{}, err
			}
			klog.Infof("updated Network %q status (spec: %s -> status: %s)", req.NamespacedName, desiredCIDR, remappedCIDR)
		}
	} else if controllerutil.ContainsFinalizer(&nw, ipamNetworkFinalizer) {
		// The resource is being deleted and the finalizer is still present. Call the IPAM to unmap the network CIDR.
		remappedCIDR = nw.Status.CIDR

		if _, _, err := net.ParseCIDR(remappedCIDR); err != nil {
			klog.Errorf("Unable to unmap CIDR %s of Network %q (inavlid format): %v", remappedCIDR, req.NamespacedName, err)
			return ctrl.Result{}, err
		}

		if err := deleteRemappedCIDR(ctx, r.IpamClient, remappedCIDR); err != nil {
			return ctrl.Result{}, err
		}

		// Remove status and finalizer, and update the object.
		nw.Status.CIDR = ""
		controllerutil.RemoveFinalizer(&nw, ipamNetworkFinalizer)

		if err := r.Update(ctx, &nw); err != nil {
			klog.Errorf("error while removing finalizer from Network %q: %v", req.NamespacedName, err)
			return ctrl.Result{}, err
		}
		klog.Infof("finalizer correctly removed from Network %q", req.NamespacedName)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager monitors Network resources.
func (r *NetworkReconciler) SetupWithManager(mgr ctrl.Manager, workers int) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ipamv1alpha1.Network{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: workers}).
		Complete(r)
}

// getRemappedCIDR returns the remapped CIDR for the given CIDR and remote clusterID.
func getRemappedCIDR(ctx context.Context, ipamClient ipam.IpamClient, desiredCIDR string) (string, error) {
	switch ipamClient.(type) {
	case nil:
		// IPAM is not enabled, use original CIDR from spec
		return desiredCIDR, nil
	default:
		// interact with the IPAM to retrieve the correct mapping.
		response, err := ipamClient.MapNetworkCIDR(ctx, &ipam.MapCIDRRequest{Cidr: desiredCIDR})
		if err != nil {
			klog.Errorf("IPAM: error while mapping network CIDR %s: %v", desiredCIDR, err)
			return "", err
		}
		klog.Infof("IPAM: mapped network CIDR %s to %s", desiredCIDR, response.Cidr)
		return response.Cidr, nil
	}
}

// deleteRemappedCIDR unmaps the CIDR for the given remote clusterID.
func deleteRemappedCIDR(ctx context.Context, ipamClient ipam.IpamClient, remappedCIDR string) error {
	switch ipamClient.(type) {
	case nil:
		// If the IPAM is not enabled we do not need to free the network CIDR.
		return nil
	default:
		// Interact with the IPAM to free the network CIDR.
		_, err := ipamClient.UnmapNetworkCIDR(ctx, &ipam.UnmapCIDRRequest{Cidr: remappedCIDR})
		if err != nil {
			klog.Errorf("IPAM: error while unmapping CIDR %s: %v", remappedCIDR, err)
			return err
		}
		klog.Infof("IPAM: unmapped CIDR %s", remappedCIDR)
		return nil
	}
}
