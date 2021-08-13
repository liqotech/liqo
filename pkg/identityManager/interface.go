package identitymanager

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"

	"github.com/liqotech/liqo/pkg/auth"
	responsetypes "github.com/liqotech/liqo/pkg/identityManager/responseTypes"
)

// IdentityReader provides the interface to retrieve the identities for the remote clusters.
type IdentityReader interface {
	GetConfig(remoteClusterID string, namespace string) (*rest.Config, error)
	GetRemoteTenantNamespace(remoteClusterID string, namespace string) (string, error)
}

// IdentityManager interface provides the methods to manage identities for the remote clusters.
type IdentityManager interface {
	IdentityReader

	CreateIdentity(remoteClusterID string) (*v1.Secret, error)
	GetSigningRequest(remoteClusterID string) ([]byte, error)
	StoreCertificate(remoteClusterID string, identityResponse *auth.CertificateIdentityResponse) error
}

// IdentityProvider provides the interface to retrieve and approve remote cluster identities.
type IdentityProvider interface {
	GetRemoteCertificate(clusterID, namespace, signingRequest string) (response *responsetypes.SigningRequestResponse, err error)
	ApproveSigningRequest(clusterID, signingRequest string) (response *responsetypes.SigningRequestResponse, err error)
}
