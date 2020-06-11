package main

import (
	"github.com/liqoTech/liqo/pkg/mutate"
	"os"
)

const (
	defaultCertFile = "/etc/ssl/liqo/cert.pem"
	defaultKeyFile  = "/etc/ssl/liqo/key.pem"
)

func setOptions(c *mutate.MutationConfig) {

	if c.KeyFile = os.Getenv("liqokey"); c.KeyFile == "" {
		c.KeyFile = defaultKeyFile
	}

	if c.CertFile = os.Getenv("liqocert"); c.CertFile == "" {
		c.CertFile = defaultCertFile
	}
}
