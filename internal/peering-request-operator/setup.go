package peering_request_operator

import (
	discoveryv1alpha1 "github.com/liqoTech/liqo/api/discovery/v1alpha1"
	"github.com/liqoTech/liqo/pkg/clusterID"
	"github.com/liqoTech/liqo/pkg/crdClient"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
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

func StartOperator(namespace string, broadcasterImage string, broadcasterServiceAccount string, kubeconfigPath string) {
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

	config, err := crdClient.NewKubeconfig(kubeconfigPath, &discoveryv1alpha1.GroupVersion)
	if err != nil {
		klog.Error(err, "unable to get kube config")
		os.Exit(1)
	}
	client, err := crdClient.NewFromConfig(config)
	if err != nil {
		klog.Error(err, "unable to create crd client")
		os.Exit(1)
	}

	clusterId, err := clusterID.NewClusterID(kubeconfigPath)
	if err != nil {
		klog.Error(err, "unable to get clusterID")
		os.Exit(1)
	}

	if err = (GetPRReconciler(
		mgr.GetScheme(),
		client,
		namespace,
		clusterId,
		broadcasterImage,
		broadcasterServiceAccount,
	)).SetupWithManager(mgr); err != nil {
		klog.Error(err, "unable to create controller")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		klog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func GetPRReconciler(scheme *runtime.Scheme, crdClient *crdClient.CRDClient, namespace string, clusterId *clusterID.ClusterID, broadcasterImage string, broadcasterServiceAccount string) *PeeringRequestReconciler {
	return &PeeringRequestReconciler{
		Scheme:                    scheme,
		crdClient:                 crdClient,
		Namespace:                 namespace,
		clusterId:                 clusterId,
		broadcasterImage:          broadcasterImage,
		broadcasterServiceAccount: broadcasterServiceAccount,
		retryTimeout:              1 * time.Minute,
		ForeignConfig:             nil,
	}
}
