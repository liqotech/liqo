package identitymanager

import (
	"context"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"

	"github.com/liqotech/liqo/pkg/clusterid"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/vkMachinery/csr"
)

type identityManager struct {
	IdentityProvider

	client           kubernetes.Interface
	localClusterID   clusterid.ClusterID
	namespaceManager tenantnamespace.Manager

	iamTokenManager tokenManager
}

// NewCertificateIdentityReader gets a new certificate identity reader.
func NewCertificateIdentityReader(client kubernetes.Interface,
	localClusterID clusterid.ClusterID, namespaceManager tenantnamespace.Manager) IdentityReader {
	return NewCertificateIdentityManager(client, localClusterID, namespaceManager)
}

// NewCertificateIdentityManager gets a new certificate identity manager.
func NewCertificateIdentityManager(client kubernetes.Interface,
	localClusterID clusterid.ClusterID, namespaceManager tenantnamespace.Manager) IdentityManager {
	idProvider := &certificateIdentityProvider{
		namespaceManager: namespaceManager,
		client:           client,
	}

	return newIdentityManager(client, localClusterID, namespaceManager, idProvider)
}

// NewCertificateIdentityProvider gets a new certificate identity approver.
func NewCertificateIdentityProvider(ctx context.Context, client kubernetes.Interface,
	localClusterID clusterid.ClusterID, namespaceManager tenantnamespace.Manager) IdentityProvider {
	req, err := labels.NewRequirement(remoteTenantCSRLabel, selection.Exists, []string{})
	utilruntime.Must(err)

	csrWatcher := csr.NewWatcher(client, 0, labels.NewSelector().Add(*req))
	csrWatcher.Start(ctx)
	idProvider := &certificateIdentityProvider{
		namespaceManager: namespaceManager,
		client:           client,
		csrWatcher:       csrWatcher,
	}

	return newIdentityManager(client, localClusterID, namespaceManager, idProvider)
}

// NewIAMIdentityReader gets a new identity reader to handle IAM identities.
func NewIAMIdentityReader(client kubernetes.Interface,
	localClusterID clusterid.ClusterID, awsConfig *AwsConfig,
	namespaceManager tenantnamespace.Manager) IdentityManager {
	return NewIAMIdentityManager(client, localClusterID, awsConfig, namespaceManager)
}

// NewIAMIdentityManager gets a new identity manager to handle IAM identities.
func NewIAMIdentityManager(client kubernetes.Interface,
	localClusterID clusterid.ClusterID, awsConfig *AwsConfig,
	namespaceManager tenantnamespace.Manager) IdentityManager {
	idProvider := &iamIdentityProvider{
		awsConfig: awsConfig,
		client:    client,
	}

	return newIdentityManager(client, localClusterID, namespaceManager, idProvider)
}

// NewIAMIdentityProvider gets a new identity approver to handle IAM identities.
func NewIAMIdentityProvider(client kubernetes.Interface,
	localClusterID clusterid.ClusterID, awsConfig *AwsConfig,
	namespaceManager tenantnamespace.Manager) IdentityProvider {
	idProvider := &iamIdentityProvider{
		awsConfig: awsConfig,
		client:    client,
	}

	return newIdentityManager(client, localClusterID, namespaceManager, idProvider)
}

func newIdentityManager(client kubernetes.Interface,
	localClusterID clusterid.ClusterID,
	namespaceManager tenantnamespace.Manager,
	idProvider IdentityProvider) *identityManager {
	iamTokenManager := &iamTokenManager{
		client:                    client,
		availableClusterIDSecrets: map[string]types.NamespacedName{},
	}
	iamTokenManager.start(context.TODO())

	return &identityManager{
		client:           client,
		localClusterID:   localClusterID,
		namespaceManager: namespaceManager,

		IdentityProvider: idProvider,

		iamTokenManager: iamTokenManager,
	}
}
