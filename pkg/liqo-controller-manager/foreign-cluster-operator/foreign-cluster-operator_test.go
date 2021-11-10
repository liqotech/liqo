// Copyright 2019-2021 The Liqo Authors
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

package foreignclusteroperator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	machtypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/discovery"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	peeringroles "github.com/liqotech/liqo/pkg/peering-roles"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	peeringconditionsutils "github.com/liqotech/liqo/pkg/utils/peeringConditions"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

const (
	timeout  = time.Second * 30
	interval = time.Millisecond * 250
)

func TestForeignClusterOperator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ForeignClusterOperator Suite")
}

var _ = Describe("ForeignClusterOperator", func() {

	var (
		cluster         testutil.Cluster
		controller      ForeignClusterReconciler
		tenantNamespace *v1.Namespace
		mgr             manager.Manager
		ctx             context.Context
		cancel          context.CancelFunc

		now = metav1.Now()

		defaultTenantNamespace = discoveryv1alpha1.TenantNamespaceType{
			Local: "default",
		}
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		var err error
		cluster, mgr, err = testutil.NewTestCluster([]string{filepath.Join("..", "..", "..", "deployments", "liqo", "crds")})
		if err != nil {
			By(err.Error())
			os.Exit(1)
		}

		homeCluster := discoveryv1alpha1.ClusterIdentity{
			ClusterID:   "local-cluster-id",
			ClusterName: "local-cluster-name",
		}

		namespaceManager := tenantnamespace.NewTenantNamespaceManager(cluster.GetClient())
		identityManagerCtrl := identitymanager.NewCertificateIdentityManager(cluster.GetClient(), homeCluster.ClusterID, namespaceManager)

		clusterID := "foreign-cluster"
		tenantNamespace, err = namespaceManager.CreateNamespace(clusterID)
		if err != nil {
			By(err.Error())
			os.Exit(1)
		}
		// Make sure the namespace has been cached for subsequent retrieval.
		Eventually(func() (*v1.Namespace, error) { return namespaceManager.GetNamespace(clusterID) }).Should(Equal(tenantNamespace))

		controller = ForeignClusterReconciler{
			Client:           mgr.GetClient(),
			Scheme:           mgr.GetScheme(),
			HomeCluster:      homeCluster,
			ResyncPeriod:     300,
			NamespaceManager: namespaceManager,
			IdentityManager:  identityManagerCtrl,

			AuthServiceAddressOverride: "127.0.0.1",
			AuthServicePortOverride:    "8443",
		}

		go mgr.GetCache().Start(ctx)
	})

	AfterEach(func() {
		cancel()

		err := cluster.GetEnv().Stop()
		if err != nil {
			By(err.Error())
			os.Exit(1)
		}
	})

	// peer namespaced

	Context("Peer Namespaced", func() {

		type peerTestcase struct {
			fc                    discoveryv1alpha1.ForeignCluster
			expectedPeeringLength types.GomegaMatcher
			expectedOutgoing      types.GomegaMatcher
			expectedIncoming      types.GomegaMatcher
		}

		DescribeTable("Peer table",
			func(c peerTestcase) {
				// set the local namespace in the foreign cluster, we will only need the local one during the test
				c.fc.Status.TenantNamespace.Local = tenantNamespace.Name

				// create the foreigncluster CR
				fc := c.fc.DeepCopy()
				Expect(controller.Create(ctx, fc)).To(Succeed())

				fc.Status = *c.fc.Status.DeepCopy()
				Expect(controller.Status().Update(ctx, fc)).To(Succeed())

				// enable the peering for that foreigncluster
				Expect(controller.peerNamespaced(ctx, fc)).To(Succeed())

				// check that the incoming and the outgoing statuses are the expected ones
				Expect(peeringconditionsutils.GetStatus(fc, discoveryv1alpha1.OutgoingPeeringCondition)).To(c.expectedOutgoing)
				Expect(peeringconditionsutils.GetStatus(fc, discoveryv1alpha1.IncomingPeeringCondition)).To(c.expectedIncoming)

				// get the resource requests in the local tenant namespace
				rrs := discoveryv1alpha1.ResourceRequestList{}
				Eventually(func() error {
					if err := controller.List(ctx, &rrs, client.InNamespace(tenantNamespace.Name)); err != nil {
						return err
					}

					// check that the length of the resource request list is the expected one,
					// and the resource request has been created in the correct namespace
					if ok, err := c.expectedPeeringLength.Match(rrs.Items); !ok {
						return err
					}
					return nil
				}).Should(Succeed())
			},

			Entry("peer", peerTestcase{
				fc: discoveryv1alpha1.ForeignCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foreign-cluster",
						Labels: map[string]string{
							discovery.DiscoveryTypeLabel: string(discovery.ManualDiscovery),
							discovery.ClusterIDLabel:     "foreign-cluster",
						},
					},
					Spec: discoveryv1alpha1.ForeignClusterSpec{
						ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
							ClusterID:   "foreign-cluster",
							ClusterName: "ClusterTest2",
						},
						OutgoingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
						IncomingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
						ForeignAuthURL:         "https://example.com",
						InsecureSkipTLSVerify:  pointer.BoolPtr(true),
					},
					Status: discoveryv1alpha1.ForeignClusterStatus{
						TenantNamespace: defaultTenantNamespace,
					},
				},
				expectedPeeringLength: HaveLen(1),
				expectedOutgoing:      Equal(discoveryv1alpha1.PeeringConditionStatusPending), // we expect a joined flag set to true for the outgoing peering
				expectedIncoming:      Equal(discoveryv1alpha1.PeeringConditionStatusNone),
			}),
		)

	})

	// unpeer namespaced

	Context("Unpeer Namespaced", func() {

		type unpeerTestcase struct {
			fc                    discoveryv1alpha1.ForeignCluster
			rr                    discoveryv1alpha1.ResourceRequest
			expectedPeeringLength types.GomegaMatcher
			expectedOutgoing      types.GomegaMatcher
			expectedIncoming      types.GomegaMatcher
		}

		DescribeTable("Unpeer table",
			func(c unpeerTestcase) {
				// set the local namespace in the foreign cluster, we will only need the local one during the test
				c.fc.Status.TenantNamespace.Local = tenantNamespace.Name

				// populate the resourcerequest CR
				c.rr.Name = controller.HomeCluster.ClusterID
				c.rr.Namespace = tenantNamespace.Name
				c.rr.Spec.ClusterIdentity.ClusterID = c.fc.Spec.ClusterIdentity.ClusterID
				c.rr.Labels = resourceRequestLabels(c.fc.Spec.ClusterIdentity.ClusterID)

				// create the foreigncluster CR
				fc := c.fc.DeepCopy()
				Expect(controller.Create(ctx, fc)).To(Succeed())

				fc.Status = *c.fc.Status.DeepCopy()
				Expect(controller.Status().Update(ctx, fc)).To(Succeed())

				// create the resourcerequest CR
				rr := c.rr.DeepCopy()
				Expect(controller.Create(ctx, rr)).To(Succeed())

				// set the ResourceRequest status to created
				rr.Status = *c.rr.Status.DeepCopy()
				Expect(controller.Status().Update(ctx, rr)).To(Succeed())

				// disable the peering for that foreigncluster
				Expect(controller.unpeerNamespaced(ctx, fc)).To(Succeed())

				// check that the incoming and the outgoing statuses are the expected ones
				Expect(peeringconditionsutils.GetStatus(fc, discoveryv1alpha1.OutgoingPeeringCondition)).To(c.expectedOutgoing)
				Expect(peeringconditionsutils.GetStatus(fc, discoveryv1alpha1.IncomingPeeringCondition)).To(c.expectedIncoming)

				// get the resource requests in the local tenant namespace
				rrs := discoveryv1alpha1.ResourceRequestList{}
				Eventually(func() error {
					if err := controller.List(ctx, &rrs, client.InNamespace(tenantNamespace.Name)); err != nil {
						return err
					}

					// check that the length of the resource request list is the expected one.
					if ok, err := c.expectedPeeringLength.Match(rrs.Items); err != nil {
						return err
					} else if !ok {
						return fmt.Errorf("the peering length does not match the expected value")
					}

					// Check that the resource request has been set for deletion in the correct namespace
					if len(rrs.Items) > 0 {
						if ok, err := BeFalse().Match(rrs.Items[0].Spec.WithdrawalTimestamp.IsZero()); err != nil {
							return err
						} else if !ok {
							return fmt.Errorf("the withdrawal timestamp has not been set")
						}
						rr = &rrs.Items[0]
					}
					return nil
				}, timeout, interval).Should(Succeed())

				// set the ResourceRequest status to deleted
				rr.Status.OfferWithdrawalTimestamp = &now
				Expect(controller.Status().Update(ctx, rr)).To(Succeed())

				// call for the second time the unpeer function to delete the ResourceRequest
				err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
					// make sure to be working on the last ForeignCluster version
					err := controller.Client.Get(ctx, machtypes.NamespacedName{
						Name: fc.GetName(),
					}, fc)
					if err != nil {
						return err
					}

					return controller.unpeerNamespaced(ctx, fc)
				})
				Expect(err).To(BeNil())

				// get the resource requests in the local tenant namespace
				Eventually(func() error {
					if err := controller.List(ctx, &rrs, client.InNamespace(tenantNamespace.Name)); err != nil {
						return err
					}

					// check that no resource requests are present in the end.
					if ok, err := HaveLen(0).Match(rrs.Items); err != nil {
						return err
					} else if !ok {
						return fmt.Errorf("the peering length does not match the expected value")
					}
					return nil
				}, timeout, interval).Should(Succeed())
			},

			Entry("unpeer", unpeerTestcase{
				fc: discoveryv1alpha1.ForeignCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foreign-cluster",
						Labels: map[string]string{
							discovery.DiscoveryTypeLabel: string(discovery.ManualDiscovery),
							discovery.ClusterIDLabel:     "foreign-cluster",
						},
					},
					Spec: discoveryv1alpha1.ForeignClusterSpec{
						ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
							ClusterID:   "foreign-cluster",
							ClusterName: "ClusterTest2",
						},
						OutgoingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
						IncomingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
						ForeignAuthURL:         "https://example.com",
						InsecureSkipTLSVerify:  pointer.BoolPtr(true),
					},
					Status: discoveryv1alpha1.ForeignClusterStatus{
						PeeringConditions: []discoveryv1alpha1.PeeringCondition{
							{
								Type:               discoveryv1alpha1.OutgoingPeeringCondition,
								Status:             discoveryv1alpha1.PeeringConditionStatusEstablished,
								LastTransitionTime: metav1.Now(),
							},
						},
						TenantNamespace: discoveryv1alpha1.TenantNamespaceType{},
					},
				},
				rr: discoveryv1alpha1.ResourceRequest{
					ObjectMeta: metav1.ObjectMeta{
						Name: "",
					},
					Spec: discoveryv1alpha1.ResourceRequestSpec{
						ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
							ClusterID:   "",
							ClusterName: "Name",
						},
						AuthURL: "",
					},
					Status: discoveryv1alpha1.ResourceRequestStatus{
						OfferState: discoveryv1alpha1.OfferStateCreated,
					},
				},
				expectedPeeringLength: HaveLen(1),
				expectedOutgoing:      Equal(discoveryv1alpha1.PeeringConditionStatusDisconnecting),
				expectedIncoming:      Equal(discoveryv1alpha1.PeeringConditionStatusNone),
			}),

			Entry("unpeer from not accepted peering", unpeerTestcase{
				fc: discoveryv1alpha1.ForeignCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foreign-cluster",
						Labels: map[string]string{
							discovery.DiscoveryTypeLabel: string(discovery.ManualDiscovery),
							discovery.ClusterIDLabel:     "foreign-cluster",
						},
					},
					Spec: discoveryv1alpha1.ForeignClusterSpec{
						ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
							ClusterID:   "foreign-cluster",
							ClusterName: "ClusterTest2",
						},
						OutgoingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
						IncomingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
						ForeignAuthURL:         "https://example.com",
						InsecureSkipTLSVerify:  pointer.BoolPtr(true),
					},
					Status: discoveryv1alpha1.ForeignClusterStatus{
						PeeringConditions: []discoveryv1alpha1.PeeringCondition{
							{
								Type:               discoveryv1alpha1.OutgoingPeeringCondition,
								Status:             discoveryv1alpha1.PeeringConditionStatusPending,
								LastTransitionTime: metav1.Now(),
							},
						},
						TenantNamespace: discoveryv1alpha1.TenantNamespaceType{},
					},
				},
				rr: discoveryv1alpha1.ResourceRequest{
					ObjectMeta: metav1.ObjectMeta{
						Name: "",
					},
					Spec: discoveryv1alpha1.ResourceRequestSpec{
						ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
							ClusterID:   "",
							ClusterName: "Name",
						},
						AuthURL: "",
					},
					Status: discoveryv1alpha1.ResourceRequestStatus{
						OfferState: discoveryv1alpha1.OfferStateNone,
					},
				},
				expectedPeeringLength: HaveLen(1),
				expectedOutgoing:      Equal(discoveryv1alpha1.PeeringConditionStatusDisconnecting),
				expectedIncoming:      Equal(discoveryv1alpha1.PeeringConditionStatusNone),
			}),
		)

	})

	Context("Test Reconciler functions", func() {

		It("Create Tenant Namespace", func() {
			foreignCluster := &discoveryv1alpha1.ForeignCluster{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ForeignCluster",
					APIVersion: discoveryv1alpha1.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "foreign-cluster",
					Labels: map[string]string{
						discovery.DiscoveryTypeLabel: string(discovery.ManualDiscovery),
						discovery.ClusterIDLabel:     "foreign-cluster-abcd",
					},
				},
				Spec: discoveryv1alpha1.ForeignClusterSpec{
					ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
						ClusterID: "foreign-cluster-abcd",
					},
					OutgoingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
					IncomingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
					ForeignAuthURL:         "https://example.com",
					InsecureSkipTLSVerify:  pointer.BoolPtr(true),
				},
			}

			client := mgr.GetClient()
			err := client.Create(ctx, foreignCluster)
			Expect(err).To(BeNil())

			err = controller.ensureLocalTenantNamespace(ctx, foreignCluster)
			Expect(err).To(BeNil())
			Expect(foreignCluster.Status.TenantNamespace.Local).ToNot(Equal(""))

			var ns *v1.Namespace
			Eventually(func() error {
				ns, err = controller.NamespaceManager.GetNamespace(foreignCluster.Spec.ClusterIdentity.ClusterID)
				return err
			}).Should(Succeed())

			var namespace v1.Namespace
			err = client.Get(ctx, machtypes.NamespacedName{Name: foreignCluster.Status.TenantNamespace.Local}, &namespace)
			Expect(err).To(BeNil())

			Expect(namespace.Name).To(Equal(ns.Name))
		})

		type checkPeeringStatusTestcase struct {
			foreignClusterStatus  discoveryv1alpha1.ForeignClusterStatus
			resourceRequests      []discoveryv1alpha1.ResourceRequest
			expectedIncomingPhase discoveryv1alpha1.PeeringConditionStatusType
		}

		var (
			getIncomingResourceRequest = func() discoveryv1alpha1.ResourceRequest {
				return discoveryv1alpha1.ResourceRequest{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "resource-request-incoming",
						Namespace: "default",
						Labels: map[string]string{
							consts.ReplicationStatusLabel: "true",
							consts.ReplicationOriginLabel: "foreign-cluster-abcd",
						},
					},
					Spec: discoveryv1alpha1.ResourceRequestSpec{
						ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
							ClusterID: "foreign-cluster-abcd",
						},
						AuthURL: "",
					},
					Status: discoveryv1alpha1.ResourceRequestStatus{
						OfferState: discoveryv1alpha1.OfferStateCreated,
					},
				}
			}

			getOutgoingResourceRequest = func(accepted bool) discoveryv1alpha1.ResourceRequest {
				rr := discoveryv1alpha1.ResourceRequest{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "resource-request-outgoing",
						Namespace: "default",
						Labels:    resourceRequestLabels("foreign-cluster-abcd"),
					},
					Spec: discoveryv1alpha1.ResourceRequestSpec{
						ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
							ClusterID: "local-id",
						},
						AuthURL: "",
					},
				}
				if accepted {
					rr.Status = discoveryv1alpha1.ResourceRequestStatus{
						OfferState: discoveryv1alpha1.OfferStateCreated,
					}
				} else {
					rr.Status = discoveryv1alpha1.ResourceRequestStatus{
						OfferState:               discoveryv1alpha1.OfferStateNone,
						OfferWithdrawalTimestamp: &now,
					}
				}
				return rr
			}
		)

		DescribeTable("checkIncomingPeeringStatus",
			func(c checkPeeringStatusTestcase) {
				foreignCluster := &discoveryv1alpha1.ForeignCluster{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ForeignCluster",
						APIVersion: discoveryv1alpha1.GroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "foreign-cluster",
						Labels: map[string]string{
							discovery.DiscoveryTypeLabel: string(discovery.ManualDiscovery),
							discovery.ClusterIDLabel:     "foreign-cluster-abcd",
						},
					},
					Spec: discoveryv1alpha1.ForeignClusterSpec{
						ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
							ClusterID: "foreign-cluster-abcd",
						},
						OutgoingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
						IncomingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
						ForeignAuthURL:         "https://example.com",
						InsecureSkipTLSVerify:  pointer.BoolPtr(true),
					},
				}

				client := mgr.GetClient()
				err := client.Create(ctx, foreignCluster)
				Expect(err).To(BeNil())

				foreignCluster.Status = c.foreignClusterStatus
				err = client.Status().Update(ctx, foreignCluster)
				Expect(err).To(BeNil())

				for i := range c.resourceRequests {
					rr := c.resourceRequests[i].DeepCopy()
					err = client.Create(ctx, rr)
					Expect(err).To(Succeed())

					rr.Status = *c.resourceRequests[i].Status.DeepCopy()
					err = client.Status().Update(ctx, rr)
					Expect(err).To(Succeed())
				}

				err = controller.checkIncomingPeeringStatus(ctx, foreignCluster)
				Expect(err).To(BeNil())

				Expect(peeringconditionsutils.GetStatus(foreignCluster, discoveryv1alpha1.IncomingPeeringCondition)).To(Equal(c.expectedIncomingPhase))
			},

			Entry("none", checkPeeringStatusTestcase{
				foreignClusterStatus: discoveryv1alpha1.ForeignClusterStatus{
					TenantNamespace: defaultTenantNamespace,
					PeeringConditions: []discoveryv1alpha1.PeeringCondition{
						{
							Type:               discoveryv1alpha1.IncomingPeeringCondition,
							Status:             discoveryv1alpha1.PeeringConditionStatusEstablished,
							LastTransitionTime: metav1.Now(),
						},
						{
							Type:               discoveryv1alpha1.OutgoingPeeringCondition,
							Status:             discoveryv1alpha1.PeeringConditionStatusDisconnecting,
							LastTransitionTime: metav1.Now(),
						},
					},
				},
				resourceRequests:      []discoveryv1alpha1.ResourceRequest{},
				expectedIncomingPhase: discoveryv1alpha1.PeeringConditionStatusNone,
			}),

			Entry("none and no update", checkPeeringStatusTestcase{
				foreignClusterStatus: discoveryv1alpha1.ForeignClusterStatus{
					TenantNamespace: defaultTenantNamespace,
					PeeringConditions: []discoveryv1alpha1.PeeringCondition{
						{
							Type:               discoveryv1alpha1.IncomingPeeringCondition,
							Status:             discoveryv1alpha1.PeeringConditionStatusNone,
							LastTransitionTime: metav1.Now(),
						},
						{
							Type:               discoveryv1alpha1.OutgoingPeeringCondition,
							Status:             discoveryv1alpha1.PeeringConditionStatusNone,
							LastTransitionTime: metav1.Now(),
						},
					},
				},
				resourceRequests:      []discoveryv1alpha1.ResourceRequest{},
				expectedIncomingPhase: discoveryv1alpha1.PeeringConditionStatusNone,
			}),

			Entry("outgoing", checkPeeringStatusTestcase{
				foreignClusterStatus: discoveryv1alpha1.ForeignClusterStatus{
					TenantNamespace: defaultTenantNamespace,
					PeeringConditions: []discoveryv1alpha1.PeeringCondition{
						{
							Type:               discoveryv1alpha1.IncomingPeeringCondition,
							Status:             discoveryv1alpha1.PeeringConditionStatusNone,
							LastTransitionTime: metav1.Now(),
						},
						{
							Type:               discoveryv1alpha1.OutgoingPeeringCondition,
							Status:             discoveryv1alpha1.PeeringConditionStatusPending,
							LastTransitionTime: metav1.Now(),
						},
					},
				},
				resourceRequests: []discoveryv1alpha1.ResourceRequest{
					getOutgoingResourceRequest(true),
				},
				expectedIncomingPhase: discoveryv1alpha1.PeeringConditionStatusNone,
			}),

			Entry("outgoing not accepted", checkPeeringStatusTestcase{
				foreignClusterStatus: discoveryv1alpha1.ForeignClusterStatus{
					TenantNamespace: defaultTenantNamespace,
					PeeringConditions: []discoveryv1alpha1.PeeringCondition{
						{
							Type:               discoveryv1alpha1.IncomingPeeringCondition,
							Status:             discoveryv1alpha1.PeeringConditionStatusNone,
							LastTransitionTime: metav1.Now(),
						},
						{
							Type:               discoveryv1alpha1.OutgoingPeeringCondition,
							Status:             discoveryv1alpha1.PeeringConditionStatusEstablished,
							LastTransitionTime: metav1.Now(),
						},
					},
				},
				resourceRequests: []discoveryv1alpha1.ResourceRequest{
					getOutgoingResourceRequest(false),
				},
				expectedIncomingPhase: discoveryv1alpha1.PeeringConditionStatusNone,
			}),

			Entry("incoming", checkPeeringStatusTestcase{
				foreignClusterStatus: discoveryv1alpha1.ForeignClusterStatus{
					TenantNamespace: defaultTenantNamespace,
					PeeringConditions: []discoveryv1alpha1.PeeringCondition{
						{
							Type:               discoveryv1alpha1.IncomingPeeringCondition,
							Status:             discoveryv1alpha1.PeeringConditionStatusPending,
							LastTransitionTime: metav1.Now(),
						},
						{
							Type:               discoveryv1alpha1.OutgoingPeeringCondition,
							Status:             discoveryv1alpha1.PeeringConditionStatusNone,
							LastTransitionTime: metav1.Now(),
						},
					},
				},
				resourceRequests: []discoveryv1alpha1.ResourceRequest{
					getIncomingResourceRequest(),
				},
				expectedIncomingPhase: discoveryv1alpha1.PeeringConditionStatusEstablished,
			}),

			Entry("bidirectional", checkPeeringStatusTestcase{
				foreignClusterStatus: discoveryv1alpha1.ForeignClusterStatus{
					TenantNamespace: defaultTenantNamespace,
					PeeringConditions: []discoveryv1alpha1.PeeringCondition{
						{
							Type:               discoveryv1alpha1.IncomingPeeringCondition,
							Status:             discoveryv1alpha1.PeeringConditionStatusPending,
							LastTransitionTime: metav1.Now(),
						},
						{
							Type:               discoveryv1alpha1.OutgoingPeeringCondition,
							Status:             discoveryv1alpha1.PeeringConditionStatusPending,
							LastTransitionTime: metav1.Now(),
						},
					},
				},
				resourceRequests: []discoveryv1alpha1.ResourceRequest{
					getIncomingResourceRequest(),
					getOutgoingResourceRequest(true),
				},
				expectedIncomingPhase: discoveryv1alpha1.PeeringConditionStatusEstablished,
			}),
		)

	})

	Context("Test Permission", func() {

		const (
			outgoingBinding = "liqo-binding-liqo-outgoing"
			incomingBinding = "liqo-binding-liqo-incoming"
		)

		var (
			clusterRole1 rbacv1.ClusterRole
			clusterRole2 rbacv1.ClusterRole
		)

		type permissionTestcase struct {
			fc                          discoveryv1alpha1.ForeignCluster
			expectedOutgoing            types.GomegaMatcher
			expectedIncoming            types.GomegaMatcher
			expectedOutgoingClusterWide types.GomegaMatcher
		}

		JustBeforeEach(func() {
			clusterRole1 = rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "liqo-outgoing",
				},
			}
			clusterRole2 = rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "liqo-incoming",
				},
			}

			Expect(controller.Client.Create(ctx, &clusterRole1)).To(Succeed())
			Expect(controller.Client.Create(ctx, &clusterRole2)).To(Succeed())

			controller.PeeringPermission = peeringroles.PeeringPermission{
				Basic: []*rbacv1.ClusterRole{},
				Incoming: []*rbacv1.ClusterRole{
					&clusterRole2,
				},
				Outgoing: []*rbacv1.ClusterRole{
					&clusterRole1,
				},
			}
		})

		JustAfterEach(func() {
			var roleBindingList rbacv1.RoleBindingList
			Expect(controller.Client.List(ctx, &roleBindingList)).To(Succeed())
			for i := range roleBindingList.Items {
				rb := &roleBindingList.Items[i]
				Expect(controller.Client.Delete(ctx, rb)).To(Succeed())
			}

			Expect(controller.Client.Delete(ctx, &clusterRole1)).To(Succeed())
			Expect(controller.Client.Delete(ctx, &clusterRole2)).To(Succeed())
		})

		DescribeTable("permission table",
			func(c permissionTestcase) {
				var checkClusterWidePermission = func(expected types.GomegaMatcher) {
					clusterRoleName := fmt.Sprintf("liqo-remote-cluster-role-%v", c.fc.Spec.ClusterIdentity.ClusterID)
					var clusterRoleBinding rbacv1.ClusterRoleBinding
					Eventually(
						func() error {
							return controller.Client.Get(ctx, machtypes.NamespacedName{Name: clusterRoleName}, &clusterRoleBinding)
						},
						timeout, interval,
					).Should(expected)
					var clusterRole rbacv1.ClusterRole
					Eventually(
						func() error {
							return controller.Client.Get(ctx, machtypes.NamespacedName{Name: clusterRoleName}, &clusterRole)
						},
						timeout, interval,
					).Should(expected)
				}

				var checkBindings = func(ownerReferencesPermissionEnforcement bool) {
					controller.OwnerReferencesPermissionEnforcement = ownerReferencesPermissionEnforcement
					c.fc.Status.TenantNamespace.Local = tenantNamespace.Name

					By("Create RoleBindings")

					Expect(controller.ensurePermission(ctx, &c.fc)).To(Succeed())

					var roleBindingList rbacv1.RoleBindingList
					Eventually(func() []string {
						Expect(controller.Client.List(ctx, &roleBindingList)).To(Succeed())

						names := make([]string, len(roleBindingList.Items))
						for i := range roleBindingList.Items {
							if roleBindingList.Items[i].DeletionTimestamp.IsZero() {
								names[i] = roleBindingList.Items[i].Name
							}
						}
						return names
					}, timeout, interval).Should(And(c.expectedIncoming, c.expectedOutgoing))

					if ownerReferencesPermissionEnforcement {
						checkClusterWidePermission(c.expectedOutgoingClusterWide)
					}

					By("Delete RoleBindings")

					// create all
					_, err := controller.NamespaceManager.BindClusterRoles(c.fc.Spec.ClusterIdentity.ClusterID, &clusterRole1, &clusterRole2)
					Expect(err).To(Succeed())

					Expect(controller.ensurePermission(ctx, &c.fc)).To(Succeed())

					Eventually(func() []string {
						Expect(controller.Client.List(ctx, &roleBindingList)).To(Succeed())

						names := make([]string, len(roleBindingList.Items))
						for i := range roleBindingList.Items {
							if roleBindingList.Items[i].DeletionTimestamp.IsZero() {
								names[i] = roleBindingList.Items[i].Name
							}
						}
						return names
					}, timeout, interval).Should(And(c.expectedIncoming, c.expectedOutgoing))

					if ownerReferencesPermissionEnforcement {
						checkClusterWidePermission(c.expectedOutgoingClusterWide)
					}
				}

				checkBindings(false)
				checkBindings(true)
			},

			Entry("none peering", permissionTestcase{
				fc: discoveryv1alpha1.ForeignCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foreign-cluster",
						Labels: map[string]string{
							discovery.DiscoveryTypeLabel: string(discovery.ManualDiscovery),
							discovery.ClusterIDLabel:     "foreign-cluster",
						},
					},
					Spec: discoveryv1alpha1.ForeignClusterSpec{
						ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
							ClusterID:   "foreign-cluster",
							ClusterName: "ClusterTest",
						},
						OutgoingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
						IncomingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
						ForeignAuthURL:         "https://example.com",
						InsecureSkipTLSVerify:  pointer.BoolPtr(true),
					},
					Status: discoveryv1alpha1.ForeignClusterStatus{
						TenantNamespace: discoveryv1alpha1.TenantNamespaceType{},
					},
				},
				expectedOutgoing:            Not(ContainElement(outgoingBinding)),
				expectedIncoming:            Not(ContainElement(incomingBinding)),
				expectedOutgoingClusterWide: HaveOccurred(),
			}),

			Entry("incoming peering", permissionTestcase{
				fc: discoveryv1alpha1.ForeignCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foreign-cluster",
						Labels: map[string]string{
							discovery.DiscoveryTypeLabel: string(discovery.ManualDiscovery),
							discovery.ClusterIDLabel:     "foreign-cluster",
						},
					},
					Spec: discoveryv1alpha1.ForeignClusterSpec{
						ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
							ClusterID:   "foreign-cluster",
							ClusterName: "ClusterTest",
						},
						OutgoingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
						IncomingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
						ForeignAuthURL:         "https://example.com",
						InsecureSkipTLSVerify:  pointer.BoolPtr(true),
					},
					Status: discoveryv1alpha1.ForeignClusterStatus{
						TenantNamespace: discoveryv1alpha1.TenantNamespaceType{},
						PeeringConditions: []discoveryv1alpha1.PeeringCondition{
							{
								Type:               discoveryv1alpha1.IncomingPeeringCondition,
								Status:             discoveryv1alpha1.PeeringConditionStatusEstablished,
								LastTransitionTime: metav1.Now(),
							},
							{
								Type:               discoveryv1alpha1.OutgoingPeeringCondition,
								Status:             discoveryv1alpha1.PeeringConditionStatusNone,
								LastTransitionTime: metav1.Now(),
							},
						},
					},
				},
				expectedOutgoing:            Not(ContainElement(outgoingBinding)),
				expectedIncoming:            ContainElement(incomingBinding),
				expectedOutgoingClusterWide: HaveOccurred(),
			}),

			Entry("outgoing peering", permissionTestcase{
				fc: discoveryv1alpha1.ForeignCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foreign-cluster",
						Labels: map[string]string{
							discovery.DiscoveryTypeLabel: string(discovery.ManualDiscovery),
							discovery.ClusterIDLabel:     "foreign-cluster",
						},
					},
					Spec: discoveryv1alpha1.ForeignClusterSpec{
						ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
							ClusterID:   "foreign-cluster",
							ClusterName: "ClusterTest",
						},
						OutgoingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
						IncomingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
						ForeignAuthURL:         "https://example.com",
						InsecureSkipTLSVerify:  pointer.BoolPtr(true),
					},
					Status: discoveryv1alpha1.ForeignClusterStatus{
						TenantNamespace: discoveryv1alpha1.TenantNamespaceType{},
						PeeringConditions: []discoveryv1alpha1.PeeringCondition{
							{
								Type:               discoveryv1alpha1.IncomingPeeringCondition,
								Status:             discoveryv1alpha1.PeeringConditionStatusNone,
								LastTransitionTime: metav1.Now(),
							},
							{
								Type:               discoveryv1alpha1.OutgoingPeeringCondition,
								Status:             discoveryv1alpha1.PeeringConditionStatusEstablished,
								LastTransitionTime: metav1.Now(),
							},
						},
					},
				},
				expectedOutgoing:            ContainElement(outgoingBinding),
				expectedIncoming:            Not(ContainElement(incomingBinding)),
				expectedOutgoingClusterWide: Not(HaveOccurred()),
			}),

			Entry("bidirectional peering", permissionTestcase{
				fc: discoveryv1alpha1.ForeignCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foreign-cluster",
						Labels: map[string]string{
							discovery.DiscoveryTypeLabel: string(discovery.ManualDiscovery),
							discovery.ClusterIDLabel:     "foreign-cluster",
						},
					},
					Spec: discoveryv1alpha1.ForeignClusterSpec{
						ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
							ClusterID:   "foreign-cluster",
							ClusterName: "ClusterTest",
						},
						OutgoingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
						IncomingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
						ForeignAuthURL:         "https://example.com",
						InsecureSkipTLSVerify:  pointer.BoolPtr(true),
					},
					Status: discoveryv1alpha1.ForeignClusterStatus{
						TenantNamespace: discoveryv1alpha1.TenantNamespaceType{},
						PeeringConditions: []discoveryv1alpha1.PeeringCondition{
							{
								Type:               discoveryv1alpha1.IncomingPeeringCondition,
								Status:             discoveryv1alpha1.PeeringConditionStatusEstablished,
								LastTransitionTime: metav1.Now(),
							},
							{
								Type:               discoveryv1alpha1.OutgoingPeeringCondition,
								Status:             discoveryv1alpha1.PeeringConditionStatusEstablished,
								LastTransitionTime: metav1.Now(),
							},
						},
					},
				},
				expectedOutgoing:            ContainElement(outgoingBinding),
				expectedIncoming:            ContainElement(incomingBinding),
				expectedOutgoingClusterWide: Not(HaveOccurred()),
			}),
		)

	})

	Context("Test isClusterProcessable", func() {

		It("multiple ForeignClusters with the same clusterID", func() {

			fc1 := &discoveryv1alpha1.ForeignCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-1",
					Labels: map[string]string{
						discovery.DiscoveryTypeLabel: string(discovery.ManualDiscovery),
						discovery.ClusterIDLabel:     "cluster-1",
					},
				},
				Spec: discoveryv1alpha1.ForeignClusterSpec{
					OutgoingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
					IncomingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
					InsecureSkipTLSVerify:  pointer.BoolPtr(true),
					ForeignAuthURL:         "https://example.com",
					ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
						ClusterID: "cluster-1",
					},
				},
			}

			fc2 := fc1.DeepCopy()
			fc2.Name = "cluster-2"

			Expect(controller.Client.Create(ctx, fc1)).To(Succeed())
			// we need at least 1 second of delay between the two creation timestamps
			time.Sleep(1 * time.Second)
			Expect(controller.Client.Create(ctx, fc2)).To(Succeed())

			By("Create the first ForeignCluster")

			processable, err := controller.isClusterProcessable(ctx, fc1)
			Expect(err).To(Succeed())
			Expect(processable).To(BeTrue())
			Expect(peeringconditionsutils.GetStatus(fc1, discoveryv1alpha1.ProcessForeignClusterStatusCondition)).
				To(Equal(discoveryv1alpha1.PeeringConditionStatusSuccess))

			By("Create the second ForeignCluster")

			processable, err = controller.isClusterProcessable(ctx, fc2)
			Expect(err).To(Succeed())
			Expect(processable).To(BeFalse())
			Expect(peeringconditionsutils.GetStatus(fc2, discoveryv1alpha1.ProcessForeignClusterStatusCondition)).
				To(Equal(discoveryv1alpha1.PeeringConditionStatusError))

			By("Delete the first ForeignCluster")

			Expect(controller.Client.Delete(ctx, fc1)).To(Succeed())

			By("Check that the second ForeignCluster is now processable")

			Eventually(func() bool {
				processable, err = controller.isClusterProcessable(ctx, fc2)
				Expect(err).To(Succeed())
				return processable
			}, timeout, interval).Should(BeTrue())
			Expect(peeringconditionsutils.GetStatus(fc2, discoveryv1alpha1.ProcessForeignClusterStatusCondition)).
				To(Equal(discoveryv1alpha1.PeeringConditionStatusSuccess))
		})

		It("add a cluster with the same clusterID of the local cluster", func() {

			fc := &discoveryv1alpha1.ForeignCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-1",
					Labels: map[string]string{
						discovery.DiscoveryTypeLabel: string(discovery.ManualDiscovery),
						discovery.ClusterIDLabel:     controller.HomeCluster.ClusterID,
					},
				},
				Spec: discoveryv1alpha1.ForeignClusterSpec{
					OutgoingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
					IncomingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
					InsecureSkipTLSVerify:  pointer.BoolPtr(true),
					ForeignAuthURL:         "https://example.com",
					ClusterIdentity:        controller.HomeCluster,
				},
			}

			Expect(controller.Client.Create(ctx, fc)).To(Succeed())

			processable, err := controller.isClusterProcessable(ctx, fc)
			Expect(err).To(Succeed())
			Expect(processable).To(BeFalse())
			Expect(peeringconditionsutils.GetStatus(fc, discoveryv1alpha1.ProcessForeignClusterStatusCondition)).
				To(Equal(discoveryv1alpha1.PeeringConditionStatusError))

		})

	})

})

var _ = Describe("PeeringPolicy", func() {

	var (
		controller ForeignClusterReconciler
	)

	BeforeEach(func() {
		controller = ForeignClusterReconciler{
			AutoJoin: true,
		}
	})

	Context("check isPeeringEnabled", func() {

		type isPeeringEnabledTestcase struct {
			foreignCluster discoveryv1alpha1.ForeignCluster
			expected       types.GomegaMatcher
		}

		DescribeTable("isPeeringEnabled table",
			func(c isPeeringEnabledTestcase) {
				Expect(controller.isOutgoingPeeringEnabled(context.TODO(), &c.foreignCluster)).To(c.expected)
			},

			Entry("peering disabled", isPeeringEnabledTestcase{
				foreignCluster: discoveryv1alpha1.ForeignCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foreign-cluster",
						Labels: map[string]string{
							discovery.DiscoveryTypeLabel: string(discovery.ManualDiscovery),
							discovery.ClusterIDLabel:     "foreign-cluster",
						},
					},
					Spec: discoveryv1alpha1.ForeignClusterSpec{
						OutgoingPeeringEnabled: discoveryv1alpha1.PeeringEnabledNo,
						IncomingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
						InsecureSkipTLSVerify:  pointer.BoolPtr(true),
					},
				},
				expected: BeFalse(),
			}),

			Entry("peering enabled", isPeeringEnabledTestcase{
				foreignCluster: discoveryv1alpha1.ForeignCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foreign-cluster",
						Labels: map[string]string{
							discovery.DiscoveryTypeLabel: string(discovery.ManualDiscovery),
							discovery.ClusterIDLabel:     "foreign-cluster",
						},
					},
					Spec: discoveryv1alpha1.ForeignClusterSpec{
						OutgoingPeeringEnabled: discoveryv1alpha1.PeeringEnabledYes,
						IncomingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
						InsecureSkipTLSVerify:  pointer.BoolPtr(true),
					},
				},
				expected: BeTrue(),
			}),

			Entry("peering automatic with manual discovery", isPeeringEnabledTestcase{
				foreignCluster: discoveryv1alpha1.ForeignCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foreign-cluster",
						Labels: map[string]string{
							discovery.DiscoveryTypeLabel: string(discovery.ManualDiscovery),
							discovery.ClusterIDLabel:     "foreign-cluster",
						},
					},
					Spec: discoveryv1alpha1.ForeignClusterSpec{
						OutgoingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
						IncomingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
						InsecureSkipTLSVerify:  pointer.BoolPtr(true),
					},
				},
				expected: BeFalse(),
			}),

			Entry("peering automatic with incoming discovery", isPeeringEnabledTestcase{
				foreignCluster: discoveryv1alpha1.ForeignCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foreign-cluster",
						Labels: map[string]string{
							discovery.DiscoveryTypeLabel: string(discovery.IncomingPeeringDiscovery),
							discovery.ClusterIDLabel:     "foreign-cluster",
						},
					},
					Spec: discoveryv1alpha1.ForeignClusterSpec{
						OutgoingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
						IncomingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
						InsecureSkipTLSVerify:  pointer.BoolPtr(true),
					},
				},
				expected: BeFalse(),
			}),

			Entry("peering automatic with LAN discovery", isPeeringEnabledTestcase{
				foreignCluster: discoveryv1alpha1.ForeignCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foreign-cluster",
						Labels: map[string]string{
							discovery.DiscoveryTypeLabel: string(discovery.LanDiscovery),
							discovery.ClusterIDLabel:     "foreign-cluster",
						},
					},
					Spec: discoveryv1alpha1.ForeignClusterSpec{
						OutgoingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
						IncomingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
						InsecureSkipTLSVerify:  pointer.BoolPtr(true),
					},
				},
				expected: BeTrue(),
			}),

			Entry("foreign cluster with deletion timestamp set", isPeeringEnabledTestcase{
				foreignCluster: discoveryv1alpha1.ForeignCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foreign-cluster",
						Labels: map[string]string{
							discovery.DiscoveryTypeLabel: string(discovery.LanDiscovery),
							discovery.ClusterIDLabel:     "foreign-cluster",
						},
						DeletionTimestamp: &metav1.Time{Time: time.Now()},
					},
					Spec: discoveryv1alpha1.ForeignClusterSpec{
						OutgoingPeeringEnabled: discoveryv1alpha1.PeeringEnabledYes,
						IncomingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
						InsecureSkipTLSVerify:  pointer.BoolPtr(true),
					},
				},
				expected: BeFalse(),
			}),
		)

	})

})
