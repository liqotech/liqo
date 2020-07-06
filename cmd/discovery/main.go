package main

import (
	"flag"
	discoveryv1 "github.com/liqoTech/liqo/api/discovery/v1"
	"github.com/liqoTech/liqo/internal/discovery"
	foreign_cluster_operator "github.com/liqoTech/liqo/internal/discovery/foreign-cluster-operator"
	search_domain_operator "github.com/liqoTech/liqo/internal/discovery/search-domain-operator"
	"github.com/liqoTech/liqo/pkg/clusterID"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog"
	"os"
	"time"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = discoveryv1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	klog.Info("Starting")

	var namespace string
	var requeueAfter int64 // seconds

	flag.StringVar(&namespace, "namespace", "default", "Namespace where your configs are stored.")
	flag.Int64Var(&requeueAfter, "requeueAfter", 30, "Period after that PeeringRequests status is rechecked (seconds)")
	flag.Parse()

	clusterId, err := clusterID.NewClusterID()
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}
	err = clusterId.SetupClusterID(namespace)
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}

	discoveryCtl, err := discovery.NewDiscoveryCtrl(namespace, clusterId)
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}

	discoveryCtl.SetupCaData()
	discoveryCtl.StartDiscovery()

	klog.Info("Starting SearchDomain operator")
	go search_domain_operator.StartOperator(time.Duration(requeueAfter)*time.Second, discoveryCtl)

	klog.Info("Starting ForeignCluster operator")
	foreign_cluster_operator.StartOperator(namespace, time.Duration(requeueAfter)*time.Second, discoveryCtl)
}
