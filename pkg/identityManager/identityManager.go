package identitymanager

import (
	"k8s.io/client-go/kubernetes"

	"github.com/liqotech/liqo/pkg/clusterid"
	tenantcontrolnamespace "github.com/liqotech/liqo/pkg/tenantControlNamespace"
)

type certificateIdentityManager struct {
	client           kubernetes.Interface
	localClusterID   clusterid.ClusterID
	namespaceManager tenantcontrolnamespace.TenantControlNamespaceManager
}

// NewCertificateIdentityManager gets a new certificate identity manager.
func NewCertificateIdentityManager(client kubernetes.Interface,
	localClusterID clusterid.ClusterID,
	namespaceManager tenantcontrolnamespace.TenantControlNamespaceManager) IdentityManager {
	return &certificateIdentityManager{
		client:           client,
		localClusterID:   localClusterID,
		namespaceManager: namespaceManager,
	}
}
