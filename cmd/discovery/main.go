package main

import (
	"github.com/netgroup-polito/dronev2/internal/discovery"
	foreign_cluster_operator "github.com/netgroup-polito/dronev2/internal/discovery/foreign-cluster-operator"
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

	log.Println("Starting ForeignCluster operator")
	go foreign_cluster_operator.StartOperator()

	<-sig
}
