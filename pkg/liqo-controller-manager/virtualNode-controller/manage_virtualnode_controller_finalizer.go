package virtualnodectrl

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutils "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// ensureVirtualNodeFinalizerPresence adds the virtualNodeControllerFinalizer if it is not already there.
func (r *VirtualNodeReconciler) ensureVirtualNodeFinalizerPresence(ctx context.Context, n *corev1.Node) error {
	if !ctrlutils.ContainsFinalizer(n, virtualNodeControllerFinalizer) {
		original := n.DeepCopy()
		ctrlutils.AddFinalizer(n, virtualNodeControllerFinalizer)
		if err := r.Patch(ctx, n, client.MergeFrom(original)); err != nil {
			klog.Errorf("%s --> Unable to add finalizer to the virtual-node '%s'", err, n.GetName())
			return err
		}
		klog.Infof("Finalizer correctly added on the virtual-node '%s'", n.GetName())
	}
	return nil
}

// removeVirtualNodeFinalizer removes the virtualNodeControllerFinalizer.
func (r *VirtualNodeReconciler) removeVirtualNodeFinalizer(ctx context.Context, n *corev1.Node) error {
	original := n.DeepCopy()
	ctrlutils.RemoveFinalizer(n, virtualNodeControllerFinalizer)
	if err := r.Patch(ctx, n, client.MergeFrom(original)); err != nil {
		klog.Errorf("%s --> Unable to remove finalizer from the virtual-node '%s'", err, n.GetName())
		return err
	}
	klog.Infof("Finalizer is correctly removed from the virtual-node '%s'", n.GetName())
	return nil
}
