package foreignclusteroperator

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog"

	"github.com/liqotech/liqo/apis/discovery/v1alpha1"
)

// createLocalTenantControlNamespace creates the LocalTenantControlNamespace for the given ForeignCluster, if it is not yet present.
// It returns a boolean indicating if the ForeignCluster requires an update.
func (r *ForeignClusterReconciler) createLocalTenantControlNamespace(foreignCluster *v1alpha1.ForeignCluster) (requireUpdate bool, err error) {
	var namespace *v1.Namespace
	namespace, err = r.namespaceManager.CreateNamespace(foreignCluster.Spec.ClusterIdentity.ClusterID)
	if err != nil {
		klog.Error(err)
		return false, err
	}

	requireUpdate = false
	if foreignCluster.Status.TenantControlNamespace.Local != namespace.Name {
		foreignCluster.Status.TenantControlNamespace.Local = namespace.Name
		requireUpdate = true
	}

	return requireUpdate, nil
}
