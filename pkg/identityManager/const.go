package identityManager

const keyLength = 2048

const defaultOrganization = "Liqo"

const (
	localIdentitySecretLabel  = "discovery.liqo.io/local-identity"
	randomIDLabel             = "discovery.liqo.io/random-id"
	certificateAvailableLabel = "discovery.liqo.io/certificate-available"
)

const (
	certificateExpireTimeAnnotation = "discovery.liqo.io/certificate-expire-time"
)

const (
	identitySecretRoot   = "liqo-identity"
	privateKeySecretKey  = "private-key"
	csrSecretKey         = "csr"
	certificateSecretKey = "certificate"
)
