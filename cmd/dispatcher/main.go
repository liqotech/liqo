package main

import (
	"flag"
	"fmt"
	clusterConfig "github.com/liqoTech/liqo/api/cluster-config/v1"
	discoveryv1 "github.com/liqoTech/liqo/api/discovery/v1"
	"github.com/liqoTech/liqo/internal/dispatcher"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
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
	remoteKubeConfig := flag.String("remoteConfig", "", "path to the kubeConfig of the remote cluster")
	flag.Parse()
	clusterID := "remoteCluster"

	cfg := ctrl.GetConfigOrDie()
	fmt.Println(remoteKubeConfig)
	remoteCfg, err := clientcmd.BuildConfigFromFlags("", *remoteKubeConfig)
	if err != nil {
		panic(err)
	}
	d := &dispatcher.DispatcherReconciler{
		Scheme:           scheme,
		LocalDynClient:   dynamic.NewForConfigOrDie(cfg),
		RemoteDynClients: make(map[string]dynamic.Interface),
		RunningWatchers:  make(map[string]chan bool),
	}
	d.RemoteDynClients[clusterID] = dynamic.NewForConfigOrDie(remoteCfg)
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:         scheme,
		Port:           9443,
		LeaderElection: false,
	})
	if err != nil {
		klog.Error(err, "unable to start manager")
		os.Exit(1)
	}
	if err = d.SetupWithManager(mgr); err != nil {
		klog.Error(err, "unable to setup the dispatcher-operator")
		os.Exit(1)
	}
	err = d.WatchConfiguration(cfg, &clusterConfig.GroupVersion)
	if err != nil {
		klog.Error(err)
		os.Exit(-1)
	}
	klog.Info("Starting dispatcher-operator")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		klog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
