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

package networkctrl

import (
	"context"
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
	"github.com/liqotech/liqo/pkg/ipam"
	"github.com/liqotech/liqo/pkg/utils"
	ipamutils "github.com/liqotech/liqo/pkg/utils/ipam"
)

const (
	ipamNetworkFinalizer = "network.ipam.liqo.io/finalizer"
)

// NetworkReconciler reconciles a Network object.
type NetworkReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	ipamClient ipam.IPAMClient
}

// NewNetworkReconciler returns a new NetworkReconciler.
func NewNetworkReconciler(cl client.Client, s *runtime.Scheme, ipamClient ipam.IPAMClient) *NetworkReconciler {
	return &NetworkReconciler{
		Client: cl,
		Scheme: s,

		ipamClient: ipamClient,
	}
}

// +kubebuilder:rbac:groups=ipam.liqo.io,resources=networks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ipam.liqo.io,resources=networks/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ipam.liqo.io,resources=networks/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.liqo.io,resources=foreignclusters,verbs=get;list;watch

// Reconcile Network objects.
func (r *NetworkReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Fetch the Network instance
	var nw ipamv1alpha1.Network
	if err := r.Get(ctx, req.NamespacedName, &nw); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("Network %q not found", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		klog.Errorf("an error occurred while getting Network %q: %v", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	if err := r.handleNetworkStatus(ctx, &nw); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager monitors Network resources.
func (r *NetworkReconciler) SetupWithManager(mgr ctrl.Manager, workers int) error {
	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlNetwork).
		For(&ipamv1alpha1.Network{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: workers}).
		Complete(r)
}

// updateNetworkStatus updates the status of the given Network resource.
func (r *NetworkReconciler) updateNetworkStatus(ctx context.Context, nw *ipamv1alpha1.Network, log bool) error {
	if err := r.Client.Status().Update(ctx, nw); err != nil {
		klog.Errorf("error while updating Network %q status: %v", client.ObjectKeyFromObject(nw), err)
		return err
	}
	if log {
		klog.Infof("updated Network %q status (spec: %s | status: %s)", client.ObjectKeyFromObject(nw), nw.Spec.CIDR, nw.Status.CIDR)
	}

	return nil
}

// handleNetworkStatus handles the status of a Network resource.
func (r *NetworkReconciler) handleNetworkStatus(ctx context.Context, nw *ipamv1alpha1.Network) error {
	if nw.GetDeletionTimestamp().IsZero() {
		if !controllerutil.ContainsFinalizer(nw, ipamNetworkFinalizer) && !utils.IsPreinstalledResource(nw) {
			// Add finalizer to prevent deletion without unmapping the Network.
			controllerutil.AddFinalizer(nw, ipamNetworkFinalizer)

			// Update the Network object
			if err := r.Update(ctx, nw); err != nil {
				klog.Errorf("error while adding finalizers to Network %q: %v", client.ObjectKeyFromObject(nw), err)
				return err
			}
			klog.Infof("finalizer %q correctly added to Network %q", ipamNetworkFinalizer, client.ObjectKeyFromObject(nw))

			// We return immediately and wait for the next reconcile to eventually update the status.
			return nil
		}

		// Update Network status if it is not set yet
		// The IPAM NetworkAcquire() function is not idempotent, so we avoid to call it
		// multiple times by checking if the status is already set.
		if nw.Status.CIDR == "" {
			desiredCIDR := nw.Spec.CIDR
			// if the Network must not be remapped, we acquire the network specifying to the IPAM that the cidr is immutable.
			immutable := ipamutils.NetworkNotRemapped(nw)
			preallocated := nw.Spec.PreAllocated
			remappedCIDR, err := getRemappedCIDR(ctx, r.ipamClient, desiredCIDR, immutable, preallocated)
			if err != nil {
				return err
			}

			// Update status
			nw.Status.CIDR = remappedCIDR
			if err := r.updateNetworkStatus(ctx, nw, true); err != nil {
				return err
			}
		}
	} else if controllerutil.ContainsFinalizer(nw, ipamNetworkFinalizer) {
		// The resource is being deleted and the finalizer is still present. Call the IPAM to unmap the network CIDR.
		remappedCIDR := nw.Status.CIDR

		if remappedCIDR != "" {
			if _, _, err := net.ParseCIDR(remappedCIDR.String()); err != nil {
				klog.Errorf("Unable to unmap CIDR %s of Network %q (inavlid format): %v", remappedCIDR, client.ObjectKeyFromObject(nw), err)
				return err
			}

			if err := deleteRemappedCIDR(ctx, r.ipamClient, remappedCIDR); err != nil {
				return err
			}
		}

		// Remove status and finalizer, and update the object.
		nw.Status.CIDR = ""
		controllerutil.RemoveFinalizer(nw, ipamNetworkFinalizer)

		if err := r.Update(ctx, nw); err != nil {
			klog.Errorf("error while removing finalizer from Network %q: %v", client.ObjectKeyFromObject(nw), err)
			return err
		}
		klog.Infof("finalizer correctly removed from Network %q", client.ObjectKeyFromObject(nw))
	}

	return nil
}
