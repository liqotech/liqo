package resourcerequestoperator

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	capsulev1alpha1 "github.com/clastix/capsule/api/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	"github.com/liqotech/liqo/pkg/clusterid"
)

var (
	cfg            *rest.Config
	k8sClient      client.Client
	homeClusterID  string
	clientset      kubernetes.Interface
	testEnv        *envtest.Environment
	newBroadcaster Broadcaster
	ctx            context.Context
	cancel         context.CancelFunc
	group          sync.WaitGroup
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

func createCluster() {
	By("bootstrapping test environment")
	ctx, cancel = context.WithCancel(context.Background())
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "deployments", "liqo", "crds"),
			filepath.Join("..", "..", "externalcrds"),
		},
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	err = discoveryv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = sharingv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = configv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = capsulev1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	// +kubebuilder:scaffold:scheme

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:             scheme.Scheme,
		MetricsBindAddress: "0", // this avoids port binding collision
	})
	Expect(err).ToNot(HaveOccurred())
	clientset = kubernetes.NewForConfigOrDie(k8sManager.GetConfig())
	homeClusterID = clusterid.NewStaticClusterID("test-cluster").GetClusterID()
	updater := OfferUpdater{}
	updater.Setup(homeClusterID, k8sManager.GetScheme(), &newBroadcaster, k8sManager.GetClient())
	err = newBroadcaster.SetupBroadcaster(clientset, &updater, 5*time.Second, 5)
	Expect(err).ToNot(HaveOccurred())
	newBroadcaster.StartBroadcaster(ctx, &group)
	testClusterConf := &configv1alpha1.ClusterConfig{
		Spec: configv1alpha1.ClusterConfigSpec{
			AdvertisementConfig: configv1alpha1.AdvertisementConfig{
				OutgoingConfig: configv1alpha1.BroadcasterConfig{
					ResourceSharingPercentage: 50,
				},
			},
		},
	}
	newBroadcaster.setConfig(testClusterConf)
	err = (&ResourceRequestReconciler{
		Client:      k8sManager.GetClient(),
		Scheme:      k8sManager.GetScheme(),
		ClusterID:   homeClusterID,
		Broadcaster: &newBroadcaster,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred())
	}()

	k8sClient = k8sManager.GetClient()
	Expect(k8sClient).ToNot(BeNil())

	ctx = context.TODO()
}

func destroyCluster() {
	By("tearing down the test environment")
	cancel()
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
	group.Wait()
}

var _ = BeforeSuite(func() {
	createCluster()
})

var _ = AfterSuite(func() {
	destroyCluster()
})
