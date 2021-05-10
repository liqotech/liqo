package auth

// IdentityRequest is the common interface for Certificate and SearviceAccount identity request.
type IdentityRequest interface {
	GetClusterID() string
	GetToken() string
}

// ServiceAccountIdentityRequest is the request for a new ServiceAccount validation.
type ServiceAccountIdentityRequest struct {
	ClusterID string `json:"clusterID"`
	Token     string `json:"token"`
}

// CertificateIdentityRequest is the request for a new certificate validation.
type CertificateIdentityRequest struct {
	ClusterID                 string `json:"clusterID"`
	Token                     string `json:"token"`
	CertificateSigningRequest string `json:"certificateSigningRequest"`
}

// GetClusterID returns the clusterid.
func (saIdentityRequest *ServiceAccountIdentityRequest) GetClusterID() string {
	return saIdentityRequest.ClusterID
}

// GetToken returns the token.
func (saIdentityRequest *ServiceAccountIdentityRequest) GetToken() string {
	return saIdentityRequest.Token
}

// GetClusterID returns the clusterid.
func (certIdentityRequest *CertificateIdentityRequest) GetClusterID() string {
	return certIdentityRequest.ClusterID
}

// GetToken returns the token.
func (certIdentityRequest *CertificateIdentityRequest) GetToken() string {
	return certIdentityRequest.Token
}
