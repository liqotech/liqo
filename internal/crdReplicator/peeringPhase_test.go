package crdreplicator

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
)

func TestDiscovery(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "PeeringPhase")
}

var _ = Describe("PeeringPhase", func() {

	Context("getPeeringPhase", func() {
		type getPeeringPhaseTestcase struct {
			foreignCluster *discoveryv1alpha1.ForeignCluster
			expectedPhase  consts.PeeringPhase
		}

		DescribeTable("getPeeringPhase table",
			func(c getPeeringPhaseTestcase) {
				phase := getPeeringPhase(c.foreignCluster)
				Expect(phase).To(Equal(c.expectedPhase))
			},

			Entry("bidirectional", getPeeringPhaseTestcase{
				foreignCluster: &discoveryv1alpha1.ForeignCluster{
					Status: discoveryv1alpha1.ForeignClusterStatus{
						Incoming: discoveryv1alpha1.Incoming{
							Joined: true,
						},
						Outgoing: discoveryv1alpha1.Outgoing{
							Joined: true,
						},
					},
				},
				expectedPhase: consts.PeeringPhaseBidirectional,
			}),

			Entry("incoming", getPeeringPhaseTestcase{
				foreignCluster: &discoveryv1alpha1.ForeignCluster{
					Status: discoveryv1alpha1.ForeignClusterStatus{
						Incoming: discoveryv1alpha1.Incoming{
							Joined: true,
						},
						Outgoing: discoveryv1alpha1.Outgoing{
							Joined: false,
						},
					},
				},
				expectedPhase: consts.PeeringPhaseIncoming,
			}),

			Entry("outgoing", getPeeringPhaseTestcase{
				foreignCluster: &discoveryv1alpha1.ForeignCluster{
					Status: discoveryv1alpha1.ForeignClusterStatus{
						Incoming: discoveryv1alpha1.Incoming{
							Joined: false,
						},
						Outgoing: discoveryv1alpha1.Outgoing{
							Joined: true,
						},
					},
				},
				expectedPhase: consts.PeeringPhaseOutgoing,
			}),

			Entry("none", getPeeringPhaseTestcase{
				foreignCluster: &discoveryv1alpha1.ForeignCluster{
					Status: discoveryv1alpha1.ForeignClusterStatus{
						Incoming: discoveryv1alpha1.Incoming{
							Joined: false,
						},
						Outgoing: discoveryv1alpha1.Outgoing{
							Joined: false,
						},
					},
				},
				expectedPhase: consts.PeeringPhaseNone,
			}),
		)
	})

})
