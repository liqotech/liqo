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

package nodecontroller

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	"github.com/liqotech/liqo/pkg/discovery"
)

// NodeReconciler creates and manages InternalNodes.
type NodeReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// NewNodeReconciler returns a new NodeReconciler.
func NewNodeReconciler(cl client.Client, s *runtime.Scheme) *NodeReconciler {
	return &NodeReconciler{
		Client: cl,
		Scheme: s,
	}
}

// cluster-role
// +kubebuilder:rbac:groups=networking.liqo.io,resources=internalnodes,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch

// Reconcile manage Node lifecycle.
func (r *NodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	node := &corev1.Node{}
	if err = r.Get(ctx, req.NamespacedName, node); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("Node %q not found", req.Name)
			return ctrl.Result{}, nil
		}
		klog.Errorf("Unable to get the Node %q: %s", req.Name, err)
		return ctrl.Result{}, err
	}

	internalNode := &networkingv1alpha1.InternalNode{
		ObjectMeta: metav1.ObjectMeta{
			Name: node.Name,
		},
	}
	if _, err = controllerutil.CreateOrUpdate(ctx, r.Client, internalNode, func() error {
		address, err := discovery.GetInternalAddress(node)
		if err != nil {
			return err
		}
		internalNode.Spec.NodeAddress = address

		// TODO:: generate random name for the gateway interface
		internalNode.Spec.Interface.Gateway.Name = internalNode.Name

		return controllerutil.SetControllerReference(node, internalNode, r.Scheme)
	}); err != nil {
		klog.Errorf("Unable to create or update InternalNode %q: %s", internalNode.Name, err)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager register the InternalFabricReconciler to the manager.
func (r *NodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Owns(&networkingv1alpha1.InternalNode{}).
		For(&corev1.Node{}).
		Complete(r)
}
