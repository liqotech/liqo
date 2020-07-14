package foreign_cluster_operator

import (
	protocolv1 "github.com/liqoTech/liqo/api/advertisement-operator/v1"
	discoveryv1 "github.com/liqoTech/liqo/api/discovery/v1"
	"github.com/liqoTech/liqo/internal/discovery"
	"github.com/liqoTech/liqo/pkg/clusterID"
	"github.com/liqoTech/liqo/pkg/crdClient"
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
	_ = discoveryv1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func StartOperator(mgr *manager.Manager, namespace string, requeueAfter time.Duration, discoveryCtrl *discovery.DiscoveryCtrl, kubeconfigPath string) {
	config, err := crdClient.NewKubeconfig(kubeconfigPath, &discoveryv1.GroupVersion)
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

	advClient, err := protocolv1.CreateAdvertisementClient(kubeconfigPath, nil)
	if err != nil {
		klog.Error(err, "unable to create local client for Advertisement")
		os.Exit(1)
	}

	if err = (GetFCReconciler(
		(*mgr).GetScheme(),
		namespace,
		discoveryClient,
		advClient,
		clusterId,
		requeueAfter,
		discoveryCtrl,
	)).SetupWithManager(*mgr); err != nil {
		klog.Error(err, "unable to create controller", "controller", "ForeignCluster")
		os.Exit(1)
	}
}

func GetFCReconciler(scheme *runtime.Scheme, namespace string, crdClient *crdClient.CRDClient, advertisementClient *crdClient.CRDClient, clusterId *clusterID.ClusterID, requeueAfter time.Duration, discoveryCtrl *discovery.DiscoveryCtrl) *ForeignClusterReconciler {
	return &ForeignClusterReconciler{
		Scheme:              scheme,
		Namespace:           namespace,
		crdClient:           crdClient,
		advertisementClient: advertisementClient,
		clusterID:           clusterId,
		ForeignConfig:       nil,
		RequeueAfter:        requeueAfter,
		DiscoveryCtrl:       discoveryCtrl,
	}
}
