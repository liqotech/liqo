package schedulingNodeOperator

import (
	"context"
	"github.com/liqoTech/liqo/api/scheduling/v1alpha1"
	advtypes "github.com/liqoTech/liqo/api/sharing/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"strings"
)

// CreateOrUpdateFromNode takes a node and creates a new scheduling Node if the
// corresponding SchedulingNode doesn't exist yet, otherwise, it updates the
// corresponding SchedulingNode CR
func (r *SchedulingNodeReconciler) CreateOrUpdateFromNode(ctx context.Context, node corev1.Node) error {

	var sn v1alpha1.SchedulingNode

	if err := r.Client.Get(ctx, v1alpha1.CreateNamespacedName(node.Name), &sn); err != nil {
		if apierrors.IsNotFound(err) {
			return r.createSchedulingNode(ctx, node)
		} else {
			return err
		}
	} else {
		return r.updateSchedulingNode(ctx, node, &sn)
	}
}

// updateSchedulingNode receives an already deployed schedulingNode and updates it
// according to the received node
func (r *SchedulingNodeReconciler) updateSchedulingNode(ctx context.Context, node corev1.Node, sn *v1alpha1.SchedulingNode) error {

	if err := sn.UpdateFromNode(node); err != nil {
		return err
	}

	if l, ok := node.GetLabels()["type"]; ok && l == "virtual-node" {
		if err := r.setNeighborsFromAdv(sn, ctx, node); err != nil {
			return err
		}
	}

	if err := r.Client.Update(ctx, sn); err != nil {
		return err
	}

	return nil
}

// createSchedulingNode receives a node and creates a new SchedulingNode CR according
// to the node capabilities
func (r *SchedulingNodeReconciler) createSchedulingNode(ctx context.Context, node corev1.Node) error {
	var sn v1alpha1.SchedulingNode

	if err := sn.CreateFromNode(node); err != nil {
		return err
	}

	if l, ok := node.GetLabels()["type"]; ok && l == "virtual-node" {
		if err := r.setNeighborsFromAdv(&sn, ctx, node); err != nil {
			return err
		}
	}

	if err := r.Client.Create(ctx, &sn); err != nil {
		return err
	}

	return nil
}

func (r *SchedulingNodeReconciler) setNeighborsFromAdv(sn *v1alpha1.SchedulingNode, ctx context.Context, node corev1.Node) error {
	var adv advtypes.Advertisement

	namespacedNodeName := types.NamespacedName{
		Name:      strings.Join([]string{"advertisement", node.Annotations["cluster-id"]}, "-"),
		Namespace: "default",
	}

	if err := r.Client.Get(ctx, namespacedNodeName, &adv); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Info("error to be managed, node cannot exist")
			return nil
		} else {
			return err
		}
	}

	if adv.Spec.Neighbors == nil {
		return nil
	}

	if sn.Spec.Neighbors == nil {
		sn.Spec.Neighbors = make(map[corev1.ResourceName]corev1.ResourceList)
	}

	for k, v := range adv.Spec.Neighbors {
		sn.Spec.Neighbors[k] = v
	}

	return nil
}
