package peering_request_operator

import (
	discoveryv1 "github.com/liqoTech/liqo/api/discovery/v1"
	"github.com/liqoTech/liqo/internal/discovery/clients"
	"github.com/liqoTech/liqo/pkg/clusterID"
	v1 "github.com/liqoTech/liqo/pkg/discovery/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog"
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

func StartOperator(namespace string, configMapName string, broadcasterImage string, broadcasterServiceAccount string) {
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

	client, err := clients.NewK8sClient()
	if err != nil {
		klog.Error(err, "unable to start manager")
		os.Exit(1)
	}
	discoveryClient, err := clients.NewDiscoveryClient()
	if err != nil {
		klog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	clusterId, err := clusterID.NewClusterID()
	if err != nil {
		klog.Error(err, "unable to get clusterID")
		os.Exit(1)
	}

	if err = (GetPRReconciler(
		mgr.GetScheme(),
		client,
		discoveryClient,
		namespace,
		clusterId,
		configMapName,
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

func GetPRReconciler(scheme *runtime.Scheme, client *kubernetes.Clientset, discoveryClient *v1.DiscoveryV1Client, namespace string, clusterId *clusterID.ClusterID, configMapName string, broadcasterImage string, broadcasterServiceAccount string) *PeeringRequestReconciler {
	return &PeeringRequestReconciler{
		Scheme:                    scheme,
		client:                    client,
		discoveryClient:           discoveryClient,
		Namespace:                 namespace,
		clusterId:                 clusterId,
		configMapName:             configMapName,
		broadcasterImage:          broadcasterImage,
		broadcasterServiceAccount: broadcasterServiceAccount,
	}
}
