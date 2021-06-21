package virtualnodectrl

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mapsv1alpha1 "github.com/liqotech/liqo/apis/virtualKubelet/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

// removeAssociatedNamespaceMaps forces the deletion of virtual-node's NamespaceMaps before deleting it.
func (r *VirtualNodeReconciler) removeAssociatedNamespaceMaps(ctx context.Context, n *corev1.Node) error {
	klog.Infof("The virtual virtualNode '%s' is requested to be deleted", n.GetName())

	// The deletion timestamp is automatically set on the NamespaceMaps associated with the virtual-node,
	// it's only necessary to wait until the NamespaceMaps are deleted.
	namespaceMapList := &mapsv1alpha1.NamespaceMapList{}
	virtualNodeClusterID := n.Annotations[liqoconst.RemoteClusterID]
	if err := r.List(ctx, namespaceMapList,
		client.InNamespace(r.getLocalTenantNamespaceName(virtualNodeClusterID)),
		client.MatchingLabels{liqoconst.RemoteClusterID: virtualNodeClusterID}); err != nil {
		klog.Errorf("%s --> Unable to List NamespaceMaps of virtual virtualNode '%s'", err, n.GetName())
		return err
	}

	if len(namespaceMapList.Items) == 0 {
		delete(r.LocalTenantNamespacesNames, virtualNodeClusterID)
		return r.removeVirtualNodeFinalizer(ctx, n)
	}

	for i := range namespaceMapList.Items {
		if namespaceMapList.Items[i].GetDeletionTimestamp().IsZero() {
			if err := r.Delete(ctx, &namespaceMapList.Items[i]); err != nil {
				klog.Errorf("%s -> unable to delete the NamespaceMap '%s'", err, namespaceMapList.Items[i].Name)
			}
		}
	}

	err := fmt.Errorf("waiting for deletion of NamespaceMaps associated with the virtual-node '%s'", n.Name)
	klog.Error(err)
	return err
}
