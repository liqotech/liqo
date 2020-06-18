package main

import (
	"flag"
	"github.com/liqoTech/liqo/internal/discovery"
	foreign_cluster_operator "github.com/liqoTech/liqo/internal/discovery/foreign-cluster-operator"
	"github.com/liqoTech/liqo/pkg/clusterID"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
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

	clusterId, err := clusterID.NewClusterID()
	if err != nil {
		mainLog.Error(err, err.Error())
		os.Exit(1)
	}
	err = clusterId.SetupClusterID(namespace)
	if err != nil {
		mainLog.Error(err, err.Error())
		os.Exit(1)
	}

	discoveryCtl, err := discovery.NewDiscoveryCtrl(namespace, clusterId)
	if err != nil {
		mainLog.Error(err, err.Error())
		os.Exit(1)
	}

	discoveryCtl.SetupCaData()
	discoveryCtl.StartDiscovery()

	mainLog.Info("Starting ForeignCluster operator")
	foreign_cluster_operator.StartOperator(namespace)
}
