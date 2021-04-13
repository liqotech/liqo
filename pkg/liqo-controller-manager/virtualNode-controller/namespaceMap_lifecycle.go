package virtualNode_controller

import (
	"context"
	"fmt"
	mapsv1alpha1 "github.com/liqotech/liqo/apis/virtualKubelet/v1alpha1"
	constctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	ctrlutils "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

// remove Finalizer and Update the NamespaceMap
func (r *VirtualNodeReconciler) removeNamespaceMapFinalizer(nm mapsv1alpha1.NamespaceMap) error {
	ctrlutils.RemoveFinalizer(&nm, namespaceMapFinalizer)
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
func (r *VirtualNodeReconciler) createNamespaceMap(n corev1.Node, s mapsv1alpha1.NamespaceMapStatus) error {
	nm := &mapsv1alpha1.NamespaceMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "namespaceresources.liqo.io/v1",
			Kind:       "NamespaceMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", n.GetAnnotations()[constctrl.VirtualNodeClusterId]),
			Namespace:    constctrl.MapNamespaceName,
			Labels: map[string]string{
				constctrl.VirtualNodeClusterId: n.GetAnnotations()[constctrl.VirtualNodeClusterId],
			},
		},
		Status: s,
	}

	ctrlutils.AddFinalizer(nm, namespaceMapFinalizer)
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
	if err := r.createNamespaceMap(n, nm.Status); err != nil {
		return err
	}

	if err := r.removeNamespaceMapFinalizer(nm); err != nil {
		return err
	}
	return nil
}

// This function manages NamespaceMaps Lifecycle on the basis of NamespaceMaps' number
func (r *VirtualNodeReconciler) namespaceMapLifecycle(n corev1.Node) error {

	nms := &mapsv1alpha1.NamespaceMapList{}
	if err := r.List(context.TODO(), nms, client.InNamespace(constctrl.MapNamespaceName),
		client.MatchingLabels{constctrl.VirtualNodeClusterId: n.GetAnnotations()[constctrl.VirtualNodeClusterId]}); err != nil {
		klog.Errorf("%s --> Unable to List NamespaceMaps of virtual node '%s'", err, n.GetName())
		return err
	}

	if len(nms.Items) == 0 {
		return r.createNamespaceMap(n, mapsv1alpha1.NamespaceMapStatus{})
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
				if err := r.removeNamespaceMapFinalizer(nm); err != nil {
					return err
				}
			}
		}

	}

	return nil
}
