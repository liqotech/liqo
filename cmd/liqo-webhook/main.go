package main

import (
	"github.com/liqotech/liqo/pkg/mutate"
	"k8s.io/klog"
	"log"
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
