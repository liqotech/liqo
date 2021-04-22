package auth

// common interface for Certificate and SearviceAccount identity request
type IdentityRequest interface {
	GetClusterID() string
	GetToken() string
}

type ServiceAccountIdentityRequest struct {
	ClusterID string `json:"clusterID"`
	Token     string `json:"token"`
}

type CertificateIdentityRequest struct {
	ClusterID                 string `json:"clusterID"`
	Token                     string `json:"token"`
	CertificateSigningRequest string `json:"certificateSigningRequest"`
}

func (saIdentityRequest *ServiceAccountIdentityRequest) GetClusterID() string {
	return saIdentityRequest.ClusterID
}

func (saIdentityRequest *ServiceAccountIdentityRequest) GetToken() string {
	return saIdentityRequest.Token
}

func (certIdentityRequest *CertificateIdentityRequest) GetClusterID() string {
	return certIdentityRequest.ClusterID
}

func (certIdentityRequest *CertificateIdentityRequest) GetToken() string {
	return certIdentityRequest.Token
}
