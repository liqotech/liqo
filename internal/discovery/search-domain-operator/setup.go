package search_domain_operator

import (
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/internal/discovery"
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

func StartOperator(mgr *manager.Manager, requeueAfter time.Duration, discoveryCtrl *discovery.DiscoveryCtrl, kubeconfigPath string) {
	config, err := crdClient.NewKubeconfig(kubeconfigPath, &discoveryv1alpha1.GroupVersion, nil)
	if err != nil {
		klog.Error(err, "unable to get kube config")
		os.Exit(1)
	}
	crdClient, err := crdClient.NewFromConfig(config)
	if err != nil {
		klog.Error(err, "unable to create crd client")
		os.Exit(1)
	}

	if err = (GetSDReconciler(
		(*mgr).GetScheme(),
		crdClient,
		discoveryCtrl,
		requeueAfter,
	)).SetupWithManager(*mgr); err != nil {
		klog.Error(err, "unable to create controller", "controller", "ForeignCluster")
		os.Exit(1)
	}
}

func GetSDReconciler(scheme *runtime.Scheme, crdClient *crdClient.CRDClient, discoveryCtrl *discovery.DiscoveryCtrl, requeueAfter time.Duration) *SearchDomainReconciler {
	return &SearchDomainReconciler{
		Scheme:        scheme,
		requeueAfter:  requeueAfter,
		crdClient:     crdClient,
		DiscoveryCtrl: discoveryCtrl,
	}
}
