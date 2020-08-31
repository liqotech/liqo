package main

import (
	"flag"
	discoveryv1alpha1 "github.com/liqoTech/liqo/api/discovery/v1alpha1"
	advtypes "github.com/liqoTech/liqo/api/sharing/v1alpha1"
	"github.com/liqoTech/liqo/internal/discovery"
	foreign_cluster_operator "github.com/liqoTech/liqo/internal/discovery/foreign-cluster-operator"
	search_domain_operator "github.com/liqoTech/liqo/internal/discovery/search-domain-operator"
	"github.com/liqoTech/liqo/pkg/clusterID"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog"
	"os"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"time"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = discoveryv1alpha1.AddToScheme(scheme)
	_ = advtypes.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	klog.Info("Starting")

	var namespace string
	var requeueAfter int64 // seconds
	var kubeconfigPath string

	flag.StringVar(&namespace, "namespace", "default", "Namespace where your configs are stored.")
	flag.Int64Var(&requeueAfter, "requeueAfter", 30, "Period after that PeeringRequests status is rechecked (seconds)")
	flag.StringVar(&kubeconfigPath, "kubeconfigPath", filepath.Join(os.Getenv("HOME"), ".kube", "config"), "For debug purpose, set path to local kubeconfig")
	flag.Parse()

	klog.Info("Namespace: ", namespace)
	klog.Info("RequeueAfter: ", requeueAfter)

	clusterId, err := clusterID.NewClusterID(kubeconfigPath)
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}
	err = clusterId.SetupClusterID(namespace)
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}

	discoveryCtl, err := discovery.NewDiscoveryCtrl(namespace, clusterId, kubeconfigPath)
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}

	err = discoveryCtl.SetupCaData()
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}

	discoveryCtl.StartDiscovery()

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:           scheme,
		Port:             9443,
		LeaderElection:   false,
		LeaderElectionID: "b3156c4e.liqo.io",
	})
	if err != nil {
		klog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	klog.Info("Starting SearchDomain operator")
	search_domain_operator.StartOperator(&mgr, time.Duration(requeueAfter)*time.Second, discoveryCtl, kubeconfigPath)

	klog.Info("Starting ForeignCluster operator")
	foreign_cluster_operator.StartOperator(&mgr, namespace, time.Duration(requeueAfter)*time.Second, discoveryCtl, kubeconfigPath)

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		klog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
