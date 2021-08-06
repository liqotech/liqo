package searchdomainoperator

import (
	"os"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
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

// StartOperator setups the SearchDomain operator.
func StartOperator(mgr manager.Manager, requeueAfter time.Duration, discoveryCtrl *discovery.Controller, kubeconfigPath string) {
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

	if err = (getSDReconciler(
		mgr.GetScheme(),
		client,
		discoveryCtrl,
		requeueAfter,
	)).SetupWithManager(mgr); err != nil {
		klog.Error(err, "unable to create controller", "controller", "ForeignCluster")
		os.Exit(1)
	}
}

func getSDReconciler(scheme *runtime.Scheme, client *crdclient.CRDClient,
	discoveryCtrl *discovery.Controller, requeueAfter time.Duration) *SearchDomainReconciler {
	return &SearchDomainReconciler{
		Scheme:        scheme,
		requeueAfter:  requeueAfter,
		crdClient:     client,
		DiscoveryCtrl: discoveryCtrl,
	}
}
