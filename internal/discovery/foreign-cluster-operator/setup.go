package foreignclusteroperator

import (
	"os"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/liqotech/liqo/pkg/clusterid"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	nettypes "github.com/liqotech/liqo/apis/net/v1alpha1"
	advtypes "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	"github.com/liqotech/liqo/internal/discovery"
	crdclient "github.com/liqotech/liqo/pkg/crdClient"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = discoveryv1alpha1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

// StartOperator setups the ForeignCluster operator.
func StartOperator(
	mgr manager.Manager, namespace string, requeueAfter time.Duration,
	discoveryCtrl *discovery.Controller, kubeconfigPath string) {
	config, err := crdclient.NewKubeconfig(kubeconfigPath, &discoveryv1alpha1.GroupVersion, nil)
	if err != nil {
		klog.Error(err, "unable to get kube config")
		os.Exit(1)
	}
	discoveryClient, err := crdclient.NewFromConfig(config)
	if err != nil {
		klog.Error(err, "unable to create crd client")
		os.Exit(1)
	}
	localclusterID, err := clusterid.NewClusterID(kubeconfigPath)
	if err != nil {
		klog.Error(err, "unable to get clusterid")
		os.Exit(1)
	}

	advClient, err := advtypes.CreateAdvertisementClient(kubeconfigPath, nil, true, nil)
	if err != nil {
		klog.Error(err, "unable to create local client for Advertisement")
		os.Exit(1)
	}

	networkClient, err := nettypes.CreateTunnelEndpointClient(kubeconfigPath)
	if err != nil {
		klog.Error(err, "unable to create local client for Networking")
		os.Exit(1)
	}

	if err = (getFCReconciler(
		mgr.GetScheme(),
		namespace,
		discoveryClient,
		advClient,
		networkClient,
		localclusterID,
		requeueAfter,
		discoveryCtrl,
	)).SetupWithManager(mgr); err != nil {
		klog.Error(err, "unable to create controller", "controller", "ForeignCluster")
		os.Exit(1)
	}
}

func getFCReconciler(scheme *runtime.Scheme,
	namespace string,
	client, advertisementClient, networkClient *crdclient.CRDClient,
	localClusterID clusterid.ClusterID,
	requeueAfter time.Duration,
	configProvider discovery.ConfigProvider) *ForeignClusterReconciler {
	return &ForeignClusterReconciler{
		Scheme:              scheme,
		Namespace:           namespace,
		crdClient:           client,
		advertisementClient: advertisementClient,
		networkClient:       networkClient,
		clusterID:           localClusterID,
		ForeignConfig:       nil,
		RequeueAfter:        requeueAfter,
		ConfigProvider:      configProvider,
	}
}
