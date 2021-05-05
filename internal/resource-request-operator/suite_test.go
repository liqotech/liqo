package resourcerequestoperator

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"

	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var clusterId string
var clientset *kubernetes.Clientset
var testEnv *envtest.Environment
var newBroadcaster Broadcaster
var ctx context.Context
var cancel context.CancelFunc
var group sync.WaitGroup

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func(done Done) {
	By("bootstrapping test environment")
	ctx, cancel = context.WithCancel(context.Background())
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "deployments", "liqo", "crds")},
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
	// +kubebuilder:scaffold:scheme

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:             scheme.Scheme,
		MetricsBindAddress: "0", // this avoids port binding collision
	})
	Expect(err).ToNot(HaveOccurred())
	clientset, err = kubernetes.NewForConfig(k8sManager.GetConfig())
	Expect(err).ToNot(HaveOccurred())
	err = newBroadcaster.SetupBroadcaster(clientset, 10*time.Second)
	Expect(err).ToNot(HaveOccurred())
	group.Add(1)
	go newBroadcaster.StartBroadcaster(ctx, &group)
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
	id, _ := uuid.NewUUID()
	clusterId = id.String()
	err = (&ResourceRequestReconciler{
		Client:      k8sManager.GetClient(),
		Scheme:      k8sManager.GetScheme(),
		ClusterID:   clusterId,
		Broadcaster: &newBroadcaster,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		err = k8sManager.Start(ctrl.SetupSignalHandler())
		Expect(err).ToNot(HaveOccurred())
	}()

	k8sClient = k8sManager.GetClient()
	Expect(k8sClient).ToNot(BeNil())
	close(done)
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
	cancel()
	group.Wait()
})
