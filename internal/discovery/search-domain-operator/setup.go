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
func StartOperator(mgr manager.Manager, requeueAfter time.Duration, discoveryCtrl *discovery.Controller) {
	if err := (&SearchDomainReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		requeueAfter:  requeueAfter,
		DiscoveryCtrl: discoveryCtrl,
	}).SetupWithManager(mgr); err != nil {
		klog.Error(err, "unable to create controller", "controller", "ForeignCluster")
		os.Exit(1)
	}
}
