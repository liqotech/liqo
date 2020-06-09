package main

import "github.com/liqoTech/liqo/pkg/mutate"

const (
	defaultNamespace  = "default"
	defaultSecretName = "pod-mutator-secret"
	defaultCertFile   = "/etc/ssl/liqo/cert.pem"
	defaultKeyFile    = "/etc/ssl/liqo/key.pem"
)

func setOptions(c *mutate.MutationConfig) {
	if c.SecretNamespace == "" {
		c.SecretNamespace = defaultNamespace
	}

	if c.SecretName == "" {
		c.SecretName = defaultSecretName
	}

	if c.KeyFile == "" {
		c.KeyFile = defaultKeyFile
	}

	if c.CertFile == "" {
		c.CertFile = defaultCertFile
	}
}
