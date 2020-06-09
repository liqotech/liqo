package main

import (
	"flag"
	"github.com/liqoTech/liqo/pkg/mutate"
	"k8s.io/klog"
	"log"
)

func main() {
	config := &mutate.MutationConfig{}

	flag.StringVar(&config.SecretNamespace, "secret-namespace", "", "The namespace in which the secret has been created")
	flag.StringVar(&config.SecretName, "secret-name", "", "The name of the secret to fetch")
	flag.StringVar(&config.CertFile, "cert-file", "", "The local path in which to copy the certificate")
	flag.StringVar(&config.KeyFile, "key-file", "", "The local path in which to copy the key")
	flag.Parse()

	setOptions(config)

	log.Println("Starting server ...")

	s, err := mutate.NewMutationServer(config)
	if err != nil {
		klog.Fatal(err)
	}

	s.Serve()
}
