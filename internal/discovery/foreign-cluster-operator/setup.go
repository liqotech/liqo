package foreign_cluster_operator

import (
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	nettypes "github.com/liqotech/liqo/apis/net/v1alpha1"
	advtypes "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	"github.com/liqotech/liqo/internal/discovery"
	"github.com/liqotech/liqo/pkg/clusterID"
	"github.com/liqotech/liqo/pkg/crdClient"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/klog"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"time"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = discoveryv1alpha1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func StartOperator(mgr *manager.Manager, namespace string, requeueAfter time.Duration, discoveryCtrl *discovery.DiscoveryCtrl, kubeconfigPath string) {
	config, err := crdClient.NewKubeconfig(kubeconfigPath, &discoveryv1alpha1.GroupVersion)
	if err != nil {
		klog.Error(err, "unable to get kube config")
		os.Exit(1)
	}
	discoveryClient, err := crdClient.NewFromConfig(config)
	if err != nil {
		klog.Error(err, "unable to create crd client")
		os.Exit(1)
	}
	clusterId, err := clusterID.NewClusterID(kubeconfigPath)
	if err != nil {
		klog.Error(err, "unable to get clusterID")
		os.Exit(1)
	}

	advClient, err := advtypes.CreateAdvertisementClient(kubeconfigPath, nil)
	if err != nil {
		klog.Error(err, "unable to create local client for Advertisement")
		os.Exit(1)
	}

	networkClient, err := nettypes.CreateTunnelEndpointClient(kubeconfigPath)
	if err != nil {
		klog.Error(err, "unable to create local client for Networking")
		os.Exit(1)
	}

	if err = (GetFCReconciler(
		(*mgr).GetScheme(),
		namespace,
		discoveryClient,
		advClient,
		networkClient,
		clusterId,
		requeueAfter,
		discoveryCtrl,
	)).SetupWithManager(*mgr); err != nil {
		klog.Error(err, "unable to create controller", "controller", "ForeignCluster")
		os.Exit(1)
	}
}

func GetFCReconciler(scheme *runtime.Scheme, namespace string, crdClient *crdClient.CRDClient, advertisementClient *crdClient.CRDClient, networkClient *crdClient.CRDClient, clusterId *clusterID.ClusterID, requeueAfter time.Duration, discoveryCtrl *discovery.DiscoveryCtrl) *ForeignClusterReconciler {
	return &ForeignClusterReconciler{
		Scheme:              scheme,
		Namespace:           namespace,
		crdClient:           crdClient,
		advertisementClient: advertisementClient,
		networkClient:       networkClient,
		clusterID:           clusterId,
		ForeignConfig:       nil,
		RequeueAfter:        requeueAfter,
		DiscoveryCtrl:       discoveryCtrl,
	}
}
