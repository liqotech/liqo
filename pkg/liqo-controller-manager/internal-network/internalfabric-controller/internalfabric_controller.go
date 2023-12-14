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
	"slices"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/internal-network/ipam"
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

	// List all internalnodes
	var internalNodes networkingv1alpha1.InternalNodeList
	if err = r.List(ctx, &internalNodes, client.InNamespace(internalFabric.Namespace),
		client.MatchingLabels{consts.InternalFabricLabelKey: req.Name}); err != nil {
		klog.Errorf("Unable to list InternalNodes: %s", err)
		return ctrl.Result{}, err
	}

	// Get the list of physical nodes names, storing only the ones that are not being deleted.
	nodeNames := make([]string, len(nodes.Items))
	for i := range nodes.Items {
		if nodes.Items[i].GetDeletionTimestamp().IsZero() {
			nodeNames[i] = nodes.Items[i].Name
		}
	}

	// Ensure status
	internalFabric.Status.AssignedIPs = make(map[string]networkingv1alpha1.IP)
	for i := range internalNodes.Items {
		internalNode := &internalNodes.Items[i]
		// Delete InternalNode if the associated physical node is deleted/deleting.
		if !slices.Contains(nodeNames, internalNode.Name) {
			if err := r.deleteInternalNode(ctx, internalNode); err != nil {
				klog.Errorf("Unable to delete InternalNode %q: %s", internalNode.Name, err)
				return ctrl.Result{}, err
			}
		} else {
			internalFabric.Status.AssignedIPs[internalNode.Name] = internalNode.Spec.IP
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
	internalNode := &networkingv1alpha1.InternalNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      node.Name,
			Namespace: internalFabric.Namespace,
		},
	}

	if !node.GetDeletionTimestamp().IsZero() {
		// Do not create/update internalNode if the associated physical node is being deleted.
		// Note: deletion of the internalNode will be handled after.
		return nil
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, internalNode, func() error {
		if internalNode.Labels == nil {
			internalNode.Labels = make(map[string]string)
		}
		internalNode.Labels[consts.InternalFabricLabelKey] = internalFabric.Name

		internalNode.Spec.FabricRef = &corev1.ObjectReference{
			Name:      internalFabric.Name,
			Namespace: internalFabric.Namespace,
			UID:       internalFabric.UID,
		}

		intIPAM, err := ipam.GetNodeIpam(ctx, r.Client)
		if err != nil {
			return err
		}

		ip, err := intIPAM.Allocate(fmt.Sprintf("%s/%s/%s",
			internalFabric.Namespace, internalFabric.Name, internalNode.Name))
		if err != nil {
			return err
		}

		internalNode.Spec.IP = networkingv1alpha1.IP(ip.String())
		internalNode.Spec.PodCIDR = networkingv1alpha1.CIDR(node.Spec.PodCIDR)
		internalNode.Spec.IsGateway = node.Name == internalFabric.Spec.NodeName

		return controllerutil.SetControllerReference(internalFabric, internalNode, r.Scheme)
	}); err != nil {
		klog.Errorf("Unable to create or update InternalNode %q: %s", internalNode.Name, err)
		return err
	}
	return nil
}

func (r *InternalFabricReconciler) deleteInternalNode(ctx context.Context, internalNode *networkingv1alpha1.InternalNode) error {
	if err := r.Delete(ctx, internalNode); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("InternalNode %q not found", internalNode.Name)
			return nil
		}
		klog.Errorf("Unable to delete InternalNode %q: %s", internalNode.Name, err)
		return err
	}
	klog.Infof("InternalNode %q deleted as associated physical node does not exists anymore", internalNode.Name)
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
		Owns(&networkingv1alpha1.InternalNode{}).
		For(&networkingv1alpha1.InternalFabric{}).
		Complete(r)
}
