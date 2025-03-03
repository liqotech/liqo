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

package nodecontroller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	internalnetwork "github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/internal-network"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/internal-network/fabricipam"
	"github.com/liqotech/liqo/pkg/utils/getters"
	"github.com/liqotech/liqo/pkg/utils/resource"
)

// NodeReconciler creates and manages InternalNodes.
type NodeReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	liqoNamespace string
}

// NewNodeReconciler returns a new NodeReconciler.
func NewNodeReconciler(cl client.Client, s *runtime.Scheme, liqoNamespace string) *NodeReconciler {
	return &NodeReconciler{
		Client: cl,
		Scheme: s,

		liqoNamespace: liqoNamespace,
	}
}

// cluster-role
// +kubebuilder:rbac:groups=networking.liqo.io,resources=internalnodes,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=nodes/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;

// Reconcile manage Node lifecycle.
func (r *NodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	node := &corev1.Node{}
	internalNode := &networkingv1beta1.InternalNode{
		ObjectMeta: metav1.ObjectMeta{
			Name: req.Name,
		},
	}

	if err = r.Get(ctx, req.NamespacedName, node); client.IgnoreNotFound(err) != nil {
		klog.Errorf("Unable to get the Node %q: %s", req.Name, err)
		return ctrl.Result{}, err
	} else if apierrors.IsNotFound(err) || !node.DeletionTimestamp.IsZero() {
		// If node has been deleted we need to remove the InternalNode resource
		klog.Infof("Deleting InternalNode %v as there is no corresponding Node resource", req.Name)

		if err := r.Client.Delete(ctx, internalNode); err != nil {
			return ctrl.Result{}, fmt.Errorf("unable to delete InternalNode %v: %w", req.Name, err)
		}

		return ctrl.Result{}, nil
	}

	cmDep, err := getters.GetControllerManagerDeployment(ctx, r.Client, r.liqoNamespace)
	if err != nil {
		klog.Errorf("Unable to get the ControllerManager deployment: %s", err)
		return ctrl.Result{}, err
	}
	if cmDep.Annotations != nil && cmDep.Annotations[consts.UninstallingAnnotationKey] == consts.UninstallingAnnotationValue {
		klog.V(4).Infof("Liqo is being uninstalled, skipping the Node %q reconciliation", node.Name)
		return ctrl.Result{}, nil
	}

	ipam, err := fabricipam.Get(ctx, r.Client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to initialize the IPAM: %w", err)
	}

	if _, err = resource.CreateOrUpdate(ctx, r.Client, internalNode, func() error {
		if internalNode.Spec.Interface.Gateway.Name, err = internalnetwork.FindFreeInterfaceName(ctx, r.Client, internalNode); err != nil {
			return err
		}

		ip, err := ipam.Allocate(internalNode.GetName())
		if err != nil {
			return err
		}
		internalNode.Spec.Interface.Node.IP = networkingv1beta1.IP(ip.String())

		return nil
	}); err != nil {
		klog.Errorf("Unable to create or update InternalNode %q: %s", internalNode.Name, err)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager register the InternalFabricReconciler to the manager.
func (r *NodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	filterByLabelsPredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{MatchLabels: map[string]string{
		consts.TypeLabel: consts.TypeNode,
	}})

	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlNode).
		Owns(&networkingv1beta1.InternalNode{}).
		// We need to reconcile only physical Nodes as we need to apply the networking rules for each of them.
		For(&corev1.Node{}, builder.WithPredicates(predicate.Not(filterByLabelsPredicate))).
		Complete(r)
}

// SyncInternalNodes makes sure that at controller startup there are no "orphans" InternalNode, so without corresponding Node.
func SyncInternalNodes(ctx context.Context, c client.Client) error {
	// Check whether there is the corresponding Node for the given InternalNode
	var internalNodeList networkingv1beta1.InternalNodeList
	if err := c.List(ctx, &internalNodeList, &client.ListOptions{}); err != nil {
		return fmt.Errorf("unable to list InternalNode resources: %w", err)
	}

	for i := range internalNodeList.Items {
		internalNode := &internalNodeList.Items[i]
		var ownerNode corev1.Node

		internalNodeName := internalNode.GetName()
		err := c.Get(ctx, types.NamespacedName{Name: internalNodeName}, &ownerNode)
		switch {
		case apierrors.IsNotFound(err):
			// Delete the internal node as there is no corresponding node
			klog.Infof("Deleting InternalNode %v as there is no corresponding Node resource", internalNodeName)
			if err := c.Delete(ctx, internalNode); err != nil {
				return fmt.Errorf("unable to delete InternalNode %v: %w", internalNodeName, err)
			}
		case err != nil:
			return fmt.Errorf("unable to get corresponding Node for InternalNode %v: %w", internalNodeName, err)
		}
	}

	return nil
}
