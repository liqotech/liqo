package dispatcher

import (
	discoveryv1 "github.com/liqoTech/liqo/api/discovery/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"time"

	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/scheme"
	"os"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"testing"
)

var (
	k8sManagerLocal ctrl.Manager
	testEnvLocal    *envtest.Environment
	dynClient       dynamic.Interface
	gvr             = schema.GroupVersionResource{
		Group:    "liqonet.liqo.io",
		Version:  "v1alpha1",
		Resource: "networkconfigs",
	}
	clusterID = "ClusterID-test"
)

func TestMain(m *testing.M) {
	setupEnv()
	defer tearDown()
	os.Exit(m.Run())
}

func setupEnv() {
	testEnvLocal = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "deployments", "liqo_chart", "crds")},
	}

	configLocal, err := testEnvLocal.Start()
	if err != nil {
		klog.Error(err, "an error occurred while setting up the local testing environment")
		os.Exit(-1)
	}
	err = clientgoscheme.AddToScheme(scheme.Scheme)
	if err != nil {
		klog.Error(err, err.Error())
	}
	err = discoveryv1.AddToScheme(scheme.Scheme)
	if err != nil {
		klog.Error(err)
	}

	k8sManagerLocal, err = ctrl.NewManager(configLocal, ctrl.Options{
		Scheme:             scheme.Scheme,
		MetricsBindAddress: "0",
	})
	if err != nil {
		klog.Error(err)
		panic(err)
	}
	dynClient = dynamic.NewForConfigOrDie(configLocal)
	time.Sleep(1 * time.Second)
}

func tearDown() {
	err := testEnvLocal.Stop()
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}
}
