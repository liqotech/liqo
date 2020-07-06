package foreign_cluster_operator

import (
	discoveryv1 "github.com/liqoTech/liqo/api/discovery/v1"
	"github.com/liqoTech/liqo/internal/discovery"
	"github.com/liqoTech/liqo/pkg/clusterID"
	"github.com/liqoTech/liqo/pkg/crdClient"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
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
	_ = discoveryv1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func StartOperator(namespace string, requeueAfter time.Duration, discoveryCtrl *discovery.DiscoveryCtrl) {
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

	config, err := crdClient.NewKubeconfig(filepath.Join(os.Getenv("HOME"), ".kube", "config"), &discoveryv1.GroupVersion)
	if err != nil {
		klog.Error(err, "unable to get kube config")
		os.Exit(1)
	}
	crdClient, err := crdClient.NewFromConfig(config)
	if err != nil {
		klog.Error(err, "unable to create crd client")
		os.Exit(1)
	}
	clusterId, err := clusterID.NewClusterID()
	if err != nil {
		klog.Error(err, "unable to get clusterID")
		os.Exit(1)
	}

	if err = (GetFCReconciler(
		mgr.GetScheme(),
		namespace,
		crdClient,
		clusterId,
		requeueAfter,
		discoveryCtrl,
	)).SetupWithManager(mgr); err != nil {
		klog.Error(err, "unable to create controller", "controller", "ForeignCluster")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		klog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func GetFCReconciler(scheme *runtime.Scheme, namespace string, crdClient *crdClient.CRDClient, clusterId *clusterID.ClusterID, requeueAfter time.Duration, discoveryCtrl *discovery.DiscoveryCtrl) *ForeignClusterReconciler {
	return &ForeignClusterReconciler{
		Scheme:        scheme,
		Namespace:     namespace,
		crdClient:     crdClient,
		clusterID:     clusterId,
		ForeignConfig: nil,
		RequeueAfter:  requeueAfter,
		DiscoveryCtrl: discoveryCtrl,
	}
}
