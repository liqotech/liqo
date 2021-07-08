package identitymanager

import (
	"k8s.io/client-go/kubernetes"

	"github.com/liqotech/liqo/pkg/clusterid"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
)

type certificateIdentityManager struct {
	client           kubernetes.Interface
	localClusterID   clusterid.ClusterID
	namespaceManager tenantnamespace.Manager
}

// NewCertificateIdentityManager gets a new certificate identity manager.
func NewCertificateIdentityManager(client kubernetes.Interface,
	localClusterID clusterid.ClusterID,
	namespaceManager tenantnamespace.Manager) IdentityManager {
	return &certificateIdentityManager{
		client:           client,
		localClusterID:   localClusterID,
		namespaceManager: namespaceManager,
	}
}
