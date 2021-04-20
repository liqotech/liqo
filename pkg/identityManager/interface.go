package identityManager

import (
	"k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
)

type IdentityManager interface {
	localManager
	remoteManager
}

// interface that allows to manage the identity in the owner cluster
type localManager interface {
	CreateIdentity(remoteClusterID string) (*v1.Secret, error)
	GetSigningRequest(remoteClusterID string) ([]byte, error)
	StoreCertificate(remoteClusterID string, certificate []byte) error

	GetConfig(remoteClusterID string, masterUrl string) (*rest.Config, error)
}

// interface that allows to manage the identity in the target cluster, where this identity has to be used
type remoteManager interface {
	ApproveSigningRequest(signingRequest []byte) (certificate []byte, err error)
}
