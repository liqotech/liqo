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

package ipctrl

import (
	"context"
	"slices"

	v1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	"github.com/liqotech/liqo/pkg/ipam"
	"github.com/liqotech/liqo/pkg/utils/getters"
	ipamutils "github.com/liqotech/liqo/pkg/utils/ipam"
	"github.com/liqotech/liqo/pkg/utils/slice"
)

const (
	ipamIPFinalizer = "ip.ipam.liqo.io/finalizer"
)

// IPReconciler reconciles a IP object.
type IPReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	ipamClient      ipam.IpamClient
	externalCIDRSet bool
}

// NewIPReconciler returns a new IPReconciler.
func NewIPReconciler(cl client.Client, s *runtime.Scheme, ipamClient ipam.IpamClient) *IPReconciler {
	return &IPReconciler{
		Client: cl,
		Scheme: s,

		ipamClient:      ipamClient,
		externalCIDRSet: false,
	}
}

// +kubebuilder:rbac:groups=ipam.liqo.io,resources=ips,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ipam.liqo.io,resources=ips/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ipam.liqo.io,resources=ips/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.liqo.io,resources=configurations,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.k8s.io,resources=endpointslices,verbs=get;list;watch;create;update;patch;delete

// Reconcile Ip objects.
func (r *IPReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var ip ipamv1alpha1.IP
	var desiredIP networkingv1alpha1.IP

	// Fetch the IP instance
	if err := r.Get(ctx, req.NamespacedName, &ip); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof(" %q not found", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		klog.Errorf("an error occurred while getting IP %q: %v", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	if !r.externalCIDRSet {
		// Retrieve the externalCIDR of the local cluster
		_, err := ipamutils.GetExternalCIDR(ctx, r.Client)
		if apierrors.IsNotFound(err) {
			klog.Errorf("ExternalCIDR is not set yet. Configure it to correctly handle IP mappings")
			return ctrl.Result{}, err
		} else if err != nil {
			klog.Errorf("error while retrieving externalCIDR: %v", err)
			return ctrl.Result{}, err
		}
		// The external CIDR is set, we do not need to check it again in successive reconciliations.
		r.externalCIDRSet = true
	}

	desiredIP = ip.Spec.IP

	// Get the clusterIDs of all remote clusters
	configurations, err := getters.ListConfigurationsByLabel(ctx, r.Client, labels.Everything())
	if err != nil {
		klog.Errorf("error while listing virtual nodes: %v", err)
		return ctrl.Result{}, err
	}

	clusterIDs := getters.RetrieveClusterIDsFromObjectsLabels(slice.ToPointerSlice(configurations.Items))

	if ip.GetDeletionTimestamp().IsZero() {
		if !controllerutil.ContainsFinalizer(&ip, ipamIPFinalizer) {
			// Add finalizer to prevent deletion without unmapping the IP.
			controllerutil.AddFinalizer(&ip, ipamIPFinalizer)

			// Update the IP object
			if err := r.Update(ctx, &ip); err != nil {
				klog.Errorf("error while adding finalizer to IP %q: %v", req.NamespacedName, err)
				return ctrl.Result{}, err
			}
			klog.Infof("finalizer %q correctly added to IP %q", ipamIPFinalizer, req.NamespacedName)

			// We return immediately and wait for the next reconcile to eventually update the status.
			return ctrl.Result{}, nil
		}

		needUpdate, err := r.forgeIPMappings(ctx, clusterIDs, desiredIP, &ip)
		if err != nil {
			return ctrl.Result{}, err
		}

		// Update resource status if needed
		if needUpdate {
			if err := r.Client.Status().Update(ctx, &ip); err != nil {
				klog.Errorf("error while updating IP %q status: %v", req.NamespacedName, err)
				return ctrl.Result{}, err
			}
			klog.Infof("updated IP %q status", req.NamespacedName)
		}

		// Create service and associated endpointslice if the template is defined
		if err := r.handleAssociatedService(ctx, &ip); err != nil {
			return ctrl.Result{}, err
		}
	} else if controllerutil.ContainsFinalizer(&ip, ipamIPFinalizer) {
		// the resource is being deleted, but the finalizer is present:
		// - unmap the remapped IPs
		// - remove finalizer from the resource.
		if err := r.handleDelete(ctx, clusterIDs, desiredIP, &ip); err != nil {
			return ctrl.Result{}, err
		}

		// Update the IP object
		if err := r.Update(ctx, &ip); err != nil {
			klog.Errorf("error while removing finalizer from IP %q: %v", req.NamespacedName, err)
			return ctrl.Result{}, err
		}
		klog.Infof("finalizer %q correctly removed from IP %q", ipamIPFinalizer, req.NamespacedName)

		// We do not have to delete possible service and endpointslice associated, as already deleted by
		// the Kubernetes garbage collector (since they are owned by the IP resource).
	}

	return ctrl.Result{}, nil
}

// SetupWithManager monitors IP resources.
func (r *IPReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, workers int) error {
	// List all IP resources and enqueue them.
	enqueuer := func(_ context.Context, obj client.Object) []reconcile.Request {
		var ipList ipamv1alpha1.IPList
		if err := r.List(ctx, &ipList); err != nil {
			klog.Errorf("error while listing IPs: %v", err)
			return nil
		}
		var requests []reconcile.Request
		for i := range ipList.Items {
			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{
				Namespace: ipList.Items[i].Namespace, Name: ipList.Items[i].Name}})
		}
		return requests
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&ipamv1alpha1.IP{}).
		Owns(&v1.Service{}).
		Owns(&discoveryv1.EndpointSlice{}).
		Watches(&networkingv1alpha1.Configuration{}, handler.EnqueueRequestsFromMapFunc(enqueuer)).
		WithOptions(controller.Options{MaxConcurrentReconciles: workers}).
		Complete(r)
}

// forgeIPMappings forge the IP mappings for each remote cluster. Return true if the status must be updated, false otherwise.
func (r *IPReconciler) forgeIPMappings(ctx context.Context, clusterIDs []string, desiredIP networkingv1alpha1.IP, ip *ipamv1alpha1.IP) (bool, error) {
	// Update IP status for each remote cluster
	needUpdate := false

	if ip.Status.IPMappings == nil {
		ip.Status.IPMappings = make(map[string]networkingv1alpha1.IP)
	}

	for i := range clusterIDs {
		remoteClusterID := &clusterIDs[i]
		// Update IP status if it is not set yet
		// The IPAM function that maps IPs is not idempotent, so we avoid to call it
		// multiple times by checking if the IP for that remote cluster is already set.
		_, found := ip.Status.IPMappings[*remoteClusterID]
		if !found {
			remappedIP, err := getRemappedIP(ctx, r.ipamClient, *remoteClusterID, desiredIP)
			if err != nil {
				return false, err
			}
			ip.Status.IPMappings[*remoteClusterID] = remappedIP
			needUpdate = true
		}
	}

	// Check if the IPMappings has entries associated to clusters that have been deleted (i.e., the virtualNode is missing)
	for entry := range ip.Status.IPMappings {
		if !slices.Contains(clusterIDs, entry) {
			// We ignore eventual errors from the IPAM because the entries in the IpamStorage for that cluster
			// may have been already removed.
			_ = deleteRemappedIP(ctx, r.ipamClient, entry, desiredIP)
			delete(ip.Status.IPMappings, entry)
			needUpdate = true
		}
	}

	return needUpdate, nil
}

// handleDelete handles the deletion of the IP resource. It call the IPAM to unmap the IPs of each remote cluster.
func (r *IPReconciler) handleDelete(ctx context.Context, clusterIDs []string, desiredIP networkingv1alpha1.IP, ip *ipamv1alpha1.IP) error {
	for i := range clusterIDs {
		remoteClusterID := &clusterIDs[i]
		if err := deleteRemappedIP(ctx, r.ipamClient, *remoteClusterID, desiredIP); err != nil {
			return err
		}
		delete(ip.Status.IPMappings, *remoteClusterID)
	}

	// Remove status and finalizer, and update the object.
	ip.Status.IPMappings = nil
	controllerutil.RemoveFinalizer(ip, ipamIPFinalizer)

	return nil
}

// getRemappedIP returns the remapped IP for the given IP and remote clusterID.
func getRemappedIP(ctx context.Context, ipamClient ipam.IpamClient, remoteClusterID string,
	desiredIP networkingv1alpha1.IP) (networkingv1alpha1.IP, error) {
	switch ipamClient.(type) {
	case nil:
		// IPAM is not enabled, use original IP from spec
		return desiredIP, nil
	default:
		// interact with the IPAM to retrieve the correct mapping.
		response, err := ipamClient.MapEndpointIP(ctx, &ipam.MapRequest{
			ClusterID: remoteClusterID, Ip: desiredIP.String()})
		if err != nil {
			klog.Errorf("IPAM: error while mapping IP %s for remote cluster %q: %v", desiredIP, remoteClusterID, err)
			return "", err
		}
		klog.Infof("IPAM: mapped IP %s to %s for remote cluster %q", desiredIP, response.Ip, remoteClusterID)
		return networkingv1alpha1.IP(response.Ip), nil
	}
}

// deleteRemappedIP unmaps the IP for the given remote clusterID.
func deleteRemappedIP(ctx context.Context, ipamClient ipam.IpamClient, remoteClusterID string, desiredIP networkingv1alpha1.IP) error {
	switch ipamClient.(type) {
	case nil:
		// If the IPAM is not enabled we do not need to release the translation.
		return nil
	default:
		// Interact with the IPAM to release the translation.
		_, err := ipamClient.UnmapEndpointIP(ctx, &ipam.UnmapRequest{
			ClusterID: remoteClusterID, Ip: desiredIP.String()})
		if err != nil {
			klog.Errorf("IPAM: error while unmapping IP %s for remote cluster %q: %v", desiredIP, remoteClusterID, err)
			return err
		}
		klog.Infof("IPAM: unmapped IP %s for remote cluster %q", desiredIP, remoteClusterID)
		return nil
	}
}
