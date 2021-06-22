package virtualnodectrl

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutils "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	mapsv1alpha1 "github.com/liqotech/liqo/apis/virtualKubelet/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

// createNamespaceMap creates a new NamespaceMap with OwnerReference.
func (r *VirtualNodeReconciler) createNamespaceMap(ctx context.Context, n *corev1.Node) error {
	nm := &mapsv1alpha1.NamespaceMap{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", n.GetAnnotations()[liqoconst.RemoteClusterID]),
			Namespace:    liqoconst.TechnicalNamespace,
			Labels: map[string]string{
				liqoconst.RemoteClusterID: n.GetAnnotations()[liqoconst.RemoteClusterID],
			},
		},
	}

	if err := ctrlutils.SetControllerReference(n, nm, r.Scheme); err != nil {
		return err
	}

	if err := r.Create(ctx, nm); err != nil {
		klog.Errorf("%s --> Problems in NamespaceMap creation for the virtual node '%s'", err, n.GetName())
		return err
	}
	klog.Infof(" Create the NamespaceMap '%s' for the virtual node '%s'", nm.GetName(), n.GetName())
	return nil
}

// ensureNamespaceMapPresence creates a new NamespaceMap associated with that virtual-node if it is not already present.
func (r *VirtualNodeReconciler) ensureNamespaceMapPresence(ctx context.Context, n *corev1.Node) error {
	nms := &mapsv1alpha1.NamespaceMapList{}
	if err := r.List(ctx, nms, client.InNamespace(liqoconst.TechnicalNamespace),
		client.MatchingLabels{liqoconst.RemoteClusterID: n.GetAnnotations()[liqoconst.RemoteClusterID]}); err != nil {
		klog.Errorf("%s --> Unable to List NamespaceMaps of virtual-node '%s'", err, n.GetName())
		return err
	}

	if len(nms.Items) == 0 {
		return r.createNamespaceMap(ctx, n)
	}

	return nil
}
