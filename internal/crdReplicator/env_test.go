package crdReplicator

import (
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"strings"
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
	dynFac          dynamicinformer.DynamicSharedInformerFactory
	localDynFac     dynamicinformer.DynamicSharedInformerFactory
	gvr             = schema.GroupVersionResource{
		Group:    netv1alpha1.GroupVersion.Group,
		Version:  netv1alpha1.GroupVersion.Version,
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
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "deployments", "liqo", "crds")},
	}

	configLocal, err := testEnvLocal.Start()
	if err != nil {
		klog.Error(err, "an error occurred while setting up the local testing environment")
		os.Exit(-1)
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
	dynFac = dynamicinformer.NewFilteredDynamicSharedInformerFactory(dynClient, ResyncPeriod, metav1.NamespaceAll, func(options *metav1.ListOptions) {
		//we want to watch only the resources that have been created by us on the remote cluster
		if options.LabelSelector == "" {
			newLabelSelector := []string{RemoteLabelSelector, "=", localClusterID}
			options.LabelSelector = strings.Join(newLabelSelector, "")
		} else {
			newLabelSelector := []string{options.LabelSelector, RemoteLabelSelector, "=", localClusterID}
			options.LabelSelector = strings.Join(newLabelSelector, "")
		}
	})

	localDynFac = dynamicinformer.NewFilteredDynamicSharedInformerFactory(dynClient, ResyncPeriod, metav1.NamespaceAll, nil)
	time.Sleep(1 * time.Second)
}

func tearDown() {
	err := testEnvLocal.Stop()
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}
}
