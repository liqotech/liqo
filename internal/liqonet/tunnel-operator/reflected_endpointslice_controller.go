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

package tunneloperator

import (
	"context"
	"fmt"
	"sync"

	"github.com/containernetworking/plugins/pkg/ns"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/liqotech/liqo/apis/offloading/v1alpha1"
	virtualkubeletv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	nsoffctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/namespaceoffloading-controller"
	liqoipset "github.com/liqotech/liqo/pkg/liqonet/ipset"
	liqoiptables "github.com/liqotech/liqo/pkg/liqonet/iptables"
	getters "github.com/liqotech/liqo/pkg/utils/getters"
	virtualnodeutils "github.com/liqotech/liqo/pkg/utils/virtualnode"
)

// ManagedByK8sEndpointsliceControllerValue is the label value used to indicate that a given resource is managed by K8s Endpointslice controller.
const ManagedByK8sEndpointsliceControllerValue = "endpointslice-controller.k8s.io"

// ReflectedEndpointsliceController reconciles an offloaded Service object.
type ReflectedEndpointsliceController struct {
	client.Client
	liqoiptables.IPTHandler
	Scheme *runtime.Scheme

	*liqoipset.IPSHandler

	// Liqo Gateway network namespace
	gatewayNetns ns.NetNS

	// Local cache of podInfo objects
	podsInfo *sync.Map

	// Local cache of endpointsliceInfo objects
	endpointslicesInfo *sync.Map
}

// +kubebuilder:rbac:groups=discovery.k8s.io,resources=endpointslices,verbs=get;list;watch
// +kubebuilder:rbac:groups=discovery.k8s.io,resources=endpointslices/endpoints,verbs=get;list;watch
// +kubebuilder:rbac:groups=discovery.k8s.io,resources=endpointslices/endpoints/addresses,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups=offloading.liqo.io,resources=namespaceoffloadings,verbs=get;list;watch
// +kubebuilder:rbac:groups=virtualkubelet.liqo.io,resources=virtualnodes,verbs=get;list;watch

// NewReflectedEndpointsliceController instantiates and initializes the reflected endpointslice controller.
func NewReflectedEndpointsliceController(
	cl client.Client,
	scheme *runtime.Scheme,
	gatewayNetns ns.NetNS,
	podsInfo, endpointslicesInfo *sync.Map,
) (*ReflectedEndpointsliceController, error) {
	// Create the IPTables handler.
	iptablesHandler, err := liqoiptables.NewIPTHandler()
	if err != nil {
		return nil, err
	}
	// Create the IPSet handler.
	ipsetHandler := liqoipset.NewIPSHandler()
	// Create and return the controller.
	return &ReflectedEndpointsliceController{
		Client:             cl,
		Scheme:             scheme,
		IPTHandler:         iptablesHandler,
		IPSHandler:         &ipsetHandler,
		gatewayNetns:       gatewayNetns,
		podsInfo:           podsInfo,
		endpointslicesInfo: endpointslicesInfo,
	}, nil
}

// Reconcile local endpointslices that are also reflected on remote clusters as a result of offloading.
func (r *ReflectedEndpointsliceController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var ensureIptablesRules = func(netns ns.NetNS) error {
		return r.EnsureRulesForClustersForwarding(r.podsInfo, r.endpointslicesInfo, r.IPSHandler)
	}
	nsName := req.NamespacedName
	klog.V(3).Infof("Reconcile Endpointslice %q", nsName)

	endpointslice := discoveryv1.EndpointSlice{}
	if err := r.Get(ctx, nsName, &endpointslice); err != nil {
		if apierror.IsNotFound(err) {
			// Endpointslice not found, endpointsliceInfo object found: delete endpointInfo objects.
			if value, ok := r.endpointslicesInfo.LoadAndDelete(nsName); ok {
				klog.V(3).Infof("Endpointslice %q not found: ensuring updated iptables rules", nsName)

				// Soft delete object
				endpointsInfo := value.(map[string]liqoiptables.EndpointInfo)
				for endpoint, endpointInfo := range endpointsInfo {
					endpointInfo.Deleting = true
					endpointsInfo[endpoint] = endpointInfo
				}
				r.endpointslicesInfo.Store(nsName, endpointsInfo)

				if err := r.gatewayNetns.Do(ensureIptablesRules); err != nil {
					return ctrl.Result{}, fmt.Errorf("error while ensuring iptables rules: %w", err)
				}

				// Hard delete object
				r.endpointslicesInfo.Delete(nsName)
			}

			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Check endpointslice's namespace offloading
	nsOffloading, err := getters.GetOffloadingByNamespace(ctx, r.Client, endpointslice.Namespace)
	if err != nil {
		if apierror.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		// Delete endpointInfo objects related to this endpointslice
		if value, ok := r.endpointslicesInfo.LoadAndDelete(nsName); ok {
			// Endpointslice not found, endpointsliceInfo object found: ensure iptables rules
			klog.V(3).Infof("Endpointslice %q not found: ensuring updated iptables rules", nsName)

			// Soft delete object
			endpointsInfo := value.(map[string]liqoiptables.EndpointInfo)
			for endpoint, endpointInfo := range endpointsInfo {
				endpointInfo.Deleting = true
				endpointsInfo[endpoint] = endpointInfo
			}
			r.endpointslicesInfo.Store(nsName, endpointsInfo)

			if err := r.gatewayNetns.Do(ensureIptablesRules); err != nil {
				return ctrl.Result{}, fmt.Errorf("error while ensuring iptables rules: %w", err)
			}

			// Hard delete object
			r.endpointslicesInfo.Delete(nsName)
		}

		return ctrl.Result{}, err
	}

	clusterSelector := nsOffloading.Spec.ClusterSelector

	nodes := virtualkubeletv1alpha1.VirtualNodeList{}
	if err := r.List(ctx, &nodes); err != nil {
		return ctrl.Result{}, err
	}

	// Build endpointInfo objects
	endpointsInfo := map[string]liqoiptables.EndpointInfo{}
	// For each endpoint, find ClusterIDs of clusters that can reach that endpoint
	for _, endpoint := range endpointslice.Endpoints {
		clusterIDs := []string{}
		for i := range nodes.Items {
			if *endpoint.NodeName == nodes.Items[i].Name {
				continue
			}

			matchClusterSelctor, err := nsoffctrl.MatchVirtualNodeSelectorTerms(ctx, r.Client, &nodes.Items[i], &clusterSelector)
			if err != nil {
				return ctrl.Result{}, err
			}

			if matchClusterSelctor {
				if clusterID, found := virtualnodeutils.GetVirtualNodeClusterID(&nodes.Items[i]); found {
					clusterIDs = append(clusterIDs, clusterID)
				}
			}
		}

		endpointsInfo[endpoint.Addresses[0]] = liqoiptables.EndpointInfo{Address: endpoint.Addresses[0], SrcClusterIDs: clusterIDs}
	}

	// Check if the object is under deletion
	if !endpointslice.ObjectMeta.DeletionTimestamp.IsZero() {
		// Endpointslice under deletion: skip creation of iptables rules and return no error
		klog.Infof("Endpointslice %q under deletion: skipping iptables rules update", nsName)
		return ctrl.Result{}, nil
	}

	// Check if endpoint(s) are no more part of the endpointslice
	value, loaded := r.endpointslicesInfo.Load(nsName)
	if loaded {
		oldEndpointsInfo := value.(map[string]liqoiptables.EndpointInfo)
		for oldEndpoint, oldEndpointInfo := range oldEndpointsInfo {
			if _, ok := endpointsInfo[oldEndpoint]; !ok {
				oldEndpointInfo.Deleting = true
				endpointsInfo[oldEndpoint] = oldEndpointInfo
			}
		}
	}

	// Check if there aren't new information: in this case it's not necessary ensure iptables rules
	if len(endpointsInfo) == 0 {
		// Endpoints fields of Endpoinslice yet empty: skip creation of iptables rules and return no error
		klog.Infof("Endpoints of endpointslice %q not yet set: skipping iptables rules update", nsName)
		return ctrl.Result{}, nil
	}

	// Store endpointslicesInfo object
	r.endpointslicesInfo.Store(nsName, endpointsInfo)

	// Ensure iptables rules
	klog.V(3).Infof("Ensuring updated iptables rules")
	if err := r.gatewayNetns.Do(ensureIptablesRules); err != nil {
		return ctrl.Result{}, fmt.Errorf("error while ensuring iptables rules: %w", err)
	}

	return ctrl.Result{}, nil
}

func (r *ReflectedEndpointsliceController) endpointsliceEnqueuer(ctx context.Context, obj client.Object) []ctrl.Request {
	gvks, _, err := r.Scheme.ObjectKinds(obj)
	// Should never happen, but if it happens we panic.
	utilruntime.Must(err)

	// If gvk is found we log.
	if len(gvks) != 0 {
		klog.V(4).Infof("handling resource %q of type %q", klog.KObj(obj), gvks[0].String())
	}

	endpointslices := discoveryv1.EndpointSliceList{}
	if err := r.List(ctx, &endpointslices, client.InNamespace(obj.GetNamespace())); err != nil {
		klog.Error(err)
		return []ctrl.Request{}
	}

	if len(endpointslices.Items) == 0 {
		klog.V(4).Infof("no endpointslice found for resource %q", klog.KObj(obj))
		return []ctrl.Request{}
	}

	requests := []ctrl.Request{}
	for i := range endpointslices.Items {
		klog.V(4).Infof("enqueuing endpointslice %q", endpointslices.Items[i].Name)
		requests = append(requests, ctrl.Request{NamespacedName: types.NamespacedName{Name: endpointslices.Items[i].Name, Namespace: obj.GetNamespace()}})
	}

	return requests
}

// SetupWithManager sets up the controller with the Manager.
func (r *ReflectedEndpointsliceController) SetupWithManager(mgr ctrl.Manager) error {
	// endpointslicePredicate selects those endpointslices matching the provided label
	endpointslicePredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchLabels: map[string]string{
			discoveryv1.LabelManagedBy: ManagedByK8sEndpointsliceControllerValue,
		},
	})
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&discoveryv1.EndpointSlice{}, builder.WithPredicates(endpointslicePredicate)).
		Watches(&v1alpha1.NamespaceOffloading{}, handler.EnqueueRequestsFromMapFunc(r.endpointsliceEnqueuer)).
		Complete(r)
}
