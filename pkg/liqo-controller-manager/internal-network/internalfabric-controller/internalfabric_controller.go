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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	"github.com/liqotech/liqo/pkg/utils/getters"
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
// +kubebuilder:rbac:groups=networking.liqo.io,resources=internalnodes,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch

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

	nodes, err := getters.ListPhysicalNodes(ctx, r.Client)
	if err != nil {
		klog.Errorf("Unable to list physical nodes: %s", err)
		return ctrl.Result{}, err
	}

	for i := range nodes.Items {
		node := &nodes.Items[i]
		if err = r.reconcileNode(ctx, internalFabric, node); err != nil {
			klog.Errorf("Unable to reconcile node %q: %s", node.Name, err)
			return ctrl.Result{}, err
		}
	}

	if err = r.Status().Update(ctx, internalFabric); err != nil {
		klog.Errorf("Unable to update InternalFabric status: %s", err)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *InternalFabricReconciler) reconcileNode(ctx context.Context,
	internalFabric *networkingv1alpha1.InternalFabric, node *corev1.Node) error {
	return nil
}

// SetupWithManager register the InternalFabricReconciler to the manager.
func (r *InternalFabricReconciler) SetupWithManager(mgr ctrl.Manager) error {
	nodeEnqueuer := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
		var internalFabrics networkingv1alpha1.InternalFabricList
		if err := r.List(ctx, &internalFabrics); err != nil {
			klog.Errorf("Unable to list InternalFabrics: %s", err)
			return nil
		}

		var requests = make([]reconcile.Request, len(internalFabrics.Items))
		for i := range internalFabrics.Items {
			internalFabric := &internalFabrics.Items[i]
			requests[i] = reconcile.Request{
				NamespacedName: client.ObjectKey{
					Name:      internalFabric.Name,
					Namespace: internalFabric.Namespace,
				},
			}
		}
		return requests
	})

	return ctrl.NewControllerManagedBy(mgr).
		Watches(&corev1.Node{}, nodeEnqueuer).
		For(&networkingv1alpha1.InternalFabric{}).
		Complete(r)
}
