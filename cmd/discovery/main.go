package main

import (
	"flag"
	"github.com/netgroup-polito/dronev2/internal/discovery"
	foreign_cluster_operator "github.com/netgroup-polito/dronev2/internal/discovery/foreign-cluster-operator"
	"os"
	"os/signal"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"syscall"
)

var (
	mainLog = ctrl.Log.WithName("main")
)

func main() {
	mainLog.Info("Starting")

	var namespace string

	flag.StringVar(&namespace, "namespace", "default", "Namespace where your configs are stored.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	signal.Notify(sig, os.Interrupt, syscall.SIGINT)

	discovery.StartDiscovery(namespace)

	mainLog.Info("Starting ForeignCluster operator")
	go foreign_cluster_operator.StartOperator()

	<-sig
}
