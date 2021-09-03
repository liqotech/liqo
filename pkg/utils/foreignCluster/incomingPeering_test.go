package foreigncluster

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
)

var _ = Describe("IncomingPeering", func() {

	Context("AllowIncomingPeering func", func() {

		var enabledForeignCluster = func() *discoveryv1alpha1.ForeignCluster {
			return &discoveryv1alpha1.ForeignCluster{
				Spec: discoveryv1alpha1.ForeignClusterSpec{
					IncomingPeeringEnabled: discoveryv1alpha1.PeeringEnabledYes,
				},
			}
		}
		var disabledForeignCluster = func() *discoveryv1alpha1.ForeignCluster {
			return &discoveryv1alpha1.ForeignCluster{
				Spec: discoveryv1alpha1.ForeignClusterSpec{
					IncomingPeeringEnabled: discoveryv1alpha1.PeeringEnabledNo,
				},
			}
		}
		var autoForeignCluster = func() *discoveryv1alpha1.ForeignCluster {
			return &discoveryv1alpha1.ForeignCluster{
				Spec: discoveryv1alpha1.ForeignClusterSpec{
					IncomingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
				},
			}
		}

		type allowIncomingPeeringTestcase struct {
			foreignCluster               *discoveryv1alpha1.ForeignCluster
			defaultEnableIncomingPeering bool
			expectedResult               OmegaMatcher
		}

		DescribeTable("AllowIncomingPeering table",
			func(c allowIncomingPeeringTestcase) {
				phase := AllowIncomingPeering(c.foreignCluster, c.defaultEnableIncomingPeering)
				Expect(phase).To(c.expectedResult)
			},

			Entry("incoming peering enabled and default enabled", allowIncomingPeeringTestcase{
				foreignCluster:               enabledForeignCluster(),
				defaultEnableIncomingPeering: true,
				expectedResult:               BeTrue(),
			}),

			Entry("incoming peering enabled and default disabled", allowIncomingPeeringTestcase{
				foreignCluster:               enabledForeignCluster(),
				defaultEnableIncomingPeering: false,
				expectedResult:               BeTrue(),
			}),

			Entry("incoming peering disabled and default enabled", allowIncomingPeeringTestcase{
				foreignCluster:               disabledForeignCluster(),
				defaultEnableIncomingPeering: true,
				expectedResult:               BeFalse(),
			}),

			Entry("incoming peering disabled and default disabled", allowIncomingPeeringTestcase{
				foreignCluster:               disabledForeignCluster(),
				defaultEnableIncomingPeering: false,
				expectedResult:               BeFalse(),
			}),

			Entry("incoming peering automatic and default enabled", allowIncomingPeeringTestcase{
				foreignCluster:               autoForeignCluster(),
				defaultEnableIncomingPeering: true,
				expectedResult:               BeTrue(),
			}),

			Entry("incoming peering automatic and default disabled", allowIncomingPeeringTestcase{
				foreignCluster:               autoForeignCluster(),
				defaultEnableIncomingPeering: false,
				expectedResult:               BeFalse(),
			}),
		)
	})

})
