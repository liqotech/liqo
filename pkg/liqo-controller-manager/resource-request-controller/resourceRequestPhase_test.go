package resourcerequestoperator

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/discovery"
)

var _ = Describe("Resource Phase", func() {

	Context("getResourceRequestPhase func", func() {

		var (
			controller *ResourceRequestReconciler

			clusterID = "fc-1"
		)

		BeforeEach(func() {
			controller = &ResourceRequestReconciler{}
			controller.Broadcaster = &Broadcaster{
				clusterConfig: configv1alpha1.ClusterConfig{
					Spec: configv1alpha1.ClusterConfigSpec{
						DiscoveryConfig: configv1alpha1.DiscoveryConfig{
							IncomingPeeringEnabled: true,
						},
					},
				},
			}
			controller.Client = k8sClient
		})

		type getResourceRequestPhaseTestcase struct {
			incomingPeeringEnabled discoveryv1alpha1.PeeringEnabledType
			resourceRequest        *discoveryv1alpha1.ResourceRequest
			expectedResult         OmegaMatcher
		}

		DescribeTable("getResourceRequestPhase table",
			func(c getResourceRequestPhaseTestcase) {
				foreignCluster := &discoveryv1alpha1.ForeignCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: clusterID,
						Labels: map[string]string{
							discovery.ClusterIDLabel: clusterID,
						},
					},
					Spec: discoveryv1alpha1.ForeignClusterSpec{
						ForeignAuthURL:         "https://example.com",
						OutgoingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
						IncomingPeeringEnabled: c.incomingPeeringEnabled,
						InsecureSkipTLSVerify:  pointer.BoolPtr(true),
					},
				}

				Expect(controller.Create(ctx, foreignCluster)).To(Succeed())

				// this eventually fixes a cache race condition
				Eventually(func() resourceRequestPhase {
					phase, err := controller.getResourceRequestPhase(ctx, c.resourceRequest)
					Expect(err).ToNot(HaveOccurred())
					return phase
				}, timeout, interval).Should(c.expectedResult)

				Expect(controller.Delete(ctx, foreignCluster)).To(Succeed())
			},

			Entry("deleted resource request", getResourceRequestPhaseTestcase{
				incomingPeeringEnabled: discoveryv1alpha1.PeeringEnabledYes,
				resourceRequest: &discoveryv1alpha1.ResourceRequest{
					ObjectMeta: metav1.ObjectMeta{
						DeletionTimestamp: &now,
					},
					Spec: discoveryv1alpha1.ResourceRequestSpec{
						ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
							ClusterID: clusterID,
						},
					},
				},
				expectedResult: Equal(deletingResourceRequestPhase),
			}),

			Entry("withdrawn resource request", getResourceRequestPhaseTestcase{
				incomingPeeringEnabled: discoveryv1alpha1.PeeringEnabledYes,
				resourceRequest: &discoveryv1alpha1.ResourceRequest{
					Spec: discoveryv1alpha1.ResourceRequestSpec{
						ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
							ClusterID: clusterID,
						},
						WithdrawalTimestamp: &now,
					},
				},
				expectedResult: Equal(deletingResourceRequestPhase),
			}),

			Entry("accepted resource request", getResourceRequestPhaseTestcase{
				incomingPeeringEnabled: discoveryv1alpha1.PeeringEnabledYes,
				resourceRequest: &discoveryv1alpha1.ResourceRequest{
					Spec: discoveryv1alpha1.ResourceRequestSpec{
						ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
							ClusterID: clusterID,
						},
					},
				},
				expectedResult: Equal(allowResourceRequestPhase),
			}),

			Entry("denied resource request", getResourceRequestPhaseTestcase{
				incomingPeeringEnabled: discoveryv1alpha1.PeeringEnabledNo,
				resourceRequest: &discoveryv1alpha1.ResourceRequest{
					Spec: discoveryv1alpha1.ResourceRequestSpec{
						ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
							ClusterID: clusterID,
						},
					},
				},
				expectedResult: Equal(denyResourceRequestPhase),
			}),
		)
	})

})
