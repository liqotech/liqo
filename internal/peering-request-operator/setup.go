package peering_request_operator

import (
	"os"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/clusterid"
	crdclient "github.com/liqotech/liqo/pkg/crdClient"
	"github.com/liqotech/liqo/pkg/mapperUtils"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = discoveryv1alpha1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func StartOperator(namespace string, broadcasterImage string, broadcasterServiceAccount string, vkServiceAccount string, kubeconfigPath string) {
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		MapperProvider:   mapperUtils.LiqoMapperProvider(scheme),
		Scheme:           scheme,
		Port:             9443,
		LeaderElection:   false,
		LeaderElectionID: "b3156c4e.liqo.io",
	})
	if err != nil {
		klog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	config, err := crdclient.NewKubeconfig(kubeconfigPath, &discoveryv1alpha1.GroupVersion, nil)
	if err != nil {
		klog.Error(err, "unable to get kube config")
		os.Exit(1)
	}
	client, err := crdclient.NewFromConfig(config)
	if err != nil {
		klog.Error(err, "unable to create crd client")
		os.Exit(1)
	}

	clusterId, err := clusterid.NewClusterID(kubeconfigPath)
	if err != nil {
		klog.Error(err, "unable to get clusterid")
		os.Exit(1)
	}

	if err = (GetPRReconciler(
		mgr.GetScheme(),
		client,
		namespace,
		clusterId,
		broadcasterImage,
		broadcasterServiceAccount,
		vkServiceAccount,
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

// GetPRReconciler builds and returns a PeeringRequestReconciler.
func GetPRReconciler(scheme *runtime.Scheme, crdClient *crdclient.CRDClient, namespace string,
	clusterID clusterid.ClusterID, broadcasterImage, broadcasterServiceAccount, vkServiceAccount string) *PeeringRequestReconciler {
	return &PeeringRequestReconciler{
		Scheme:                    scheme,
		crdClient:                 crdClient,
		Namespace:                 namespace,
		clusterID:                 clusterID,
		broadcasterImage:          broadcasterImage,
		broadcasterServiceAccount: broadcasterServiceAccount,
		vkServiceAccount:          vkServiceAccount,
		retryTimeout:              1 * time.Minute,
		ForeignConfig:             nil,
	}
}
