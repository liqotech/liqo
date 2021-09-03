package auth

import configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"

// ConfigProvider is a provider for the Authentication Configuration.
type ConfigProvider interface {
	// GetAPIServerConfig retrieves the ApiServerConfiguration (i.e. Address, Port and TrustedCA).
	GetAPIServerConfig() *configv1alpha1.APIServerConfig
}
