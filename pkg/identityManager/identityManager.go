package identityManager

import (
	"github.com/liqotech/liqo/pkg/clusterID"
	"github.com/liqotech/liqo/pkg/tenantControlNamespace"
	"k8s.io/client-go/kubernetes"
)

type certificateIdentityManager struct {
	client           kubernetes.Interface
	localClusterID   clusterID.ClusterID
	namespaceManager tenantControlNamespace.TenantControlNamespaceManager
}

// get a new certificate identity manager
func NewCertificateIdentityManager(client kubernetes.Interface, localClusterID clusterID.ClusterID, namespaceManager tenantControlNamespace.TenantControlNamespaceManager) IdentityManager {
	return &certificateIdentityManager{
		client:           client,
		localClusterID:   localClusterID,
		namespaceManager: namespaceManager,
	}
}
