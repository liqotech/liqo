// Copyright 2019-2023 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package resourceoffercontroller

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	virtualkubeletv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/discovery"
	"github.com/liqotech/liqo/pkg/identityManager/fake"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

const (
	timeout  = time.Second * 30
	interval = time.Millisecond * 250

	testNamespace        = "default"
	kubeconfigSecretName = "kubeconfig-secret"
)

var (
	cluster               testutil.Cluster
	remoteClusterIdentity = discoveryv1alpha1.ClusterIdentity{
		ClusterID:   "remote-cluster-id",
		ClusterName: "remote-cluster-name",
	}
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
			Name: remoteClusterIdentity.ClusterName,
			Labels: map[string]string{
				discovery.ClusterIDLabel: remoteClusterIdentity.ClusterID,
			},
		},
		Spec: discoveryv1alpha1.ForeignClusterSpec{
			ClusterIdentity:        remoteClusterIdentity,
			ForeignAuthURL:         "https://127.0.0.1:8080",
			OutgoingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
			IncomingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
			InsecureSkipTLSVerify:  pointer.Bool(true),
		},
	}
	if err := controller.Client.Create(ctx, foreignCluster); err != nil {
		By(err.Error())
		os.Exit(1)
	}

	foreignCluster.Status = discoveryv1alpha1.ForeignClusterStatus{
		TenantNamespace: discoveryv1alpha1.TenantNamespaceType{
			Local: testNamespace,
		},
	}
	if err := controller.Client.Status().Update(ctx, foreignCluster); err != nil {
		By(err.Error())
		os.Exit(1)
	}
}

var _ = BeforeSuite(func() {
	testutil.LogsToGinkgoWriter()
})

var _ = Describe("ResourceOffer Controller", func() {

	BeforeEach(func() {
		var err error
		cluster, mgr, err = testutil.NewTestCluster([]string{filepath.Join("..", "..", "..", "deployments", "liqo", "crds")})
		if err != nil {
			By(err.Error())
			os.Exit(1)
		}

		identityReader := fake.NewIdentityReader().Add(remoteClusterIdentity.ClusterID,
			testNamespace, kubeconfigSecretName, cluster.GetCfg())

		controller = NewResourceOfferController(mgr, identityReader, 10*time.Second, true)
		if err := controller.SetupWithManager(mgr); err != nil {
			By(err.Error())
			os.Exit(1)
		}

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
						consts.ReplicationOriginLabel: "origin-cluster-id",
						consts.ReplicationStatusLabel: "true",
					},
				},
				Spec: sharingv1alpha1.ResourceOfferSpec{
					ClusterID: remoteClusterIdentity.ClusterID,
				},
			},
			expectedPhase: sharingv1alpha1.ResourceOfferManualActionRequired, // auto-accept is off
		}),

		// this entry should not be taken by the operator, it has not the labels of a replicated resource.
		Entry("valid pending resource offer without labels", resourceOfferTestcase{
			resourceOffer: sharingv1alpha1.ResourceOffer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "resource-offer-2",
					Namespace: testNamespace,
				},
				Spec: sharingv1alpha1.ResourceOfferSpec{
					ClusterID: remoteClusterIdentity.ClusterID,
				},
			},
			expectedPhase: "",
		}),
	)

	Describe("ResourceOffer VirtualNode", func() {

		It("test VirtualNode creation", func() {
			resourceOffer := &sharingv1alpha1.ResourceOffer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "resource-offer",
					Namespace: testNamespace,
					Labels: map[string]string{
						consts.ReplicationOriginLabel: "origin-cluster-id",
						consts.ReplicationStatusLabel: "true",
					},
				},
				Spec: sharingv1alpha1.ResourceOfferSpec{
					ClusterID: remoteClusterIdentity.ClusterID,
				},
			}
			key := client.ObjectKeyFromObject(resourceOffer)

			err := controller.Client.Create(ctx, resourceOffer)
			Expect(err).To(BeNil())

			// The offer should not be automatically accepted, as specified in the config
			Eventually(func() sharingv1alpha1.OfferPhase {
				if err = controller.Client.Get(ctx, key, resourceOffer); err != nil {
					return "error"
				}
				return resourceOffer.Status.Phase
			}, timeout, interval).Should(Equal(sharingv1alpha1.ResourceOfferManualActionRequired))

			// Accept it manually
			resourceOffer.Status.Phase = sharingv1alpha1.ResourceOfferAccepted
			Expect(controller.Status().Update(ctx, resourceOffer)).To(Succeed())

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

			// check the creation of the virtual node
			Eventually(func() bool {
				var virtualNodeList virtualkubeletv1alpha1.VirtualNodeList
				err := controller.Client.List(ctx, &virtualNodeList)
				if err != nil || len(virtualNodeList.Items) != 1 {
					return false
				}

				vnStatus, err := controller.getVirtualNodeStatus(ctx, resourceOffer)
				if err != nil || vnStatus == nil {
					return false
				}
				return reflect.DeepEqual(virtualNodeList.Items[0].Status, *vnStatus)
			}, timeout, interval).Should(BeTrue())

			// check that the VirtualNode has the controller reference annotation
			Eventually(func() string {
				var virtualNodeList virtualkubeletv1alpha1.VirtualNodeList
				err := controller.Client.List(ctx, &virtualNodeList)
				if err != nil || len(virtualNodeList.Items) != 1 {
					return ""
				}

				if len(virtualNodeList.Items[0].OwnerReferences) != 1 {
					return ""
				}
				return virtualNodeList.Items[0].OwnerReferences[0].Name
			}, timeout, interval).Should(Equal(resourceOffer.Name))

			// get the VirtualNode and delete it
			var virtualNodeList virtualkubeletv1alpha1.VirtualNodeList
			err = controller.Client.List(ctx, &virtualNodeList)
			Expect(err).To(BeNil())
			Expect(len(virtualNodeList.Items)).To(Equal(1))
			err = controller.Client.Delete(ctx, &virtualNodeList.Items[0])
			Expect(err).To(BeNil())
			virtualNode := virtualNodeList.Items[0]

			// check the VirtualNode recreation
			Eventually(func() types.UID {
				var virtualNodeList virtualkubeletv1alpha1.VirtualNodeList
				err = controller.Client.List(ctx, &virtualNodeList)
				if err != nil || len(virtualNodeList.Items) != 1 {
					return virtualNode.UID // this will cause the eventually statement to not terminate
				}
				return virtualNodeList.Items[0].UID
			}, timeout, interval).ShouldNot(Equal(virtualNode.UID))

			Eventually(func() error {
				err = controller.Client.Get(ctx, client.ObjectKeyFromObject(resourceOffer), resourceOffer)
				if err != nil {
					return err
				}

				// refuse the offer to delete the virtual node
				resourceOffer.Status.Phase = sharingv1alpha1.ResourceOfferRefused
				return controller.Client.Status().Update(ctx, resourceOffer)
			}, timeout, interval).Should(Succeed())

			// check that the vk status is set correctly
			Eventually(func() sharingv1alpha1.VirtualKubeletStatus {
				if err = controller.Client.Get(ctx, key, resourceOffer); err != nil {
					return "error"
				}
				return resourceOffer.Status.VirtualKubeletStatus
			}, timeout, interval).Should(Equal(sharingv1alpha1.VirtualKubeletStatusNone))

			// check the deletion of the virtual node
			Eventually(func() int {
				var virtualNodeList virtualkubeletv1alpha1.VirtualNodeList
				err := controller.Client.List(ctx, &virtualNodeList)
				if err != nil {
					return -1
				}
				return len(virtualNodeList.Items)
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
})
