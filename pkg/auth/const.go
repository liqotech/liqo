package auth

const (
	// IdsURI is the path where to contact the Authentication Service to get the clusterID.
	IdsURI = "/ids"
	// IdentityURI is the path where to contact the Authentication Service
	// to have a ServiceAccont Identity.
	IdentityURI = "/identity"
	// CertIdentityURI is the path where to contact the Authentication Service
	// to have a Certificate Identity.
	CertIdentityURI = "/identity/certificate"
)
