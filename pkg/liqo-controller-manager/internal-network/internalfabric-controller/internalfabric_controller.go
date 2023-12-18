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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

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

	route, err := forgeRouteConfiguration(internalFabric.Name, internalFabric.Namespace, internalFabric)
	if err != nil {
		klog.Errorf("Unable to forge RouteConfiguration %q: %s", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	if _, err = controllerutil.CreateOrUpdate(ctx, r.Client, route, func() error {
		if err := mutateRouteConfiguration(route, internalFabric); err != nil {
			klog.Errorf("Unable to mutate RouteConfiguration %q: %s", req.NamespacedName, err)
			return err
		}
		return controllerutil.SetControllerReference(internalFabric, route, r.Scheme)
	}); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func forgeRouteConfiguration(name, namespace string,
	internalFabric *networkingv1alpha1.InternalFabric) (*networkingv1alpha1.RouteConfiguration, error) {
	route := &networkingv1alpha1.RouteConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	if err := mutateRouteConfiguration(route, internalFabric); err != nil {
		return nil, err
	}

	return route, nil
}

func mutateRouteConfiguration(route *networkingv1alpha1.RouteConfiguration, internalFabric *networkingv1alpha1.InternalFabric) error {
	remoteClusterID, ok := internalFabric.Labels[consts.RemoteClusterID]
	if !ok {
		return fmt.Errorf("internal fabric %q does not have remote cluster ID label", client.ObjectKeyFromObject(internalFabric))
	}

	if internalFabric.Spec.GatewayIP == "" {
		return fmt.Errorf("internal fabric %q has gateway ip empty", client.ObjectKeyFromObject(internalFabric))
	}

	if route.Labels == nil {
		route.Labels = make(map[string]string)
	}
	route.SetLabels(labels.Merge(route.Labels, geneve.ForgeRouteTargetLabels(remoteClusterID)))
	route.SetLabels(labels.Merge(route.Labels, labels.Set{consts.RemoteClusterID: remoteClusterID}))

	var rules []networkingv1alpha1.Rule
	for _, remoteCIDR := range internalFabric.Spec.RemoteCIDRs {
		rule := networkingv1alpha1.Rule{
			Routes: []networkingv1alpha1.Route{
				{
					Dst: ptr.To(remoteCIDR),
					Gw:  ptr.To(networkingv1alpha1.IP("10.200.0.1")), // TODO:: get from Network resource
					// Gw:  ptr.To(internalFabric.Spec.GatewayIP), // TODO: check if makes sense
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

	return nil
}

// SetupWithManager register the InternalFabricReconciler to the manager.
func (r *InternalFabricReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Owns(&networkingv1alpha1.RouteConfiguration{}).
		For(&networkingv1alpha1.InternalFabric{}).
		Complete(r)
}
