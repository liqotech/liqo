package namespacemapctrl

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/clusterid"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
)

// checkRemoteClientPresence creates a new controller-runtime Client for a remote cluster, if it isn't already present
// in RemoteClients Struct of NamespaceMap Controller.
func (r *NamespaceMapReconciler) checkRemoteClientPresence(remoteClusterID string) error {
	if r.RemoteClients == nil {
		r.RemoteClients = map[string]kubernetes.Interface{}
	}

	if _, ok := r.RemoteClients[remoteClusterID]; !ok {
		clusterID := clusterid.NewStaticClusterID(r.LocalClusterID)
		tenantNamespaceManager := tenantnamespace.NewTenantNamespaceManager(r.IdentityManagerClient)
		identityManager := identitymanager.NewCertificateIdentityReader(r.IdentityManagerClient, clusterID, tenantNamespaceManager)
		restConfig, err := identityManager.GetConfig(remoteClusterID, "")
		if err != nil {
			klog.Error(err)
			return err
		}

		if r.RemoteClients[remoteClusterID], err = kubernetes.NewForConfig(restConfig); err != nil {
			klog.Errorf("%s -> unable to create client for cluster '%s'", err, remoteClusterID)
			return err
		}
	}
	return nil
}
