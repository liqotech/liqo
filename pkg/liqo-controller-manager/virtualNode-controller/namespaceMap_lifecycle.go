package virtualNode_controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	ctrlutils "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	mapsv1alpha1 "github.com/liqotech/liqo/apis/virtualKubelet/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"

	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *VirtualNodeReconciler) removeAllDesiredMappings(nm mapsv1alpha1.NamespaceMap) error {

	for localName := range nm.Spec.DesiredMapping {
		delete(nm.Spec.DesiredMapping, localName)
	}

	ctrlutils.RemoveFinalizer(&nm, virtualNodeControllerFinalizer)
	klog.Infof("The NamespaceMap '%s' is requested to be deleted", nm.GetName())

	if err := r.Update(context.TODO(), &nm); err != nil {
		klog.Errorf(" %s --> Problems while removing finalizer from '%s'", err, nm.GetName())
		return err
	}
	klog.Infof("Finalizer is correctly removed from the NamespaceMap '%s'", nm.GetName())

	return nil
}

// remove Finalizer and Update the NamespaceMap
func (r *VirtualNodeReconciler) removeNamespaceMapFinalizers(nm mapsv1alpha1.NamespaceMap) error {
	ctrlutils.RemoveFinalizer(&nm, virtualNodeControllerFinalizer)
	ctrlutils.RemoveFinalizer(&nm, liqoconst.NamespaceMapControllerFinalizer)

	klog.Infof("The NamespaceMap '%s' is requested to be deleted", nm.GetName())

	if err := r.Update(context.TODO(), &nm); err != nil {
		// WARNING: Is possible that this Update is called on a resource that is no more here
		// it doesn't return "NotFound" but "Conflict"
		klog.Errorf(" %s --> Problems while removing finalizer from '%s'", err, nm.GetName())
		return err
	}
	klog.Infof("Finalizer is correctly removed from the NamespaceMap '%s'", nm.GetName())

	return nil
}

// create a new NamespaceMap with Finalizer and OwnerReference
func (r *VirtualNodeReconciler) createNamespaceMap(n corev1.Node, stat mapsv1alpha1.NamespaceMapStatus, spec mapsv1alpha1.NamespaceMapSpec) error {

	if _, ok := n.GetAnnotations()[liqoconst.VirtualNodeClusterId]; !ok {
		err := fmt.Errorf("label '%s' is not found on node '%s'", liqoconst.VirtualNodeClusterId, n.GetName())
		klog.Error(err)
		return err
	}

	nm := &mapsv1alpha1.NamespaceMap{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", n.GetAnnotations()[liqoconst.VirtualNodeClusterId]),
			Namespace:    liqoconst.MapNamespaceName,
			Labels: map[string]string{
				liqoconst.VirtualNodeClusterId: n.GetAnnotations()[liqoconst.VirtualNodeClusterId],
			},
		},
		Spec:   spec,
		Status: stat,
	}

	if len(nm.Status.CurrentMapping) > 0 {
		ctrlutils.AddFinalizer(nm, liqoconst.NamespaceMapControllerFinalizer)
	}

	ctrlutils.AddFinalizer(nm, virtualNodeControllerFinalizer)
	if err := ctrlutils.SetControllerReference(&n, nm, r.Scheme); err != nil {
		return err
	}

	if err := r.Create(context.TODO(), nm); err != nil {
		klog.Errorf("%s --> Problems in NamespaceMap creation for the virtual node '%s'", err, n.GetName())
		return err
	}
	klog.Infof(" Create NamespaceMap '%s' for the virtual node '%s'", nm.GetName(), n.GetName())
	return nil
}

// first create a new NamespaceMap which preserves the Status and then delete the other
func (r *VirtualNodeReconciler) regenerateNamespaceMap(nm mapsv1alpha1.NamespaceMap, n corev1.Node) error {

	// create a new namespaceMap with same Status but with different Name
	if err := r.createNamespaceMap(n, nm.Status, nm.Spec); err != nil {
		return err
	}

	if err := r.removeNamespaceMapFinalizers(nm); err != nil {
		return err
	}
	return nil
}

// This function manages NamespaceMaps Lifecycle on the basis of NamespaceMaps' number
func (r *VirtualNodeReconciler) namespaceMapLifecycle(n corev1.Node) error {

	nms := &mapsv1alpha1.NamespaceMapList{}
	if err := r.List(context.TODO(), nms, client.InNamespace(liqoconst.MapNamespaceName),
		client.MatchingLabels{liqoconst.VirtualNodeClusterId: n.GetAnnotations()[liqoconst.VirtualNodeClusterId]}); err != nil {
		klog.Errorf("%s --> Unable to List NamespaceMaps of virtual node '%s'", err, n.GetName())
		return err
	}

	if len(nms.Items) == 0 {
		return r.createNamespaceMap(n, mapsv1alpha1.NamespaceMapStatus{}, mapsv1alpha1.NamespaceMapSpec{})
	}

	if len(nms.Items) == 1 {
		if !nms.Items[0].GetDeletionTimestamp().IsZero() {
			return r.regenerateNamespaceMap(nms.Items[0], n)
		}
		return nil
	}

	if len(nms.Items) > 1 {

		oldestCreation := metav1.Time{Time: time.Now()}
		var oldestMap int

		for i, nm := range nms.Items {
			if nm.CreationTimestamp.Before(&oldestCreation) && nm.DeletionTimestamp.IsZero() {
				oldestCreation = nm.CreationTimestamp
				oldestMap = i
			}
		}

		for i, nm := range nms.Items {
			if i != oldestMap {
				if nm.GetDeletionTimestamp().IsZero() {
					if err := r.Delete(context.TODO(), &nm); err != nil {
						klog.Errorf(" %s --> Unable to remove NamespaceMap '%s'", err, nm.GetName())
						return err
					}
					continue
				}
				if err := r.removeNamespaceMapFinalizers(nm); err != nil {
					return err
				}
			}
		}

	}

	return nil
}
