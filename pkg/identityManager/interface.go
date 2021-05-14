package identitymanager

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
)

// IdentityManager interface provides the methods to manage identities for the remote clusters.
type IdentityManager interface {
	localManager
	remoteManager
}

// interface that allows to manage the identity in the owner cluster.
type localManager interface {
	CreateIdentity(remoteClusterID string) (*v1.Secret, error)
	GetSigningRequest(remoteClusterID string) ([]byte, error)
	StoreCertificate(remoteClusterID string, certificate []byte) error

	GetConfig(remoteClusterID string, masterUrl string) (*rest.Config, error)
}

// interface that allows to manage the identity in the target cluster, where this identity has to be used.
type remoteManager interface {
	ApproveSigningRequest(clusterID string, signingRequest string) (certificate []byte, err error)
	GetRemoteCertificate(clusterID string, signingRequest string) (certificate []byte, err error)
}
