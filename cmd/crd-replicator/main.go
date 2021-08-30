package main

import (
	"flag"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"

	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	crdreplicator "github.com/liqotech/liqo/internal/crdReplicator"
	"github.com/liqotech/liqo/pkg/clusterid"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	"github.com/liqotech/liqo/pkg/liqonet/utils"
	"github.com/liqotech/liqo/pkg/mapperUtils"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
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
	restcfg.InitFlags(nil)
	klog.InitFlags(nil)

	flag.Parse()

	cfg := restcfg.SetRateLimiter(ctrl.GetConfigOrDie())
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		MapperProvider: mapperUtils.LiqoMapperProvider(scheme),
		Scheme:         scheme,
		Port:           9443,
		LeaderElection: false,
	})
	if err != nil {
		klog.Error(err, "unable to start manager")
		os.Exit(-1)
	}
	// Create a clientSet.
	k8sClient := kubernetes.NewForConfigOrDie(cfg)
	// Get the namespace where the operator is running.
	namespaceName, found := os.LookupEnv("NAMESPACE")
	if !found {
		klog.Errorf("namespace env variable not set, please set it in manifest file of the operator")
		os.Exit(-1)
	}

	// 7 attempts with 30 seconds sleep between one another
	// for a total of 3 minutes
	backoff := wait.Backoff{
		Steps:    7,
		Duration: 30 * time.Second,
		Factor:   1.0,
		Jitter:   0,
	}
	clusterID, err := utils.GetClusterID(k8sClient, clusterIDConfMap, namespaceName, backoff)
	if err != nil {
		klog.Errorf("an error occurred while retrieving the clusterID: %s", err)
		os.Exit(-1)
	} else {
		klog.Infof("setting local clusterID to: %s", clusterID)
	}
	clusterIDInterface := clusterid.NewStaticClusterID(clusterID)
	namespaceManager := tenantnamespace.NewTenantNamespaceManager(k8sClient)
	dynClient := dynamic.NewForConfigOrDie(cfg)
	d := &crdreplicator.Controller{
		Scheme:                mgr.GetScheme(),
		Client:                mgr.GetClient(),
		ClientSet:             k8sClient,
		ClusterID:             clusterID,
		RemoteDynClients:      make(map[string]dynamic.Interface),
		LocalDynClient:        dynClient,
		RegisteredResources:   nil,
		UnregisteredResources: nil,
		LocalWatchers:         make(map[string]chan struct{}),
		RemoteWatchers:        make(map[string]map[string]chan struct{}),
		NamespaceManager:      namespaceManager,
		IdentityReader: identitymanager.NewCertificateIdentityReader(
			k8sClient, clusterIDInterface, namespaceManager),
		LocalToRemoteNamespaceMapper:     map[string]string{},
		RemoteToLocalNamespaceMapper:     map[string]string{},
		ClusterIDToLocalNamespaceMapper:  map[string]string{},
		ClusterIDToRemoteNamespaceMapper: map[string]string{},
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
