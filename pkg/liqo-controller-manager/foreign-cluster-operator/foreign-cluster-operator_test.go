// Copyright 2019-2024 The Liqo Authors
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
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
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

var _ = BeforeSuite(func() {
	testutil.LogsToGinkgoWriter()
})

var _ = Describe("ForeignClusterOperator", func() {

	var (
		cluster         testutil.Cluster
		controller      ForeignClusterReconciler
		tenantNamespace *v1.Namespace
		mgr             manager.Manager
		ctx             context.Context
		cancel          context.CancelFunc
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

		authSvc := &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: "liqo-auth",
				Annotations: map[string]string{
					consts.OverrideAddressAnnotation: "auth.liqo.io",
					consts.OverridePortAnnotation:    "443",
				},
			},
			Spec: v1.ServiceSpec{
				Type: v1.ServiceTypeLoadBalancer,
				Ports: []v1.ServicePort{
					{
						Name:       "https",
						Port:       443,
						Protocol:   v1.ProtocolTCP,
						NodePort:   30000,
						TargetPort: intstr.FromInt(8443),
					},
				},
			},
			Status: v1.ServiceStatus{
				LoadBalancer: v1.LoadBalancerStatus{
					Ingress: []v1.LoadBalancerIngress{
						{
							IP: "1.2.3.4",
						},
					},
				},
			},
		}
		authSvcStatus := authSvc.Status.DeepCopy()
		authSvc, err = cluster.GetClient().CoreV1().Services("default").Create(ctx, authSvc, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(authSvc).ToNot(BeNil())

		authSvc.Status = *authSvcStatus
		authSvc, err = cluster.GetClient().CoreV1().Services("default").UpdateStatus(ctx, authSvc, metav1.UpdateOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(authSvc).ToNot(BeNil())

		namespaceManager := tenantnamespace.NewManager(cluster.GetClient())
		identityManagerCtrl := identitymanager.NewCertificateIdentityManager(ctx, mgr.GetClient(), cluster.GetClient(), mgr.GetConfig(),
			homeCluster, namespaceManager)

		foreignCluster := discoveryv1alpha1.ClusterIdentity{
			ClusterID:   "foreign-cluster-id",
			ClusterName: "foreign-cluster-name",
		}
		tenantNamespace, err = namespaceManager.CreateNamespace(ctx, foreignCluster)
		if err != nil {
			By(err.Error())
			os.Exit(1)
		}
		// Make sure the namespace has been cached for subsequent retrieval.
		Eventually(func() (*v1.Namespace, error) { return namespaceManager.GetNamespace(ctx, foreignCluster) }).Should(Equal(tenantNamespace))

		controller = ForeignClusterReconciler{
			Client:           mgr.GetClient(),
			Scheme:           mgr.GetScheme(),
			HomeCluster:      homeCluster,
			ResyncPeriod:     300,
			NamespaceManager: namespaceManager,
			IdentityManager:  identityManagerCtrl,
			LiqoNamespace:    "default",
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
				_, err := controller.NamespaceManager.BindClusterRoles(ctx, c.fc.Spec.ClusterIdentity, &clusterRole1, &clusterRole2)
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
						Name: "foreign-cluster-name",
						Labels: map[string]string{
							discovery.ClusterIDLabel: "foreign-cluster-id",
						},
					},
					Spec: discoveryv1alpha1.ForeignClusterSpec{
						ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
							ClusterID:   "foreign-cluster-id",
							ClusterName: "foreign-cluster-name",
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
						Name: "foreign-cluster-name",
						Labels: map[string]string{
							discovery.ClusterIDLabel: "foreign-cluster-id",
						},
					},
					Spec: discoveryv1alpha1.ForeignClusterSpec{
						ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
							ClusterID:   "foreign-cluster-id",
							ClusterName: "foreign-cluster-name",
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
				expectedOutgoingClusterWide: Not(HaveOccurred()),
			}),

			Entry("outgoing peering", permissionTestcase{
				fc: discoveryv1alpha1.ForeignCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foreign-cluster-name",
						Labels: map[string]string{
							discovery.ClusterIDLabel: "foreign-cluster-id",
						},
					},
					Spec: discoveryv1alpha1.ForeignClusterSpec{
						ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
							ClusterID:   "foreign-cluster-id",
							ClusterName: "foreign-cluster-name",
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
				expectedOutgoingClusterWide: HaveOccurred(),
			}),

			Entry("bidirectional peering", permissionTestcase{
				fc: discoveryv1alpha1.ForeignCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foreign-cluster-name",
						Labels: map[string]string{
							discovery.ClusterIDLabel: "foreign-cluster-id",
						},
					},
					Spec: discoveryv1alpha1.ForeignClusterSpec{
						ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
							ClusterID:   "foreign-cluster-id",
							ClusterName: "foreign-cluster-name",
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
						discovery.ClusterIDLabel: "cluster-1",
					},
				},
				Spec: discoveryv1alpha1.ForeignClusterSpec{
					OutgoingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
					IncomingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
					InsecureSkipTLSVerify:  pointer.BoolPtr(true),
					ForeignAuthURL:         "https://example.com",
					ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
						ClusterID:   "cluster-1",
						ClusterName: "testcluster",
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
						discovery.ClusterIDLabel: controller.HomeCluster.ClusterID,
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

		It("add a cluster with invalid proxy URL", func() {

			fc := &discoveryv1alpha1.ForeignCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-1",
				},
				Spec: discoveryv1alpha1.ForeignClusterSpec{
					ForeignProxyURL: "https://example\n.Invalid",
					ClusterIdentity: controller.HomeCluster,
				},
			}

			processable, err := controller.isClusterProcessable(ctx, fc)
			Expect(err).ToNot(HaveOccurred())
			Expect(processable).To(BeFalse())
			Expect(peeringconditionsutils.GetStatus(fc, discoveryv1alpha1.ProcessForeignClusterStatusCondition)).
				To(Equal(discoveryv1alpha1.PeeringConditionStatusError))
		})

	})

})
