package resourceoffercontroller

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	crdreplicator "github.com/liqotech/liqo/internal/crdReplicator"
	"github.com/liqotech/liqo/pkg/clusterid"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/discovery"
	"github.com/liqotech/liqo/pkg/utils/testutil"
	"github.com/liqotech/liqo/pkg/vkMachinery/forge"
)

const (
	timeout   = time.Second * 30
	interval  = time.Millisecond * 250
	clusterID = "cluster-id"

	testNamespace = "default"

	virtualKubeletImage     = "vk-image"
	initVirtualKubeletImage = "init-vk-image"
)

var (
	cluster    testutil.Cluster
	mgr        manager.Manager
	controller *ResourceOfferReconciler
	ctx        context.Context
	cancel     context.CancelFunc

	now = metav1.Now()
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
			ForeignAuthURL:         "https://127.0.0.1:8080",
			OutgoingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
			IncomingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
			InsecureSkipTLSVerify:  pointer.BoolPtr(true),
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
		cluster, mgr, err = testutil.NewTestCluster([]string{filepath.Join("..", "..", "..", "deployments", "liqo", "crds")})
		if err != nil {
			By(err.Error())
			os.Exit(1)
		}

		clusterID := clusterid.NewStaticClusterID("remote-id")

		kubeletOpts := &forge.VirtualKubeletOpts{
			ContainerImage:     virtualKubeletImage,
			InitContainerImage: initVirtualKubeletImage,
		}

		controller = NewResourceOfferController(mgr, clusterID, 10*time.Second, testNamespace, kubeletOpts)
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

	DescribeTable("ResourceOffer phase table",

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
					Namespace: testNamespace,
					Labels: map[string]string{
						crdreplicator.RemoteLabelSelector:    "origin-cluster-id",
						crdreplicator.ReplicationStatuslabel: "true",
					},
				},
				Spec: sharingv1alpha1.ResourceOfferSpec{
					ClusterId: clusterID,
				},
			},
			expectedPhase: sharingv1alpha1.ResourceOfferAccepted,
		}),

		// this entry should not be taken by the operator, it has not the labels of a replicated resource.
		Entry("valid pending resource offer without labels", resourceOfferTestcase{
			resourceOffer: sharingv1alpha1.ResourceOffer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "resource-offer-2",
					Namespace: testNamespace,
				},
				Spec: sharingv1alpha1.ResourceOfferSpec{
					ClusterId: clusterID,
				},
			},
			expectedPhase: "",
		}),
	)

	Describe("ResourceOffer virtual-kubelet", func() {

		It("test virtual kubelet creation", func() {
			resourceOffer := &sharingv1alpha1.ResourceOffer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "resource-offer",
					Namespace: testNamespace,
					Labels: map[string]string{
						crdreplicator.RemoteLabelSelector:    "origin-cluster-id",
						crdreplicator.ReplicationStatuslabel: "true",
					},
				},
				Spec: sharingv1alpha1.ResourceOfferSpec{
					ClusterId: clusterID,
				},
			}
			key := client.ObjectKeyFromObject(resourceOffer)

			err := controller.Client.Create(ctx, resourceOffer)
			Expect(err).To(BeNil())

			Eventually(func() sharingv1alpha1.OfferPhase {
				if err = controller.Client.Get(ctx, key, resourceOffer); err != nil {
					return "error"
				}
				return resourceOffer.Status.Phase
			}, timeout, interval).Should(Equal(sharingv1alpha1.ResourceOfferAccepted))

			// check that the vk status is set correctly
			Eventually(func() sharingv1alpha1.VirtualKubeletStatus {
				if err = controller.Client.Get(ctx, key, resourceOffer); err != nil {
					return "error"
				}
				return resourceOffer.Status.VirtualKubeletStatus
			}, timeout, interval).Should(Equal(sharingv1alpha1.VirtualKubeletStatusCreated))

			// check the creation of the deployment
			Eventually(func() bool {
				var deploymentList v1.DeploymentList
				err := controller.Client.List(ctx, &deploymentList)
				if err != nil || len(deploymentList.Items) != 1 {
					return false
				}

				vkDeploy, err := controller.getVirtualKubeletDeployment(ctx, resourceOffer)
				if err != nil || vkDeploy == nil {
					return false
				}
				return reflect.DeepEqual(deploymentList.Items[0], *vkDeploy)
			}, timeout, interval).Should(BeTrue())

			// check that the deployment has the controller reference annotation
			Eventually(func() string {
				vkDeploy, err := controller.getVirtualKubeletDeployment(ctx, resourceOffer)
				if err != nil || vkDeploy == nil {
					return ""
				}
				return vkDeploy.Annotations[resourceOfferAnnotation]
			}, timeout, interval).Should(Equal(resourceOffer.Name))

			// check the existence of the ClusterRoleBinding
			Eventually(func() int {
				labels := forge.ClusterRoleLabels(clusterID)
				var clusterRoleBindingList rbacv1.ClusterRoleBindingList
				err := controller.Client.List(ctx, &clusterRoleBindingList, client.MatchingLabels(labels))
				if err != nil {
					return -1
				}
				return len(clusterRoleBindingList.Items)
			}, timeout, interval).Should(BeNumerically("==", 1))

			// get the vk deployment and delete it
			vkDeploy, err := controller.getVirtualKubeletDeployment(ctx, resourceOffer)
			Expect(err).To(BeNil())
			err = controller.Client.Delete(ctx, vkDeploy)
			Expect(err).To(BeNil())

			// check the deployment recreation
			Eventually(func() types.UID {
				newVkDeploy, err := controller.getVirtualKubeletDeployment(ctx, resourceOffer)
				if err != nil || newVkDeploy == nil {
					return vkDeploy.UID // this will cause the eventually statement to not terminate
				}
				return newVkDeploy.UID
			}, timeout, interval).ShouldNot(Equal(vkDeploy.UID))

			err = controller.Client.Get(ctx, client.ObjectKeyFromObject(resourceOffer), resourceOffer)
			Expect(err).To(BeNil())

			// refuse the offer to delete the virtual-kubelet
			resourceOffer.Status.Phase = sharingv1alpha1.ResourceOfferRefused
			err = controller.Client.Status().Update(ctx, resourceOffer)
			Expect(err).To(BeNil())

			// check that the vk status is set correctly
			Eventually(func() sharingv1alpha1.VirtualKubeletStatus {
				if err = controller.Client.Get(ctx, key, resourceOffer); err != nil {
					return "error"
				}
				return resourceOffer.Status.VirtualKubeletStatus
			}, timeout, interval).Should(Equal(sharingv1alpha1.VirtualKubeletStatusNone))

			// check the deletion of the deployment
			Eventually(func() int {
				var deploymentList v1.DeploymentList
				err := controller.Client.List(ctx, &deploymentList)
				if err != nil {
					return -1
				}
				return len(deploymentList.Items)
			}, timeout, interval).Should(BeNumerically("==", 0))

			// check the deletion of the ClusterRoleBinding
			Eventually(func() int {
				labels := forge.ClusterRoleLabels(clusterID)
				var clusterRoleBindingList rbacv1.ClusterRoleBindingList
				err := controller.Client.List(ctx, &clusterRoleBindingList, client.MatchingLabels(labels))
				if err != nil {
					return -1
				}
				return len(clusterRoleBindingList.Items)
			}, timeout, interval).Should(BeNumerically("==", 0))

			err = controller.Client.Delete(ctx, resourceOffer)
			Expect(err).To(BeNil())
		})

	})

})

var _ = Describe("ResourceOffer Operator util functions", func() {

	Context("getDeleteVirtualKubeletPhase", func() {

		type getDeleteVirtualKubeletPhaseTestcase struct {
			resourceOffer *sharingv1alpha1.ResourceOffer
			expected      OmegaMatcher
		}

		DescribeTable("getDeleteVirtualKubeletPhase table",

			func(c getDeleteVirtualKubeletPhaseTestcase) {
				Expect(getDeleteVirtualKubeletPhase(c.resourceOffer)).To(c.expected)
			},

			Entry("refused ResourceOffer", getDeleteVirtualKubeletPhaseTestcase{
				resourceOffer: &sharingv1alpha1.ResourceOffer{
					ObjectMeta: metav1.ObjectMeta{
						Finalizers: []string{},
					},
					Spec: sharingv1alpha1.ResourceOfferSpec{
						WithdrawalTimestamp: &now,
					},
					Status: sharingv1alpha1.ResourceOfferStatus{
						Phase: sharingv1alpha1.ResourceOfferRefused,
					},
				},
				expected: Equal(kubeletDeletePhaseNodeDeleted),
			}),

			Entry("accepted ResourceOffer", getDeleteVirtualKubeletPhaseTestcase{
				resourceOffer: &sharingv1alpha1.ResourceOffer{
					ObjectMeta: metav1.ObjectMeta{
						Finalizers: []string{},
					},
					Spec: sharingv1alpha1.ResourceOfferSpec{},
					Status: sharingv1alpha1.ResourceOfferStatus{
						Phase: sharingv1alpha1.ResourceOfferAccepted,
					},
				},
				expected: Equal(kubeletDeletePhaseNone),
			}),

			Entry("accepted ResourceOffer with deletion timestamp", getDeleteVirtualKubeletPhaseTestcase{
				resourceOffer: &sharingv1alpha1.ResourceOffer{
					ObjectMeta: metav1.ObjectMeta{
						DeletionTimestamp: &metav1.Time{
							Time: time.Now(),
						},
						Finalizers: []string{},
					},
					Spec: sharingv1alpha1.ResourceOfferSpec{},
					Status: sharingv1alpha1.ResourceOfferStatus{
						Phase: sharingv1alpha1.ResourceOfferAccepted,
					},
				},
				expected: Equal(kubeletDeletePhaseNodeDeleted),
			}),

			Entry("refused ResourceOffer with finalizer", getDeleteVirtualKubeletPhaseTestcase{
				resourceOffer: &sharingv1alpha1.ResourceOffer{
					ObjectMeta: metav1.ObjectMeta{
						Finalizers: []string{
							consts.NodeFinalizer,
						},
					},
					Spec: sharingv1alpha1.ResourceOfferSpec{},
					Status: sharingv1alpha1.ResourceOfferStatus{
						Phase: sharingv1alpha1.ResourceOfferRefused,
					},
				},
				expected: Equal(kubeletDeletePhaseDrainingNode),
			}),

			Entry("accepted ResourceOffer with deletion timestamp and finalizer", getDeleteVirtualKubeletPhaseTestcase{
				resourceOffer: &sharingv1alpha1.ResourceOffer{
					ObjectMeta: metav1.ObjectMeta{
						DeletionTimestamp: &metav1.Time{
							Time: time.Now(),
						},
						Finalizers: []string{
							consts.NodeFinalizer,
						},
					},
					Spec: sharingv1alpha1.ResourceOfferSpec{},
					Status: sharingv1alpha1.ResourceOfferStatus{
						Phase: sharingv1alpha1.ResourceOfferAccepted,
					},
				},
				expected: Equal(kubeletDeletePhaseDrainingNode),
			}),

			Entry("desired deletion of ResourceOffer", getDeleteVirtualKubeletPhaseTestcase{
				resourceOffer: &sharingv1alpha1.ResourceOffer{
					ObjectMeta: metav1.ObjectMeta{
						Finalizers: []string{
							consts.NodeFinalizer,
						},
					},
					Spec: sharingv1alpha1.ResourceOfferSpec{
						WithdrawalTimestamp: &now,
					},
					Status: sharingv1alpha1.ResourceOfferStatus{
						Phase: sharingv1alpha1.ResourceOfferAccepted,
					},
				},
				expected: Equal(kubeletDeletePhaseDrainingNode),
			}),

			Entry("desired deletion of ResourceOffer without finalizer", getDeleteVirtualKubeletPhaseTestcase{
				resourceOffer: &sharingv1alpha1.ResourceOffer{
					ObjectMeta: metav1.ObjectMeta{
						Finalizers: []string{},
					},
					Spec: sharingv1alpha1.ResourceOfferSpec{
						WithdrawalTimestamp: &now,
					},
					Status: sharingv1alpha1.ResourceOfferStatus{
						Phase: sharingv1alpha1.ResourceOfferAccepted,
					},
				},
				expected: Equal(kubeletDeletePhaseNodeDeleted),
			}),
		)

	})

	Context("getRequestFromObject", func() {

		type getRequestFromObjectTestcase struct {
			resourceOffer     *sharingv1alpha1.ResourceOffer
			expectedErr       OmegaMatcher
			expectedErrString OmegaMatcher
			expectedResult    OmegaMatcher
		}

		DescribeTable("getRequestFromObject table",

			func(c getRequestFromObjectTestcase) {
				res, err := getReconcileRequestFromObject(c.resourceOffer)
				Expect(err).To(c.expectedErr)
				if err != nil {
					Expect(err.Error()).To(c.expectedErrString)
				}
				Expect(res).To(c.expectedResult)
			},

			Entry("Object with no annotation", getRequestFromObjectTestcase{
				resourceOffer: &sharingv1alpha1.ResourceOffer{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "name",
						Namespace:   "namespace",
						Annotations: map[string]string{},
					},
				},
				expectedErr:       HaveOccurred(),
				expectedErrString: ContainSubstring("%v annotation not found in object %v/%v", resourceOfferAnnotation, "namespace", "name"),
				expectedResult:    Equal(reconcile.Request{}),
			}),

			Entry("Object with annotation", getRequestFromObjectTestcase{
				resourceOffer: &sharingv1alpha1.ResourceOffer{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "name",
						Namespace: "namespace",
						Annotations: map[string]string{
							resourceOfferAnnotation: "name",
						},
					},
				},
				expectedErr: Not(HaveOccurred()),
				expectedResult: Equal(reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "name",
						Namespace: "namespace",
					},
				}),
			}),
		)

	})

})
