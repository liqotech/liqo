package resourceoffercontroller

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	crdreplicator "github.com/liqotech/liqo/internal/crdReplicator"
	"github.com/liqotech/liqo/pkg/discovery"
	testUtils "github.com/liqotech/liqo/pkg/utils/testUtils"
)

const (
	timeout   = time.Second * 30
	interval  = time.Millisecond * 250
	clusterID = "clusterID"
)

var (
	cluster    testUtils.Cluster
	mgr        manager.Manager
	controller *ResourceOfferReconciler
	ctx        context.Context
	cancel     context.CancelFunc
)

func TestIdentityManager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ResourceOffer Controller Suite")
}

func createForeignCluster() {
	foreignCluster := &discoveryv1alpha1.ForeignCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foreigncluster",
			Labels: map[string]string{
				discovery.ClusterIDLabel: clusterID,
			},
		},
		Spec: discoveryv1alpha1.ForeignClusterSpec{
			AuthURL: "https://127.0.0.1:8080",
		},
	}

	if err := controller.Client.Create(ctx, foreignCluster); err != nil {
		By(err.Error())
		os.Exit(1)
	}
}

var _ = Describe("ResourceOffer Controller", func() {

	BeforeEach(func() {
		var err error
		cluster, mgr, err = testUtils.NewTestCluster([]string{filepath.Join("..", "..", "..", "deployments", "liqo", "crds")})
		if err != nil {
			By(err.Error())
			os.Exit(1)
		}

		controller = NewResourceOfferController(mgr, 10*time.Second)
		if err := controller.SetupWithManager(mgr); err != nil {
			By(err.Error())
			os.Exit(1)
		}

		controller.setConfig(&configv1alpha1.ClusterConfig{
			Spec: configv1alpha1.ClusterConfigSpec{
				AdvertisementConfig: configv1alpha1.AdvertisementConfig{
					IngoingConfig: configv1alpha1.AdvOperatorConfig{
						MaxAcceptableAdvertisement: 1000,
						AcceptPolicy:               configv1alpha1.AutoAcceptMax,
					},
				},
			},
		})

		ctx, cancel = context.WithCancel(context.Background())
		go mgr.Start(ctx)

		createForeignCluster()
	})

	AfterEach(func() {
		cancel()

		err := cluster.GetEnv().Stop()
		if err != nil {
			By(err.Error())
			os.Exit(1)
		}
	})

	type resourceOfferTestcase struct {
		resourceOffer sharingv1alpha1.ResourceOffer
		expectedPhase sharingv1alpha1.OfferPhase
	}

	DescribeTable("ResourceOffer table",

		func(c resourceOfferTestcase) {
			err := controller.Client.Create(ctx, &c.resourceOffer)
			Expect(err).To(BeNil())

			Eventually(func() sharingv1alpha1.OfferPhase {
				var resourceOffer sharingv1alpha1.ResourceOffer
				if err = controller.Client.Get(ctx, client.ObjectKeyFromObject(&c.resourceOffer), &resourceOffer); err != nil {
					return "error"
				}
				return resourceOffer.Status.Phase
			}, timeout, interval).Should(Equal(c.expectedPhase))

			err = controller.Client.Delete(ctx, &c.resourceOffer)
			Expect(err).To(BeNil())
		},

		// this entry should be taken by the operator, and it should set the phase and the virtual-kubelet deployment accordingly.
		Entry("valid pending resource offer", resourceOfferTestcase{
			resourceOffer: sharingv1alpha1.ResourceOffer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "resource-offer",
					Namespace: "default",
					Labels: map[string]string{
						crdreplicator.RemoteLabelSelector:    "originClusterID",
						crdreplicator.ReplicationStatuslabel: "true",
					},
				},
				Spec: sharingv1alpha1.ResourceOfferSpec{
					ClusterId:  clusterID,
					Timestamp:  metav1.Now(),
					TimeToLive: metav1.NewTime(time.Now().Add(1 * time.Hour)),
				},
			},
			expectedPhase: sharingv1alpha1.ResourceOfferAccepted,
		}),

		// this entry should not be taken by the operator, it has not the labels of a replicated resource.
		Entry("valid pending resource offer without labels", resourceOfferTestcase{
			resourceOffer: sharingv1alpha1.ResourceOffer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "resource-offer-2",
					Namespace: "default",
				},
				Spec: sharingv1alpha1.ResourceOfferSpec{
					ClusterId:  clusterID,
					Timestamp:  metav1.Now(),
					TimeToLive: metav1.NewTime(time.Now().Add(1 * time.Hour)),
				},
			},
			expectedPhase: "",
		}),
	)

})
