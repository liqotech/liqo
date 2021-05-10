package main

import (
	"log"

	"k8s.io/klog"

	"github.com/liqotech/liqo/pkg/mutate"
)

func main() {
	config := &mutate.MutationConfig{}

	setOptions(config)

	log.Println("Starting server ...")

	s, err := mutate.NewMutationServer(config)
	if err != nil {
		klog.Fatal(err)
	}

	s.Serve()
}
