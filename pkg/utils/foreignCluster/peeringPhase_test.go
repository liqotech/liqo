package foreigncluster

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
)

func TestForeignClusterUtils(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ForeignClusterUtils")
}

var _ = Describe("PeeringPhase", func() {

	Context("getPeeringPhase", func() {
		type getPeeringPhaseTestcase struct {
			foreignCluster *discoveryv1alpha1.ForeignCluster
			expectedPhase  consts.PeeringPhase
		}

		DescribeTable("getPeeringPhase table",
			func(c getPeeringPhaseTestcase) {
				phase := GetPeeringPhase(c.foreignCluster)
				Expect(phase).To(Equal(c.expectedPhase))
			},

			Entry("bidirectional", getPeeringPhaseTestcase{
				foreignCluster: &discoveryv1alpha1.ForeignCluster{
					Status: discoveryv1alpha1.ForeignClusterStatus{
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
				expectedPhase: consts.PeeringPhaseBidirectional,
			}),

			Entry("incoming", getPeeringPhaseTestcase{
				foreignCluster: &discoveryv1alpha1.ForeignCluster{
					Status: discoveryv1alpha1.ForeignClusterStatus{
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
				expectedPhase: consts.PeeringPhaseIncoming,
			}),

			Entry("outgoing", getPeeringPhaseTestcase{
				foreignCluster: &discoveryv1alpha1.ForeignCluster{
					Status: discoveryv1alpha1.ForeignClusterStatus{
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
				expectedPhase: consts.PeeringPhaseOutgoing,
			}),

			Entry("none", getPeeringPhaseTestcase{
				foreignCluster: &discoveryv1alpha1.ForeignCluster{
					Status: discoveryv1alpha1.ForeignClusterStatus{
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
				},
				expectedPhase: consts.PeeringPhaseNone,
			}),
		)
	})

})
