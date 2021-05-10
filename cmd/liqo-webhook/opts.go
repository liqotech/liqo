package main

import (
	"os"

	"github.com/liqotech/liqo/pkg/mutate"
)

const (
	defaultCertFile = "/etc/ssl/liqo/tls.crt"
	defaultKeyFile  = "/etc/ssl/liqo/tls.key"
)

func setOptions(c *mutate.MutationConfig) {
	if c.KeyFile = os.Getenv("LIQO_KEY"); c.KeyFile == "" {
		c.KeyFile = defaultKeyFile
	}

	if c.CertFile = os.Getenv("LIQO_CERT"); c.CertFile == "" {
		c.CertFile = defaultCertFile
	}
}
