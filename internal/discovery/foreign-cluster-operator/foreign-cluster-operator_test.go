package foreignclusteroperator

import (
	"context"
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
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/liqotech/liqo/apis/config/v1alpha1"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	crdreplicator "github.com/liqotech/liqo/internal/crdReplicator"
	"github.com/liqotech/liqo/pkg/clusterid/test"
	"github.com/liqotech/liqo/pkg/discovery"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	peeringroles "github.com/liqotech/liqo/pkg/peering-roles"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	peeringconditionsutils "github.com/liqotech/liqo/pkg/utils/peeringConditions"
	testUtils "github.com/liqotech/liqo/pkg/utils/testUtils"
)

const (
	timeout  = time.Second * 30
	interval = time.Millisecond * 250
)

type configMock struct {
	config v1alpha1.DiscoveryConfig
}

func (c *configMock) GetConfig() *v1alpha1.DiscoveryConfig {
	c.config.AuthServiceAddress = "127.0.0.1"
	c.config.AuthServicePort = "8443"
	return &c.config
}

func (c *configMock) GetAPIServerConfig() *v1alpha1.APIServerConfig {
	return &v1alpha1.APIServerConfig{
		Address:   os.Getenv("APISERVER"),
		Port:      os.Getenv("APISERVER_PORT"),
		TrustedCA: false,
	}
}

func TestForeignClusterOperator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ForeignClusterOperator Suite")
}

var _ = Describe("ForeignClusterOperator", func() {

	var (
		cluster         testUtils.Cluster
		controller      ForeignClusterReconciler
		config          configMock
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
		cluster, mgr, err = testUtils.NewTestCluster([]string{filepath.Join("..", "..", "..", "deployments", "liqo", "crds")})
		if err != nil {
			By(err.Error())
			os.Exit(1)
		}

		cID := &test.ClusterIDMock{}
		_ = cID.SetupClusterID("default")

		namespaceManager := tenantnamespace.NewTenantNamespaceManager(cluster.GetClient().Client())
		identityManagerCtrl := identitymanager.NewCertificateIdentityManager(cluster.GetClient().Client(), cID, namespaceManager)

		tenantNamespace, err = namespaceManager.CreateNamespace("foreign-cluster")
		if err != nil {
			By(err.Error())
			os.Exit(1)
		}

		config.config = v1alpha1.DiscoveryConfig{
			AuthService:         "_liqo_auth._tcp",
			ClusterName:         "Name",
			AutoJoin:            true,
			AutoJoinUntrusted:   false,
			Domain:              "local.",
			EnableAdvertisement: false,
			EnableDiscovery:     false,
			Name:                "MyLiqo",
			Port:                6443,
			Service:             "_liqo_api._tcp",
			TTL:                 90,
		}

		controller = ForeignClusterReconciler{
			Client:           mgr.GetClient(),
			Scheme:           mgr.GetScheme(),
			Namespace:        "default",
			crdClient:        cluster.GetClient(),
			networkClient:    cluster.GetNetClient(),
			clusterID:        cID,
			ForeignConfig:    cluster.GetCfg(),
			RequeueAfter:     300,
			ConfigProvider:   &config,
			namespaceManager: namespaceManager,
			identityManager:  identityManagerCtrl,
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
				obj, err := controller.crdClient.Resource("foreignclusters").Create(&c.fc, &metav1.CreateOptions{})
				Expect(err).To(BeNil())
				Expect(obj).NotTo(BeNil())

				fc, ok := obj.(*discoveryv1alpha1.ForeignCluster)
				Expect(ok).To(BeTrue())
				Expect(fc).NotTo(BeNil())

				fc.Status = *c.fc.Status.DeepCopy()
				obj, err = controller.crdClient.Resource("foreignclusters").UpdateStatus(fc.Name, fc, &metav1.UpdateOptions{})
				Expect(err).To(BeNil())
				Expect(obj).NotTo(BeNil())

				fc, ok = obj.(*discoveryv1alpha1.ForeignCluster)
				Expect(ok).To(BeTrue())
				Expect(fc).NotTo(BeNil())

				// enable the peering for that foreigncluster
				err = controller.peerNamespaced(ctx, fc)
				Expect(err).To(BeNil())

				// check that the incoming and the outgoing statuses are the expected ones
				Expect(peeringconditionsutils.GetStatus(fc, discoveryv1alpha1.OutgoingPeeringCondition)).To(c.expectedOutgoing)
				Expect(peeringconditionsutils.GetStatus(fc, discoveryv1alpha1.IncomingPeeringCondition)).To(c.expectedIncoming)

				// get the resource requests in the local tenant namespace
				obj, err = controller.crdClient.Resource("resourcerequests").Namespace(tenantNamespace.Name).List(&metav1.ListOptions{})
				Expect(err).To(BeNil())
				Expect(obj).NotTo(BeNil())

				rrs, ok := obj.(*discoveryv1alpha1.ResourceRequestList)
				Expect(ok).To(BeTrue())
				Expect(rrs).NotTo(BeNil())

				// check that the length of the resource request list is the expected one,
				// and the resource request has been created in the correct namespace
				Expect(len(rrs.Items)).To(c.expectedPeeringLength)
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
						Namespace:     "liqo",
						DiscoveryType: discovery.ManualDiscovery,
						AuthURL:       "",
						TrustMode:     discovery.TrustModeUntrusted,
					},
					Status: discoveryv1alpha1.ForeignClusterStatus{
						TenantNamespace: defaultTenantNamespace,
					},
				},
				expectedPeeringLength: Equal(1),
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
				c.rr.Name = controller.clusterID.GetClusterID()
				c.rr.Spec.ClusterIdentity.ClusterID = c.fc.Spec.ClusterIdentity.ClusterID
				c.rr.Labels = resourceRequestLabels(c.fc.Spec.ClusterIdentity.ClusterID)

				// create the foreigncluster CR
				obj, err := controller.crdClient.Resource("foreignclusters").Create(&c.fc, &metav1.CreateOptions{})
				Expect(err).To(BeNil())
				Expect(obj).NotTo(BeNil())

				fc, ok := obj.(*discoveryv1alpha1.ForeignCluster)
				Expect(ok).To(BeTrue())
				Expect(fc).NotTo(BeNil())

				fc.Status = *c.fc.Status.DeepCopy()
				obj, err = controller.crdClient.Resource("foreignclusters").UpdateStatus(fc.Name, fc, &metav1.UpdateOptions{})
				Expect(err).To(BeNil())
				Expect(obj).NotTo(BeNil())

				fc, ok = obj.(*discoveryv1alpha1.ForeignCluster)
				Expect(ok).To(BeTrue())
				Expect(fc).NotTo(BeNil())

				// create the resourcerequest CR
				obj, err = controller.crdClient.Resource("resourcerequests").Namespace(tenantNamespace.Name).Create(&c.rr, &metav1.CreateOptions{})
				Expect(err).To(BeNil())
				Expect(obj).NotTo(BeNil())

				rr, ok := obj.(*discoveryv1alpha1.ResourceRequest)
				Expect(ok).To(BeTrue())
				Expect(rr).NotTo(BeNil())

				// set the ResourceRequest status to created
				obj, err = controller.crdClient.Resource("resourcerequests").Namespace(tenantNamespace.Name).UpdateStatus(rr.Name, rr, &metav1.UpdateOptions{})
				Expect(err).To(BeNil())
				Expect(obj).NotTo(BeNil())

				rr, ok = obj.(*discoveryv1alpha1.ResourceRequest)
				Expect(ok).To(BeTrue())
				Expect(rr).NotTo(BeNil())

				// disable the peering for that foreigncluster
				err = controller.unpeerNamespaced(ctx, fc)
				Expect(err).To(BeNil())

				// check that the incoming and the outgoing statuses are the expected ones
				Expect(peeringconditionsutils.GetStatus(fc, discoveryv1alpha1.OutgoingPeeringCondition)).To(c.expectedOutgoing)
				Expect(peeringconditionsutils.GetStatus(fc, discoveryv1alpha1.IncomingPeeringCondition)).To(c.expectedIncoming)

				// get the resource requests in the local tenant namespace
				obj, err = controller.crdClient.Resource("resourcerequests").Namespace(tenantNamespace.Name).List(&metav1.ListOptions{})
				Expect(err).To(BeNil())
				Expect(obj).NotTo(BeNil())

				rrs, ok := obj.(*discoveryv1alpha1.ResourceRequestList)
				Expect(ok).To(BeTrue())
				Expect(rrs).NotTo(BeNil())

				// check that the length of the resource request list is the expected one,
				// and the resource request has been set for deletion in the correct namespace
				Expect(len(rrs.Items)).To(c.expectedPeeringLength)
				if len(rrs.Items) > 0 {
					Expect(rrs.Items[0].Spec.WithdrawalTimestamp.IsZero()).To(BeFalse())
					rr = &rrs.Items[0]
				}

				// set the ResourceRequest status to deleted
				rr.Status.OfferWithdrawalTimestamp = &now
				obj, err = controller.crdClient.Resource("resourcerequests").Namespace(tenantNamespace.Name).UpdateStatus(rr.Name, rr, &metav1.UpdateOptions{})
				Expect(err).To(BeNil())
				Expect(obj).NotTo(BeNil())

				rr, ok = obj.(*discoveryv1alpha1.ResourceRequest)
				Expect(ok).To(BeTrue())
				Expect(rr).NotTo(BeNil())

				// call for the second time the unpeer function to delete the ResourceRequest
				err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
					// make sure to be working on the last ForeignCluster version
					err = controller.Client.Get(ctx, machtypes.NamespacedName{
						Name: fc.GetName(),
					}, fc)
					if err != nil {
						return err
					}

					return controller.unpeerNamespaced(ctx, fc)
				})
				Expect(err).To(BeNil())

				// get the resource requests in the local tenant namespace
				obj, err = controller.crdClient.Resource("resourcerequests").Namespace(tenantNamespace.Name).List(&metav1.ListOptions{})
				Expect(err).To(BeNil())
				Expect(obj).NotTo(BeNil())

				rrs, ok = obj.(*discoveryv1alpha1.ResourceRequestList)
				Expect(ok).To(BeTrue())
				Expect(rrs).NotTo(BeNil())

				Expect(len(rrs.Items)).To(BeNumerically("==", 0))
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
						Namespace:     "liqo",
						DiscoveryType: discovery.ManualDiscovery,
						AuthURL:       "",
						TrustMode:     discovery.TrustModeUntrusted,
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
				},
				expectedPeeringLength: Equal(1),
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
					DiscoveryType: discovery.ManualDiscovery,
					AuthURL:       "",
					TrustMode:     discovery.TrustModeUntrusted,
				},
			}

			client := mgr.GetClient()
			err := client.Create(ctx, foreignCluster)
			Expect(err).To(BeNil())

			err = controller.ensureLocalTenantNamespace(ctx, foreignCluster)
			Expect(err).To(BeNil())
			Expect(foreignCluster.Status.TenantNamespace.Local).ToNot(Equal(""))

			ns, err := controller.namespaceManager.GetNamespace(foreignCluster.Spec.ClusterIdentity.ClusterID)
			Expect(err).To(BeNil())
			Expect(ns).NotTo(BeNil())

			var namespace v1.Namespace
			err = client.Get(ctx, machtypes.NamespacedName{Name: foreignCluster.Status.TenantNamespace.Local}, &namespace)
			Expect(err).To(BeNil())

			Expect(namespace.Name).To(Equal(ns.Name))
		})

		type checkPeeringStatusTestcase struct {
			foreignClusterStatus  discoveryv1alpha1.ForeignClusterStatus
			resourceRequests      []discoveryv1alpha1.ResourceRequest
			expectedIncomingPhase discoveryv1alpha1.PeeringConditionStatusType
			expectedOutgoingPhase discoveryv1alpha1.PeeringConditionStatusType
		}

		var (
			getIncomingResourceRequest = func() discoveryv1alpha1.ResourceRequest {
				return discoveryv1alpha1.ResourceRequest{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "resource-request-incoming",
						Namespace: "default",
						Labels: map[string]string{
							crdreplicator.ReplicationStatuslabel: "true",
							crdreplicator.RemoteLabelSelector:    "foreign-cluster-abcd",
						},
					},
					Spec: discoveryv1alpha1.ResourceRequestSpec{
						ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
							ClusterID: "foreign-cluster-abcd",
						},
						AuthURL: "",
					},
				}
			}

			getOutgoingResourceRequest = func() discoveryv1alpha1.ResourceRequest {
				return discoveryv1alpha1.ResourceRequest{
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
			}
		)

		DescribeTable("checkPeeringStatus",
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
						DiscoveryType: discovery.ManualDiscovery,
						AuthURL:       "",
						TrustMode:     discovery.TrustModeUntrusted,
					},
				}

				client := mgr.GetClient()
				err := client.Create(ctx, foreignCluster)
				Expect(err).To(BeNil())

				foreignCluster.Status = c.foreignClusterStatus
				err = client.Status().Update(ctx, foreignCluster)
				Expect(err).To(BeNil())

				for i := range c.resourceRequests {
					err = client.Create(ctx, &c.resourceRequests[i])
					Expect(err).To(BeNil())
				}

				err = controller.checkPeeringStatus(ctx, foreignCluster)
				Expect(err).To(BeNil())

				Expect(peeringconditionsutils.GetStatus(foreignCluster, discoveryv1alpha1.IncomingPeeringCondition)).To(Equal(c.expectedIncomingPhase))
				Expect(peeringconditionsutils.GetStatus(foreignCluster, discoveryv1alpha1.OutgoingPeeringCondition)).To(Equal(c.expectedOutgoingPhase))
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
				expectedOutgoingPhase: discoveryv1alpha1.PeeringConditionStatusNone,
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
				expectedOutgoingPhase: discoveryv1alpha1.PeeringConditionStatusNone,
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
					getOutgoingResourceRequest(),
				},
				expectedIncomingPhase: discoveryv1alpha1.PeeringConditionStatusNone,
				expectedOutgoingPhase: discoveryv1alpha1.PeeringConditionStatusEstablished,
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
				expectedOutgoingPhase: discoveryv1alpha1.PeeringConditionStatusNone,
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
					getOutgoingResourceRequest(),
				},
				expectedIncomingPhase: discoveryv1alpha1.PeeringConditionStatusEstablished,
				expectedOutgoingPhase: discoveryv1alpha1.PeeringConditionStatusEstablished,
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
			fc               discoveryv1alpha1.ForeignCluster
			expectedOutgoing types.GomegaMatcher
			expectedIncoming types.GomegaMatcher
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

			controller.peeringPermission = peeringroles.PeeringPermission{
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

				By("Delete RoleBindings")

				// create all
				_, err := controller.namespaceManager.BindClusterRoles(c.fc.Spec.ClusterIdentity.ClusterID, &clusterRole1, &clusterRole2)
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
						Namespace:     "liqo",
						DiscoveryType: discovery.ManualDiscovery,
						AuthURL:       "",
						TrustMode:     discovery.TrustModeUntrusted,
					},
					Status: discoveryv1alpha1.ForeignClusterStatus{
						TenantNamespace: discoveryv1alpha1.TenantNamespaceType{},
					},
				},
				expectedOutgoing: Not(ContainElement(outgoingBinding)),
				expectedIncoming: Not(ContainElement(incomingBinding)),
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
						Namespace:     "liqo",
						DiscoveryType: discovery.ManualDiscovery,
						AuthURL:       "",
						TrustMode:     discovery.TrustModeUntrusted,
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
				expectedOutgoing: Not(ContainElement(outgoingBinding)),
				expectedIncoming: ContainElement(incomingBinding),
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
						Namespace:     "liqo",
						DiscoveryType: discovery.ManualDiscovery,
						AuthURL:       "",
						TrustMode:     discovery.TrustModeUntrusted,
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
				expectedOutgoing: ContainElement(outgoingBinding),
				expectedIncoming: Not(ContainElement(incomingBinding)),
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
						Namespace:     "liqo",
						DiscoveryType: discovery.ManualDiscovery,
						AuthURL:       "",
						TrustMode:     discovery.TrustModeUntrusted,
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
				expectedOutgoing: ContainElement(outgoingBinding),
				expectedIncoming: ContainElement(incomingBinding),
			}),
		)

	})

})
