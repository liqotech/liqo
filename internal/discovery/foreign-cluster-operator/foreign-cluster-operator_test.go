package foreignclusteroperator

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/liqotech/liqo/apis/config/v1alpha1"
	v1alpha12 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
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
	)

	BeforeEach(func() {
		var err error
		cluster, _, err = testUtils.NewTestCluster([]string{filepath.Join("..", "..", "..", "deployments", "liqo", "crds")})
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
			Scheme:              scheme,
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
	})

	AfterEach(func() {
		err := cluster.GetEnv().Stop()
		if err != nil {
			By(err.Error())
			os.Exit(1)
		}
	})

	// peer

	Context("Peer", func() {

		type peerTestcase struct {
			fc                    v1alpha12.ForeignCluster
			expectedPeeringLength types.GomegaMatcher
			expectedOutgoing      types.GomegaMatcher
			expectedIncoming      types.GomegaMatcher
		}

		DescribeTable("Peer table",
			func(c peerTestcase) {
				obj, err := controller.crdClient.Resource("foreignclusters").Create(&c.fc, &metav1.CreateOptions{})
				Expect(err).To(BeNil())
				Expect(obj).NotTo(BeNil())

				fc, ok := obj.(*v1alpha12.ForeignCluster)
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

				prs, ok := obj.(*v1alpha12.PeeringRequestList)
				Expect(ok).To(BeTrue())
				Expect(prs).NotTo(BeNil())

				Expect(len(prs.Items)).To(c.expectedPeeringLength)
			},

			Entry("peer", peerTestcase{
				fc: v1alpha12.ForeignCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foreign-cluster",
						Labels: map[string]string{
							discovery.DiscoveryTypeLabel: string(discovery.ManualDiscovery),
							discovery.ClusterIDLabel:     "foreign-cluster",
						},
					},
					Spec: v1alpha12.ForeignClusterSpec{
						ClusterIdentity: v1alpha12.ClusterIdentity{
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
				expectedOutgoing: Equal(v1alpha12.Outgoing{
					Joined:                   true,
					RemotePeeringRequestName: "local-cluster",
				}),
				expectedIncoming: Equal(v1alpha12.Incoming{}),
			}),
		)

	})

	// unpeer

	Context("Unpeer", func() {

		type unpeerTestcase struct {
			fc                    v1alpha12.ForeignCluster
			pr                    v1alpha12.PeeringRequest
			expectedPeeringLength types.GomegaMatcher
			expectedOutgoing      types.GomegaMatcher
			expectedIncoming      types.GomegaMatcher
		}

		DescribeTable("Unpeer table",
			func(c unpeerTestcase) {
				obj, err := controller.crdClient.Resource("foreignclusters").Create(&c.fc, &metav1.CreateOptions{})
				Expect(err).To(BeNil())
				Expect(obj).NotTo(BeNil())

				fc, ok := obj.(*v1alpha12.ForeignCluster)
				Expect(ok).To(BeTrue())
				Expect(fc).NotTo(BeNil())

				obj, err = controller.crdClient.Resource("peeringrequests").Create(&c.pr, &metav1.CreateOptions{})
				Expect(err).To(BeNil())
				Expect(obj).NotTo(BeNil())

				pr, ok := obj.(*v1alpha12.PeeringRequest)
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

				prs, ok := obj.(*v1alpha12.PeeringRequestList)
				Expect(ok).To(BeTrue())
				Expect(prs).NotTo(BeNil())

				Expect(len(prs.Items)).To(c.expectedPeeringLength)
			},

			Entry("unpeer", unpeerTestcase{
				fc: v1alpha12.ForeignCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foreign-cluster",
						Labels: map[string]string{
							discovery.DiscoveryTypeLabel: string(discovery.ManualDiscovery),
							discovery.ClusterIDLabel:     "foreign-cluster",
						},
					},
					Spec: v1alpha12.ForeignClusterSpec{
						ClusterIdentity: v1alpha12.ClusterIdentity{
							ClusterID:   "foreign-cluster",
							ClusterName: "ClusterTest2",
						},
						Namespace:     "liqo",
						DiscoveryType: discovery.ManualDiscovery,
						AuthURL:       "",
						TrustMode:     discovery.TrustModeUntrusted,
					},
					Status: v1alpha12.ForeignClusterStatus{
						Outgoing: v1alpha12.Outgoing{
							Joined:                   true,
							RemotePeeringRequestName: "local-cluster",
						},
						Incoming: v1alpha12.Incoming{},
					},
				},
				pr: v1alpha12.PeeringRequest{
					ObjectMeta: metav1.ObjectMeta{
						Name: "local-cluster",
					},
					Spec: v1alpha12.PeeringRequestSpec{
						ClusterIdentity: v1alpha12.ClusterIdentity{
							ClusterID:   "local-cluster",
							ClusterName: "Name",
						},
						Namespace: "default",
						AuthURL:   "",
					},
				},
				expectedPeeringLength: Equal(0),
				expectedOutgoing:      Equal(v1alpha12.Outgoing{}),
				expectedIncoming:      Equal(v1alpha12.Incoming{}),
			}),
		)

	})

	// peer namespaced

	Context("Peer Namespaced", func() {

		type peerTestcase struct {
			fc                    v1alpha12.ForeignCluster
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

				fc, ok := obj.(*v1alpha12.ForeignCluster)
				Expect(ok).To(BeTrue())
				Expect(fc).NotTo(BeNil())

				// enable the peering for that foreigncluster
				fc, err = controller.Peer(fc, cluster.GetClient())
				Expect(err).To(BeNil())
				Expect(fc).NotTo(BeNil())

				// check that the incoming and the outgoing statuses are the expected ones
				Expect(fc.Status.Outgoing).To(c.expectedOutgoing)
				Expect(fc.Status.Incoming).To(c.expectedIncoming)

				// get the resource requests in the local tenant namespace
				obj, err = controller.crdClient.Resource("resourcerequests").Namespace(tenantNamespace.Name).List(&metav1.ListOptions{})
				Expect(err).To(BeNil())
				Expect(obj).NotTo(BeNil())

				rrs, ok := obj.(*v1alpha12.ResourceRequestList)
				Expect(ok).To(BeTrue())
				Expect(rrs).NotTo(BeNil())

				// check that the length of the resource request list is the expected one,
				// and the resource request has been created in the correct namespace
				Expect(len(rrs.Items)).To(c.expectedPeeringLength)
			},

			Entry("peer", peerTestcase{
				fc: v1alpha12.ForeignCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foreign-cluster",
						Labels: map[string]string{
							discovery.DiscoveryTypeLabel: string(discovery.ManualDiscovery),
							discovery.ClusterIDLabel:     "foreign-cluster",
						},
					},
					Spec: v1alpha12.ForeignClusterSpec{
						ClusterIdentity: v1alpha12.ClusterIdentity{
							ClusterID:   "foreign-cluster",
							ClusterName: "ClusterTest2",
						},
						Namespace:     "liqo",
						DiscoveryType: discovery.ManualDiscovery,
						AuthURL:       "",
						TrustMode:     discovery.TrustModeUntrusted,
					},
					Status: v1alpha12.ForeignClusterStatus{
						TenantControlNamespace: v1alpha12.TenantControlNamespace{},
					},
				},
				expectedPeeringLength: Equal(1),
				expectedOutgoing: Equal(v1alpha12.Outgoing{
					Joined: true, // we expect a joined flag set to true for the outgoing peering
				}),
				expectedIncoming: Equal(v1alpha12.Incoming{}),
			}),
		)

	})

	// unpeer namespaced

	Context("Unpeer Namespaced", func() {

		type unpeerTestcase struct {
			fc                    v1alpha12.ForeignCluster
			rr                    v1alpha12.ResourceRequest
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
				c.rr.Spec.ClusterIdentity.ClusterID = controller.clusterID.GetClusterID()

				// create the foreigncluster CR
				obj, err := controller.crdClient.Resource("foreignclusters").Create(&c.fc, &metav1.CreateOptions{})
				Expect(err).To(BeNil())
				Expect(obj).NotTo(BeNil())

				fc, ok := obj.(*v1alpha12.ForeignCluster)
				Expect(ok).To(BeTrue())
				Expect(fc).NotTo(BeNil())

				// create the resourcerequest CR
				obj, err = controller.crdClient.Resource("resourcerequests").Namespace(tenantNamespace.Name).Create(&c.rr, &metav1.CreateOptions{})
				Expect(err).To(BeNil())
				Expect(obj).NotTo(BeNil())

				rr, ok := obj.(*v1alpha12.ResourceRequest)
				Expect(ok).To(BeTrue())
				Expect(rr).NotTo(BeNil())

				// disable the peering for that foreigncluster
				fc, err = controller.Unpeer(fc, cluster.GetClient())
				Expect(err).To(BeNil())
				Expect(fc).NotTo(BeNil())

				// check that the incoming and the outgoing statuses are the expected ones
				Expect(fc.Status.Outgoing).To(c.expectedOutgoing)
				Expect(fc.Status.Incoming).To(c.expectedIncoming)

				// get the resource requests in the local tenant namespace
				obj, err = controller.crdClient.Resource("resourcerequests").Namespace(tenantNamespace.Name).List(&metav1.ListOptions{})
				Expect(err).To(BeNil())
				Expect(obj).NotTo(BeNil())

				rrs, ok := obj.(*v1alpha12.ResourceRequestList)
				Expect(ok).To(BeTrue())
				Expect(rrs).NotTo(BeNil())

				// check that the length of the resource request list is the expected one,
				// and the resource request has been deleted in the correct namespace
				Expect(len(rrs.Items)).To(c.expectedPeeringLength)
			},

			Entry("unpeer", unpeerTestcase{
				fc: v1alpha12.ForeignCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foreign-cluster",
						Labels: map[string]string{
							discovery.DiscoveryTypeLabel: string(discovery.ManualDiscovery),
							discovery.ClusterIDLabel:     "foreign-cluster",
						},
					},
					Spec: v1alpha12.ForeignClusterSpec{
						ClusterIdentity: v1alpha12.ClusterIdentity{
							ClusterID:   "foreign-cluster",
							ClusterName: "ClusterTest2",
						},
						Namespace:     "liqo",
						DiscoveryType: discovery.ManualDiscovery,
						AuthURL:       "",
						TrustMode:     discovery.TrustModeUntrusted,
					},
					Status: v1alpha12.ForeignClusterStatus{
						Outgoing: v1alpha12.Outgoing{
							Joined: true,
						},
						Incoming:               v1alpha12.Incoming{},
						TenantControlNamespace: v1alpha12.TenantControlNamespace{},
					},
				},
				rr: v1alpha12.ResourceRequest{
					ObjectMeta: metav1.ObjectMeta{
						Name: "",
					},
					Spec: v1alpha12.ResourceRequestSpec{
						ClusterIdentity: v1alpha12.ClusterIdentity{
							ClusterID:   "",
							ClusterName: "Name",
						},
						AuthURL: "",
					},
				},
				expectedPeeringLength: Equal(0),
				expectedOutgoing:      Equal(v1alpha12.Outgoing{}),
				expectedIncoming:      Equal(v1alpha12.Incoming{}),
			}),
		)

	})

})
