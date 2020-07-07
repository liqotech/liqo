package search_domain_operator

import (
	discoveryv1 "github.com/liqoTech/liqo/api/discovery/v1"
	"github.com/liqoTech/liqo/internal/discovery"
	"github.com/liqoTech/liqo/pkg/crdClient/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/klog"
	"os"
	"path/filepath"
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

func StartOperator(mgr *manager.Manager, requeueAfter time.Duration, discoveryCtrl *discovery.DiscoveryCtrl) {
	config, err := v1alpha1.NewKubeconfig(filepath.Join(os.Getenv("HOME"), ".kube", "config"), &discoveryv1.GroupVersion)
	if err != nil {
		klog.Error(err, "unable to get kube config")
		os.Exit(1)
	}
	crdClient, err := v1alpha1.NewFromConfig(config)
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

	/*sdRec := GetSDReconciler(
		(*mgr).GetScheme(),
		crdClient,
		discoveryCtrl,
		requeueAfter,
	)

	w, err := crdClient.Resource("searchdomains").Watch(metav1.ListOptions{})
	if err != nil {
		klog.Error(err, "unable to start watcher")
		os.Exit(1)
	}
	wc := w.ResultChan()
	for event := range wc {
		sd, ok := event.Object.(*discoveryv1.SearchDomain)
		if !ok {
			klog.Error("Retrieved object is not a SearchDomain ", event.Object)
			continue
		}
		requeueAfter, err := sdRec.Reconcile(event, sd)
		if err != nil {
			klog.Error(err, err.Error())
		}
		if requeueAfter > 0 {
			// TODO
		}
	}*/
}

func GetSDReconciler(scheme *runtime.Scheme, crdClient *v1alpha1.CRDClient, discoveryCtrl *discovery.DiscoveryCtrl, requeueAfter time.Duration) *SearchDomainReconciler {
	return &SearchDomainReconciler{
		Scheme:        scheme,
		requeueAfter:  requeueAfter,
		crdClient:     crdClient,
		DiscoveryCtrl: discoveryCtrl,
	}
}
