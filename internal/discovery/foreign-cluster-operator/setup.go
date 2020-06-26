package foreign_cluster_operator

import (
	discoveryv1 "github.com/liqoTech/liqo/api/discovery/v1"
	"github.com/liqoTech/liqo/internal/discovery/clients"
	"github.com/liqoTech/liqo/pkg/clusterID"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"time"
)

var (
	scheme = runtime.NewScheme()
	log    = ctrl.Log.WithName("foreign-cluster-operator-setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = discoveryv1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func StartOperator(namespace string, requeueAfter time.Duration) {
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:           scheme,
		Port:             9443,
		LeaderElection:   false,
		LeaderElectionID: "b3156c4e.liqo.io",
	})
	if err != nil {
		log.Error(err, "unable to start manager")
		os.Exit(1)
	}

	client, err := clients.NewK8sClient()
	if err != nil {
		log.Error(err, "unable to start manager")
		os.Exit(1)
	}
	discoveryClient, err := clients.NewDiscoveryClient()
	if err != nil {
		log.Error(err, "unable to start manager")
		os.Exit(1)
	}
	clusterId, err := clusterID.NewClusterID()
	if err != nil {
		log.Error(err, "unable to get clusterID")
		os.Exit(1)
	}

	if err = (&ForeignClusterReconciler{
		Log:             ctrl.Log.WithName("controllers").WithName("ForeignCluster"),
		Scheme:          mgr.GetScheme(),
		Namespace:       namespace,
		client:          client,
		discoveryClient: discoveryClient,
		clusterID:       clusterId,
		RequeueAfter:    requeueAfter,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", "ForeignCluster")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Error(err, "problem running manager")
		os.Exit(1)
	}
}
