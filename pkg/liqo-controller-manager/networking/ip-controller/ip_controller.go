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

package ipctrl

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/ipam"
	"github.com/liqotech/liqo/pkg/utils"
	ipamutils "github.com/liqotech/liqo/pkg/utils/ipam"
)

const (
	ipamIPFinalizer = "ip.ipam.liqo.io/finalizer"
)

// IPReconciler reconciles a IP object.
type IPReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	ipamClient ipam.IPAMClient

	externalCidrRef corev1.ObjectReference
	externalCidr    networkingv1beta1.CIDR
}

// NewIPReconciler returns a new IPReconciler.
func NewIPReconciler(cl client.Client, s *runtime.Scheme, ipamClient ipam.IPAMClient) *IPReconciler {
	return &IPReconciler{
		Client: cl,
		Scheme: s,

		ipamClient: ipamClient,
	}
}

// +kubebuilder:rbac:groups=ipam.liqo.io,resources=ips,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ipam.liqo.io,resources=ips/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ipam.liqo.io,resources=ips/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ipam.liqo.io,resources=networks,verbs=get;list;watch
// +kubebuilder:rbac:groups=ipam.liqo.io,resources=networks/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.k8s.io,resources=endpointslices,verbs=get;list;watch;create;update;patch;delete

// Reconcile Ip objects.
func (r *IPReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Fetch the IP instance
	var ip ipamv1alpha1.IP
	if err := r.Get(ctx, req.NamespacedName, &ip); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof(" %q not found", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		klog.Errorf("an error occurred while getting IP %q: %v", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	// Get the CIDR of the Network referenced by the IP.
	// If it is not set, it is defaulted to the external CIDR of the local cluster.
	networkRef, cidr, err := r.handleNetworkRef(ctx, &ip)
	if client.IgnoreNotFound(err) != nil {
		klog.Errorf("error while handling network reference for IP %q: %v", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	networkExists := !apierrors.IsNotFound(err)
	deleting := !ip.GetDeletionTimestamp().IsZero()

	// Print a warning if the IP is not being deleted and it is referencing a non-existing network.
	if !deleting && !networkExists {
		klog.Warningf("network referenced by IP %q does not exist", req.NamespacedName)
	}

	// The resource is being deleted or the referenced network does not exist:
	// - call the IPAM to release the IP if it set, and empty the status.
	// - remove eventual finalizers from the resource.
	if deleting || !networkExists {
		if err := r.handleIPStatusDeletion(ctx, &ip); err != nil {
			klog.Errorf("error while handling IP status deletion %q: %v", req.NamespacedName, err)
			return ctrl.Result{}, err
		}

		if err := r.ensureAssociatedServiceAbsence(ctx, &ip); err != nil {
			klog.Errorf("error while ensuring absence of the associated service of IP %q: %v", req.NamespacedName, err)
			return ctrl.Result{}, err
		}

		// Remove finalizer from the IP resource if present.
		if controllerutil.ContainsFinalizer(&ip, ipamIPFinalizer) {
			controllerutil.RemoveFinalizer(&ip, ipamIPFinalizer)
			if err := r.Update(ctx, &ip); err != nil {
				klog.Errorf("error while removing finalizer from IP %q: %v", req.NamespacedName, err)
				return ctrl.Result{}, err
			}
			klog.Infof("finalizer %q correctly removed from IP %q", ipamIPFinalizer, req.NamespacedName)
		}

		return ctrl.Result{}, nil
	}

	// Add finalizer to prevent deletion without releasing the IP.
	if !controllerutil.ContainsFinalizer(&ip, ipamIPFinalizer) && !utils.IsPreinstalledResource(&ip) {
		controllerutil.AddFinalizer(&ip, ipamIPFinalizer)
		if err := r.Update(ctx, &ip); err != nil {
			klog.Errorf("error while adding finalizer to IP %q: %v", req.NamespacedName, err)
			return ctrl.Result{}, err
		}
		klog.Infof("finalizer %q correctly added to IP %q", ipamIPFinalizer, req.NamespacedName)

		// We return immediately and wait for the next reconcile to eventually update the status.
		return ctrl.Result{}, nil
	}

	// Add network reference to the IP in the labels. This is used to trigger the reconciliation
	// of the IP by watching deletion events of the father Network.
	if ip.Labels == nil {
		ip.Labels = make(map[string]string)
	}
	ip.Labels[consts.NetworkNamespaceLabelKey] = networkRef.Namespace
	ip.Labels[consts.NetworkNameLabelKey] = networkRef.Name

	if err := r.Update(ctx, &ip); err != nil {
		klog.Errorf("error while updating IP %q: %v", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	// Forge the IP status if it is not set yet.
	if ip.Status.IP == "" {
		if err := r.forgeIPStatus(ctx, &ip, cidr); err != nil {
			klog.Errorf("error while forging IP status for IP %q: %v", req.NamespacedName, err)
			return ctrl.Result{}, err
		}

		if err := r.Client.Status().Update(ctx, &ip); err != nil {
			klog.Errorf("error while updating IP %q: %v", req.NamespacedName, err)
			return ctrl.Result{}, err
		}
		klog.Infof("updated IP %q", req.NamespacedName)
	}

	// Create service and associated endpointslice if the template is defined
	if err := r.handleAssociatedService(ctx, &ip); err != nil {
		klog.Errorf("error while handling associated service for IP %q: %v", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager monitors IP resources.
func (r *IPReconciler) SetupWithManager(mgr ctrl.Manager, workers int) error {
	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlIP).
		For(&ipamv1alpha1.IP{}).
		Owns(&corev1.Service{}).
		Owns(&discoveryv1.EndpointSlice{}).
		Watches(&ipamv1alpha1.Network{}, handler.EnqueueRequestsFromMapFunc(r.ipEnqueuerFromNetwork)).
		WithOptions(controller.Options{MaxConcurrentReconciles: workers}).
		Complete(r)
}

func (r *IPReconciler) ipEnqueuerFromNetwork(ctx context.Context, obj client.Object) []ctrl.Request {
	var requests []reconcile.Request

	// Get the IPs associated with the Network.
	var ipList ipamv1alpha1.IPList
	if err := r.List(ctx, &ipList, client.MatchingLabels{
		consts.NetworkNamespaceLabelKey: obj.GetNamespace(),
		consts.NetworkNameLabelKey:      obj.GetName(),
	}); err != nil {
		klog.Errorf("error while listing IPs associated with Network %q: %v", client.ObjectKeyFromObject(obj), err)
		return nil
	}

	// Enqueue reconcile requests for each IP associated with the Network.
	for i := range ipList.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(&ipList.Items[i]),
		})
	}

	return requests
}

// handleNetworkRef get the CIDR of the Network referenced by the IP, or default to the
// external CIDR of the local cluster if the IP has no NetworkRef set.
func (r *IPReconciler) handleNetworkRef(ctx context.Context, ip *ipamv1alpha1.IP) (*corev1.ObjectReference, networkingv1beta1.CIDR, error) {
	// If the IP has not set a reference to a Network CIDR, we remap it on the external CIDR of the local cluster.
	if ip.Spec.NetworkRef == nil {
		if r.externalCidr == "" {
			network, err := ipamutils.GetExternalCIDRNetwork(ctx, r.Client, corev1.NamespaceAll)
			if err != nil {
				return nil, "", err
			}
			// The externalCIDR Network has no CIDR set yet, we return an error.
			if network.Status.CIDR == "" {
				return nil, "", fmt.Errorf("externalCIDR is not set yet. Configure it to correctly handle IP mapping")
			}

			r.externalCidrRef = corev1.ObjectReference{
				Namespace: network.Namespace,
				Name:      network.Name,
			}
			r.externalCidr = network.Status.CIDR
		}
		return &r.externalCidrRef, r.externalCidr, nil
	}

	// Retrieve the Network object referenced by the IP.
	var network ipamv1alpha1.Network
	if err := r.Get(ctx, client.ObjectKey{Namespace: ip.Spec.NetworkRef.Namespace, Name: ip.Spec.NetworkRef.Name}, &network); err != nil {
		return nil, "", err
	}
	if network.Status.CIDR == "" {
		return nil, "", fmt.Errorf("network %s/%s has no CIDR set yet", network.Namespace, network.Name)
	}
	return ip.Spec.NetworkRef, network.Status.CIDR, nil
}

// forgeIPStatus forge the IP status.
func (r *IPReconciler) forgeIPStatus(ctx context.Context, ip *ipamv1alpha1.IP, cidr networkingv1beta1.CIDR) error {
	// Update IP status if it is not set yet.
	// The IPAM function that maps IPs is not idempotent, so we avoid to call it
	// multiple times by checking if the IP is already set.
	if ip.Status.IP == "" {
		acquiredIP, err := acquireIP(ctx, r.ipamClient, cidr)
		if err != nil {
			return err
		}
		ip.Status.IP = acquiredIP
		ip.Status.CIDR = cidr
	}

	return nil
}

// handleIPStatusDeletion handles the deletion of the IP status.
// It calls the IPAM to release the IP and empties the status.
func (r *IPReconciler) handleIPStatusDeletion(ctx context.Context, ip *ipamv1alpha1.IP) error {
	if ip.Status.IP != "" && ip.Status.CIDR != "" {
		if err := releaseIP(ctx, r.ipamClient, ip.Status.IP, ip.Status.CIDR); err != nil {
			return err
		}

		// Remove status and finalizer, and update the object.
		ip.Status = ipamv1alpha1.IPStatus{}

		// Update the IP status
		if err := r.Client.Status().Update(ctx, ip); err != nil {
			return err
		}

		klog.Infof("IP %q status correctly cleaned", client.ObjectKeyFromObject(ip))
	}

	return nil
}

// acquireIP acquire a free IP of a given CIDR from the IPAM.
func acquireIP(ctx context.Context, ipamClient ipam.IPAMClient, cidr networkingv1beta1.CIDR) (networkingv1beta1.IP, error) {
	switch ipamClient.(type) {
	case nil:
		// IPAM is not enabled, return an error.
		return "", fmt.Errorf("IPAM is not enabled")
	default:
		// interact with the IPAM to retrieve the correct mapping.
		response, err := ipamClient.IPAcquire(ctx, &ipam.IPAcquireRequest{
			Cidr: string(cidr),
		})
		if err != nil {
			klog.Errorf("IPAM: error while acquiring IP from CIDR %q: %v", cidr, err)
			return "", err
		}
		klog.Infof("IPAM: acquired IP %q from CIDR %q", response.Ip, cidr)
		return networkingv1beta1.IP(response.Ip), nil
	}
}

// releaseIP release a IP of a given CIDR from the IPAM.
func releaseIP(ctx context.Context, ipamClient ipam.IPAMClient, ip networkingv1beta1.IP, cidr networkingv1beta1.CIDR) error {
	switch ipamClient.(type) {
	case nil:
		// If the IPAM is not enabled we do not need to release any IP.
		return nil
	default:
		// Interact with the IPAM to release the translation.
		_, err := ipamClient.IPRelease(ctx, &ipam.IPReleaseRequest{
			Ip:   string(ip),
			Cidr: string(cidr),
		})
		if err != nil {
			klog.Errorf("IPAM: error while de allocating IP %q from CIDR: %q: %v", ip, cidr, err)
			return err
		}
		klog.Infof("IPAM: de-allocated IP %q from CIDR %q", ip, cidr)
		return nil
	}
}
