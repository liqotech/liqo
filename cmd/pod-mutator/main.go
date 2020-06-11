package main

import (
	"flag"
	"github.com/joho/godotenv"
	"github.com/liqoTech/liqo/pkg/mutate"
	"k8s.io/klog"
	"log"
)

const (
	inputFile = "/etc/environment/liqo/env"
)

func main() {
	config := &mutate.MutationConfig{}

	var inputEnvFile string

	flag.StringVar(&inputEnvFile, "input-env-file", inputFile, "The environment variable file to source at startup")
	flag.Parse()

	if err := godotenv.Load(inputEnvFile); err != nil {
		klog.Fatal("The env variable file hasn't been correctly loaded")
	}
	setOptions(config)

	log.Println("Starting server ...")

	s, err := mutate.NewMutationServer(config)
	if err != nil {
		klog.Fatal(err)
	}

	s.Serve()
}
