package foreignclusteroperator

import (
	"context"

	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/apis/discovery/v1alpha1"
)

// ensureLocalTenantNamespace creates the LocalTenantControlNamespace for the given ForeignCluster, if it is not yet present.
func (r *ForeignClusterReconciler) ensureLocalTenantNamespace(
	ctx context.Context, foreignCluster *v1alpha1.ForeignCluster) error {
	namespace, err := r.namespaceManager.CreateNamespace(foreignCluster.Spec.ClusterIdentity.ClusterID)
	if err != nil {
		klog.Error(err)
		return err
	}

	foreignCluster.Status.TenantControlNamespace.Local = namespace.Name
	return nil
}
