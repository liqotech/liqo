package foreignclusteroperator

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	v1 "k8s.io/api/core/v1"
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
	tenantcontrolnamespace "github.com/liqotech/liqo/pkg/tenantControlNamespace"
	testUtils "github.com/liqotech/liqo/pkg/utils/testUtils"
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

		namespaceManager := tenantcontrolnamespace.NewTenantControlNamespaceManager(cluster.GetClient().Client())
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
			Client:              mgr.GetClient(),
			Scheme:              mgr.GetScheme(),
			Namespace:           "default",
			crdClient:           cluster.GetClient(),
			advertisementClient: cluster.GetAdvClient(),
			networkClient:       cluster.GetNetClient(),
			clusterID:           cID,
			ForeignConfig:       cluster.GetCfg(),
			RequeueAfter:        300,
			ConfigProvider:      &config,
			namespaceManager:    namespaceManager,
			identityManager:     identityManagerCtrl,
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

	// peer

	Context("Peer", func() {

		type peerTestcase struct {
			fc                    discoveryv1alpha1.ForeignCluster
			expectedPeeringLength types.GomegaMatcher
			expectedOutgoing      types.GomegaMatcher
			expectedIncoming      types.GomegaMatcher
		}

		DescribeTable("Peer table",
			func(c peerTestcase) {
				obj, err := controller.crdClient.Resource("foreignclusters").Create(&c.fc, &metav1.CreateOptions{})
				Expect(err).To(BeNil())
				Expect(obj).NotTo(BeNil())

				fc, ok := obj.(*discoveryv1alpha1.ForeignCluster)
				Expect(ok).To(BeTrue())
				Expect(fc).NotTo(BeNil())

				fc, err = controller.Peer(fc, cluster.GetClient())
				Expect(err).To(BeNil())
				Expect(fc).NotTo(BeNil())

				Expect(fc.Status.Outgoing).To(c.expectedOutgoing)
				Expect(fc.Status.Incoming).To(c.expectedIncoming)

				obj, err = controller.crdClient.Resource("peeringrequests").List(&metav1.ListOptions{})
				Expect(err).To(BeNil())
				Expect(obj).NotTo(BeNil())

				prs, ok := obj.(*discoveryv1alpha1.PeeringRequestList)
				Expect(ok).To(BeTrue())
				Expect(prs).NotTo(BeNil())

				Expect(len(prs.Items)).To(c.expectedPeeringLength)
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
				},
				expectedPeeringLength: Equal(1),
				expectedOutgoing: Equal(discoveryv1alpha1.Outgoing{
					PeeringPhase:             discoveryv1alpha1.PeeringPhaseEstablished,
					RemotePeeringRequestName: "local-cluster",
				}),
				expectedIncoming: Equal(discoveryv1alpha1.Incoming{}),
			}),
		)

	})

	// unpeer

	Context("Unpeer", func() {

		type unpeerTestcase struct {
			fc                    discoveryv1alpha1.ForeignCluster
			pr                    discoveryv1alpha1.PeeringRequest
			expectedPeeringLength types.GomegaMatcher
			expectedOutgoing      types.GomegaMatcher
			expectedIncoming      types.GomegaMatcher
		}

		DescribeTable("Unpeer table",
			func(c unpeerTestcase) {
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

				obj, err = controller.crdClient.Resource("peeringrequests").Create(&c.pr, &metav1.CreateOptions{})
				Expect(err).To(BeNil())
				Expect(obj).NotTo(BeNil())

				pr, ok := obj.(*discoveryv1alpha1.PeeringRequest)
				Expect(ok).To(BeTrue())
				Expect(pr).NotTo(BeNil())

				fc, err = controller.Unpeer(fc, cluster.GetClient())
				Expect(err).To(BeNil())
				Expect(fc).NotTo(BeNil())

				Expect(fc.Status.Outgoing).To(c.expectedOutgoing)
				Expect(fc.Status.Incoming).To(c.expectedIncoming)

				obj, err = controller.crdClient.Resource("peeringrequests").List(&metav1.ListOptions{})
				Expect(err).To(BeNil())
				Expect(obj).NotTo(BeNil())

				prs, ok := obj.(*discoveryv1alpha1.PeeringRequestList)
				Expect(ok).To(BeTrue())
				Expect(prs).NotTo(BeNil())

				Expect(len(prs.Items)).To(c.expectedPeeringLength)
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
						Outgoing: discoveryv1alpha1.Outgoing{
							PeeringPhase:             discoveryv1alpha1.PeeringPhaseEstablished,
							RemotePeeringRequestName: "local-cluster",
						},
						Incoming: discoveryv1alpha1.Incoming{},
					},
				},
				pr: discoveryv1alpha1.PeeringRequest{
					ObjectMeta: metav1.ObjectMeta{
						Name: "local-cluster",
					},
					Spec: discoveryv1alpha1.PeeringRequestSpec{
						ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
							ClusterID:   "local-cluster",
							ClusterName: "Name",
						},
						Namespace: "default",
						AuthURL:   "",
					},
				},
				expectedPeeringLength: Equal(0),
				expectedOutgoing: Equal(discoveryv1alpha1.Outgoing{
					PeeringPhase: discoveryv1alpha1.PeeringPhaseNone,
				}),
				expectedIncoming: Equal(discoveryv1alpha1.Incoming{
					PeeringPhase: discoveryv1alpha1.PeeringPhaseNone,
				}),
			}),
		)

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
				// enable the new authentication flow
				controller.useNewAuth = true

				// set the local namespace in the foreign cluster, we will only need the local one during the test
				c.fc.Status.TenantControlNamespace.Local = tenantNamespace.Name

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
				Expect(fc.Status.Outgoing).To(c.expectedOutgoing)
				Expect(fc.Status.Incoming).To(c.expectedIncoming)

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
						TenantControlNamespace: discoveryv1alpha1.TenantControlNamespace{
							Local: "default",
						},
					},
				},
				expectedPeeringLength: Equal(1),
				expectedOutgoing: Equal(discoveryv1alpha1.Outgoing{
					PeeringPhase: discoveryv1alpha1.PeeringPhasePending, // we expect a joined flag set to true for the outgoing peering
				}),
				expectedIncoming: Equal(discoveryv1alpha1.Incoming{
					PeeringPhase: discoveryv1alpha1.PeeringPhaseNone,
				}),
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
				// enable the new authentication flow
				controller.useNewAuth = true

				// set the local namespace in the foreign cluster, we will only need the local one during the test
				c.fc.Status.TenantControlNamespace.Local = tenantNamespace.Name

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
				Expect(fc.Status.Outgoing).To(c.expectedOutgoing)
				Expect(fc.Status.Incoming).To(c.expectedIncoming)

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
						Outgoing: discoveryv1alpha1.Outgoing{
							PeeringPhase: discoveryv1alpha1.PeeringPhaseEstablished,
						},
						Incoming:               discoveryv1alpha1.Incoming{},
						TenantControlNamespace: discoveryv1alpha1.TenantControlNamespace{},
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
				expectedOutgoing: Equal(discoveryv1alpha1.Outgoing{
					PeeringPhase: discoveryv1alpha1.PeeringPhaseDisconnecting,
				}),
				expectedIncoming: Equal(discoveryv1alpha1.Incoming{
					PeeringPhase: discoveryv1alpha1.PeeringPhaseNone,
				}),
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
			Expect(foreignCluster.Status.TenantControlNamespace.Local).ToNot(Equal(""))

			ns, err := controller.namespaceManager.GetNamespace(foreignCluster.Spec.ClusterIdentity.ClusterID)
			Expect(err).To(BeNil())
			Expect(ns).NotTo(BeNil())

			var namespace v1.Namespace
			err = client.Get(ctx, machtypes.NamespacedName{Name: foreignCluster.Status.TenantControlNamespace.Local}, &namespace)
			Expect(err).To(BeNil())

			Expect(namespace.Name).To(Equal(ns.Name))
		})

		type checkPeeringStatusTestcase struct {
			foreignClusterStatus  discoveryv1alpha1.ForeignClusterStatus
			resourceRequests      []discoveryv1alpha1.ResourceRequest
			expectedIncomingPhase discoveryv1alpha1.PeeringPhaseType
			expectedOutgoingPhase discoveryv1alpha1.PeeringPhaseType
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

				Expect(foreignCluster.Status.Incoming.PeeringPhase).To(Equal(c.expectedIncomingPhase))
				Expect(foreignCluster.Status.Outgoing.PeeringPhase).To(Equal(c.expectedOutgoingPhase))
			},

			Entry("none", checkPeeringStatusTestcase{
				foreignClusterStatus: discoveryv1alpha1.ForeignClusterStatus{
					TenantControlNamespace: discoveryv1alpha1.TenantControlNamespace{
						Local: "default",
					},
					Incoming: discoveryv1alpha1.Incoming{
						PeeringPhase: discoveryv1alpha1.PeeringPhaseEstablished,
					},
					Outgoing: discoveryv1alpha1.Outgoing{
						PeeringPhase: discoveryv1alpha1.PeeringPhaseDisconnecting,
					},
				},
				resourceRequests:      []discoveryv1alpha1.ResourceRequest{},
				expectedIncomingPhase: discoveryv1alpha1.PeeringPhaseNone,
				expectedOutgoingPhase: discoveryv1alpha1.PeeringPhaseNone,
			}),

			Entry("none and no update", checkPeeringStatusTestcase{
				foreignClusterStatus: discoveryv1alpha1.ForeignClusterStatus{
					TenantControlNamespace: discoveryv1alpha1.TenantControlNamespace{
						Local: "default",
					},
					Incoming: discoveryv1alpha1.Incoming{
						PeeringPhase: discoveryv1alpha1.PeeringPhaseNone,
					},
					Outgoing: discoveryv1alpha1.Outgoing{
						PeeringPhase: discoveryv1alpha1.PeeringPhaseNone,
					},
				},
				resourceRequests:      []discoveryv1alpha1.ResourceRequest{},
				expectedIncomingPhase: discoveryv1alpha1.PeeringPhaseNone,
				expectedOutgoingPhase: discoveryv1alpha1.PeeringPhaseNone,
			}),

			Entry("outgoing", checkPeeringStatusTestcase{
				foreignClusterStatus: discoveryv1alpha1.ForeignClusterStatus{
					TenantControlNamespace: discoveryv1alpha1.TenantControlNamespace{
						Local: "default",
					},
					Incoming: discoveryv1alpha1.Incoming{
						PeeringPhase: discoveryv1alpha1.PeeringPhaseNone,
					},
					Outgoing: discoveryv1alpha1.Outgoing{
						PeeringPhase: discoveryv1alpha1.PeeringPhasePending,
					},
				},
				resourceRequests: []discoveryv1alpha1.ResourceRequest{
					getOutgoingResourceRequest(),
				},
				expectedIncomingPhase: discoveryv1alpha1.PeeringPhaseNone,
				expectedOutgoingPhase: discoveryv1alpha1.PeeringPhaseEstablished,
			}),

			Entry("incoming", checkPeeringStatusTestcase{
				foreignClusterStatus: discoveryv1alpha1.ForeignClusterStatus{
					TenantControlNamespace: discoveryv1alpha1.TenantControlNamespace{
						Local: "default",
					},
					Incoming: discoveryv1alpha1.Incoming{
						PeeringPhase: discoveryv1alpha1.PeeringPhasePending,
					},
					Outgoing: discoveryv1alpha1.Outgoing{
						PeeringPhase: discoveryv1alpha1.PeeringPhaseNone,
					},
				},
				resourceRequests: []discoveryv1alpha1.ResourceRequest{
					getIncomingResourceRequest(),
				},
				expectedIncomingPhase: discoveryv1alpha1.PeeringPhaseEstablished,
				expectedOutgoingPhase: discoveryv1alpha1.PeeringPhaseNone,
			}),

			Entry("bidirectional", checkPeeringStatusTestcase{
				foreignClusterStatus: discoveryv1alpha1.ForeignClusterStatus{
					TenantControlNamespace: discoveryv1alpha1.TenantControlNamespace{
						Local: "default",
					},
					Incoming: discoveryv1alpha1.Incoming{
						PeeringPhase: discoveryv1alpha1.PeeringPhasePending,
					},
					Outgoing: discoveryv1alpha1.Outgoing{
						PeeringPhase: discoveryv1alpha1.PeeringPhasePending,
					},
				},
				resourceRequests: []discoveryv1alpha1.ResourceRequest{
					getIncomingResourceRequest(),
					getOutgoingResourceRequest(),
				},
				expectedIncomingPhase: discoveryv1alpha1.PeeringPhaseEstablished,
				expectedOutgoingPhase: discoveryv1alpha1.PeeringPhaseEstablished,
			}),
		)

	})

})
