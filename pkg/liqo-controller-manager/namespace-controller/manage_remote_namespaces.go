package namespace_controller

import (
	"context"
	mapsv1alpha1 "github.com/liqotech/liqo/apis/virtualKubelet/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Removes right entry from one NamespaceMap
func (r *NamespaceReconciler) removeRemoteNamespace(localName string, nm mapsv1alpha1.NamespaceMap) error {

	if _, ok := nm.Status.NattingTable[localName]; ok {
		delete(nm.Status.NattingTable, localName)
		if err := r.Update(context.TODO(), &nm); err != nil {
			klog.Error(err, " --> Unable to update NamespaceMap")
			return err
		}
		klog.Info(" Entries deleted correctly")
	}

	return nil
}

// Removes right entries from more than one NamespaceMap (it depends on len(nms))
func (r *NamespaceReconciler) removeRemoteNamespaces(localName string, nms map[string]mapsv1alpha1.NamespaceMap) error {

	for _, nm := range nms {
		if err := r.removeRemoteNamespace(localName, nm); err != nil {
			return err
		}
	}
	return nil
}

// Adds right entry on one NamespaceMap, if it isn't already there
func (r *NamespaceReconciler) createRemoteNamespace(n *corev1.Namespace, remoteName string, nm mapsv1alpha1.NamespaceMap) error {

	if nm.Status.NattingTable == nil {
		nm.Status.NattingTable = map[string]string{}
	}

	if oldValue, ok := nm.Status.NattingTable[n.GetName()]; ok {
		// if entries are already here, but mappingLabel has a different value from the previous one, we force again the old value.
		// Common user cannot change remote namespace name while the namespace is offloaded onto remote clusters
		if oldValue != remoteName {
			n.GetLabels()[mappingLabel] = oldValue
			if err := r.Update(context.TODO(), n); err != nil {
				klog.Error(err, " --> Unable to update mapping label")
				return err
			}
		}
	} else {
		nm.Status.NattingTable[n.GetName()] = remoteName
		if err := r.Patch(context.TODO(), &nm, client.Merge); err != nil {
			klog.Error(err, " --> Unable to add entries in NamespaceMap")
			return err
		}
	}
	return nil
}

// Adds right entries on more than one NamespaceMap (it depends on len(nms)), if they aren't already there
func (r *NamespaceReconciler) createRemoteNamespaces(n *corev1.Namespace, remoteName string, nms map[string]mapsv1alpha1.NamespaceMap) error {

	for _, nm := range nms {
		if err := r.createRemoteNamespace(n, remoteName, nm); err != nil {
			return err
		}
	}
	return nil
}
