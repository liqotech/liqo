package main

import (
	"flag"
	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/internal/crdReplicator"
	util "github.com/liqotech/liqo/pkg/liqonet"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	scheme           = runtime.NewScheme()
	clusterIDConfMap = "cluster-id"
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = discoveryv1alpha1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	flag.Parse()
	cfg := ctrl.GetConfigOrDie()
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:         scheme,
		Port:           9443,
		LeaderElection: false,
	})
	if err != nil {
		klog.Error(err, "unable to start manager")
		os.Exit(-1)
	}
	//create a clientSet
	k8sClient := kubernetes.NewForConfigOrDie(cfg)
	//get namespace where the operator is running
	namespaceName, found := os.LookupEnv("NAMESPACE")
	if !found {
		klog.Errorf("namespace env variable not set, please set it in manifest file of the operator")
		os.Exit(-1)
	}
	clusterID, err := util.GetClusterID(k8sClient, clusterIDConfMap, namespaceName)
	if err != nil {
		klog.Errorf("an error occurred while retrieving the clusterID: %s", err)
		os.Exit(-1)
	} else {
		klog.Infof("setting local clusterID to: %s", clusterID)
	}
	dynClient := dynamic.NewForConfigOrDie(cfg)
	dynFac := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dynClient, crdReplicator.ResyncPeriod, metav1.NamespaceAll, crdReplicator.SetLabelsForLocalResources)
	d := &crdReplicator.CRDReplicatorReconciler{
		Scheme:                         mgr.GetScheme(),
		Client:                         mgr.GetClient(),
		ClientSet:                      k8sClient,
		ClusterID:                      clusterID,
		RemoteDynClients:               make(map[string]dynamic.Interface),
		LocalDynClient:                 dynClient,
		LocalDynSharedInformerFactory:  dynFac,
		RegisteredResources:            nil,
		UnregisteredResources:          nil,
		LocalWatchers:                  make(map[string]map[string]chan struct{}),
		RemoteWatchers:                 make(map[string]map[string]chan struct{}),
		RemoteDynSharedInformerFactory: make(map[string]dynamicinformer.DynamicSharedInformerFactory),
	}
	if err = d.SetupWithManager(mgr); err != nil {
		klog.Error(err, "unable to setup the crdreplicator-operator")
		os.Exit(1)
	}
	err = d.WatchConfiguration(cfg, &configv1alpha1.GroupVersion)
	if err != nil {
		klog.Error(err)
		os.Exit(-1)
	}
	klog.Info("Starting crdreplicator-operator")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		klog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
