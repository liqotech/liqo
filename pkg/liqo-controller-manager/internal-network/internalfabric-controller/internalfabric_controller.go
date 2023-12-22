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

package internalfabriccontroller

import (
	"context"
	"fmt"
	"sort"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/gateway/fabric/geneve"
)

// InternalFabricReconciler manage InternalFabric lifecycle.
type InternalFabricReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// NewInternalFabricReconciler returns a new InternalFabricReconciler.
func NewInternalFabricReconciler(cl client.Client, s *runtime.Scheme) *InternalFabricReconciler {
	return &InternalFabricReconciler{
		Client: cl,
		Scheme: s,
	}
}

// cluster-role
// +kubebuilder:rbac:groups=networking.liqo.io,resources=internalfabrics,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=internalfabrics/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=routeconfigurations,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=genevetunnels,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=internalnodes,verbs=get;list;watch

// Reconcile manage InternalFabric lifecycle.
func (r *InternalFabricReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	internalFabric := &networkingv1alpha1.InternalFabric{}
	if err = r.Get(ctx, req.NamespacedName, internalFabric); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("InternalFabric %q not found", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		klog.Errorf("Unable to get the InternalFabric %q: %s", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	if !internalFabric.DeletionTimestamp.IsZero() && controllerutil.ContainsFinalizer(internalFabric, consts.InternalFabricGeneveTunnelFinalizer) {
		if err = deleteGeneveTunnels(ctx, r.Client, internalFabric); err != nil {
			klog.Errorf("Unable to delete GeneveTunnels: %s", err)
			return ctrl.Result{}, err
		}
	}

	// route configuration

	if err = r.ensureRouteConfiguration(ctx, internalFabric); err != nil {
		return ctrl.Result{}, err
	}

	// geneve tunnel

	var internalNodeList networkingv1alpha1.InternalNodeList
	if err = r.List(ctx, &internalNodeList); err != nil {
		klog.Errorf("Unable to list InternalNodes: %s", err)
		return ctrl.Result{}, err
	}

	if err = ensureGeneveTunnels(ctx, r.Client, r.Scheme, internalFabric, &internalNodeList); err != nil {
		klog.Errorf("Unable to ensure GeneveTunnels: %s", err)
		return ctrl.Result{}, err
	}

	if err = cleanupGeneveTunnels(ctx, r.Client, internalFabric, &internalNodeList); err != nil {
		klog.Errorf("Unable to cleanup GeneveTunnels: %s", err)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *InternalFabricReconciler) ensureRouteConfiguration(ctx context.Context, internalFabric *networkingv1alpha1.InternalFabric) error {
	remoteClusterID, ok := internalFabric.Labels[consts.RemoteClusterID]
	if !ok {
		return fmt.Errorf("internal fabric %q does not have remote cluster ID label", client.ObjectKeyFromObject(internalFabric))
	}
	if internalFabric.Spec.Interface.Node.Name == "" {
		return fmt.Errorf("internal fabric %q has node interface name empty", client.ObjectKeyFromObject(internalFabric))
	}

	route := &networkingv1alpha1.RouteConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      internalFabric.Name,
			Namespace: internalFabric.Namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, route, func() error {
		// Forge metadata
		if route.Labels == nil {
			route.Labels = make(map[string]string)
		}
		route.SetLabels(labels.Merge(route.Labels, geneve.ForgeRouteTargetLabels(remoteClusterID)))
		route.SetLabels(labels.Merge(route.Labels, labels.Set{consts.RemoteClusterID: remoteClusterID}))

		// Add route rule for every remote CIDR
		var rules []networkingv1alpha1.Rule
		remoteCIDRs := internalFabric.Spec.RemoteCIDRs
		// sort slice to prevent useless updates if CIDRs are in different order
		sort.Slice(remoteCIDRs, func(i, j int) bool {
			return remoteCIDRs[i] < remoteCIDRs[j]
		})
		for _, remoteCIDR := range remoteCIDRs {
			rule := networkingv1alpha1.Rule{
				Routes: []networkingv1alpha1.Route{
					{
						Dst: ptr.To(remoteCIDR),
						Gw:  ptr.To(networkingv1alpha1.IP("10.200.0.1")), // TODO:: get from Network resource
						Dev: ptr.To(internalFabric.Spec.Interface.Node.Name),
					},
				},
				Dst: ptr.To(remoteCIDR),
			}

			rules = append(rules, rule)
		}

		route.Spec = networkingv1alpha1.RouteConfigurationSpec{
			Table: networkingv1alpha1.Table{
				Name:  route.Name,
				Rules: rules,
			},
		}

		return controllerutil.SetControllerReference(internalFabric, route, r.Scheme)
	})
	if err != nil {
		klog.Errorf("Unable to create or update RouteConfiguration %q: %s", route.Name, err)
		return err
	}

	return nil
}

// SetupWithManager register the InternalFabricReconciler to the manager.
func (r *InternalFabricReconciler) SetupWithManager(mgr ctrl.Manager) error {
	internalNodeEnqueuer := handler.EnqueueRequestsFromMapFunc(
		func(ctx context.Context, obj client.Object) []reconcile.Request {
			var requests []reconcile.Request

			var internalFabricList networkingv1alpha1.InternalFabricList
			if err := r.List(ctx, &internalFabricList); err != nil {
				klog.Errorf("Unable to list InternalFabrics: %s", err)
				return nil
			}

			for i := range internalFabricList.Items {
				fabric := &internalFabricList.Items[i]

				requests = append(requests, reconcile.Request{
					NamespacedName: client.ObjectKeyFromObject(fabric),
				})
			}

			return requests
		},
	)

	return ctrl.NewControllerManagedBy(mgr).
		Watches(&networkingv1alpha1.InternalNode{}, internalNodeEnqueuer).
		Owns(&networkingv1alpha1.RouteConfiguration{}).
		Owns(&networkingv1alpha1.GeneveTunnel{}).
		For(&networkingv1alpha1.InternalFabric{}).
		Complete(r)
}
