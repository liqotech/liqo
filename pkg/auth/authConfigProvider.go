package auth

import configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"

// AuthConfigProvider is a provider for the Authentication Configuration
type AuthConfigProvider interface {
	// GetConfig retrieves the AuthConfiguration, such as the peering permission and the token authentication settings
	GetConfig() *configv1alpha1.AuthConfig
	// GetApiServerConfig retrieves the ApiServerConfiguration (i.e. Address, Port and TrustedCA)
	GetApiServerConfig() *configv1alpha1.ApiServerConfig
}
