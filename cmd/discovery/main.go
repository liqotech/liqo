package main

import (
	"github.com/netgroup-polito/dronev2/internal/discovery"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	log.Println("Starting")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	signal.Notify(sig, os.Interrupt, syscall.SIGINT)

	discovery.StartDiscovery()

	<-sig
}
