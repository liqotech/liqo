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

package internalnodecontroller

import (
	"context"
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
)

// InternalNodeReconciler manage InternalNode lifecycle.
type InternalNodeReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// NewInternalNodeReconciler returns a new InternalNodeReconciler.
func NewInternalNodeReconciler(cl client.Client, s *runtime.Scheme) *InternalNodeReconciler {
	return &InternalNodeReconciler{
		Client: cl,
		Scheme: s,
	}
}

// cluster-role
// +kubebuilder:rbac:groups=networking.liqo.io,resources=internalnodes,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=routeconfigurations,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=internalfabrics,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch

// Reconcile manage InternalNode lifecycle.
func (r *InternalNodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	var internalNode networkingv1alpha1.InternalNode
	if err = r.Get(ctx, req.NamespacedName, &internalNode); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("InternalNode %q not found", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		klog.Errorf("Unable to get the InternalNode %q: %s", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	// Get the associated InternalFabric.
	var internalFabric networkingv1alpha1.InternalFabric
	internalFabricNsName := types.NamespacedName{
		Namespace: internalNode.Spec.FabricRef.Namespace,
		Name:      internalNode.Spec.FabricRef.Name,
	}
	if err = r.Get(ctx, internalFabricNsName, &internalFabric); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("InternalFabric %q not found", internalFabricNsName)
			return ctrl.Result{}, nil
		}
		klog.Errorf("Unable to get InternalFabric %q: %s", internalFabricNsName, err)
		return ctrl.Result{}, err
	}

	// Create the Route for every remoteCIDRs of the InternalFabric.
	for _, remoteCIDR := range internalFabric.Spec.RemoteCIDRs {
		routeName := fmt.Sprintf("%s-%s-%s", internalFabric.Name, internalNode.Name, strings.ReplaceAll(remoteCIDR.String(), "/", "-"))

		route := forgeRouteConfiguration(routeName, internalNode.Namespace, remoteCIDR, &internalFabric, &internalNode)

		if _, err = controllerutil.CreateOrUpdate(ctx, r.Client, route, func() error {
			mutateRouteConfiguration(route, remoteCIDR, &internalFabric, &internalNode)
			return controllerutil.SetControllerReference(&internalNode, route, r.Scheme)
		}); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager register the InternalNodeReconciler to the manager.
func (r *InternalNodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1alpha1.InternalNode{}).
		Owns(&networkingv1alpha1.RouteConfiguration{}).
		Complete(r)
}

func forgeRouteConfiguration(name, namespace string, remoteCIDR networkingv1alpha1.CIDR,
	internalFabric *networkingv1alpha1.InternalFabric, internalNode *networkingv1alpha1.InternalNode) *networkingv1alpha1.RouteConfiguration {
	route := &networkingv1alpha1.RouteConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	mutateRouteConfiguration(route, remoteCIDR, internalFabric, internalNode)

	return route
}

func mutateRouteConfiguration(route *networkingv1alpha1.RouteConfiguration, remoteCIDR networkingv1alpha1.CIDR,
	internalFabric *networkingv1alpha1.InternalFabric, internalNode *networkingv1alpha1.InternalNode) {
	if route.Labels == nil {
		route.Labels = make(map[string]string)
	}
	route.Labels[consts.InternalFabricLabelKey] = internalFabric.Name

	route.Spec = networkingv1alpha1.RouteConfigurationSpec{
		Table: networkingv1alpha1.Table{
			Name: route.Name,
			Rules: []networkingv1alpha1.Rule{
				{
					Routes: []networkingv1alpha1.Route{
						{
							Dst: ptr.To(remoteCIDR),
							Src: ptr.To(internalNode.Spec.IP),
							// TODO: add support for Gw
							// TODO: add support for Dev
						},
					},
					Dst: ptr.To(remoteCIDR),
					// TODO: add support for Src
				},
			},
		},
	}
}
