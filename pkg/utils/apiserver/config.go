package apiserver

import "flag"

// config is the instance of the configuration automatically populated based on the command line parameters.
var config Config

// Config defines the configuration parameters to contact the Kubernetes API server.
type Config struct {
	// The address of the Kubernetes API Server, advertised to the peering clusters.
	// Overrides the IP address of a control plane node, with the default port (6443).
	Address string

	// Whether the Kubernetes API Server is exposed through a trusted certification authority,
	// which does not need to be explicitly advertised.
	TrustedCA bool
}

// InitFlags initializes the flags to configure the API server parameters.
func InitFlags(flagset *flag.FlagSet) {
	if flagset == nil {
		flagset = flag.CommandLine
	}

	flagset.StringVar(&config.Address, "advertise-api-server-address", "",
		"The address of the Kubernetes API Server, advertised to the peering clusters. "+
			"If not set, it is automatically set to the IP address of a control plane node, with the default port (6443)")
	flagset.BoolVar(&config.TrustedCA, "advertise-api-server-trusted-ca", false,
		"Whether the Kubernetes API Server is exposed through a trusted certification authority, which does not need to be explicitly advertised.")
}

// GetConfig returns the API server configuration populated based on the command line parameters.
func GetConfig() Config { return config }
