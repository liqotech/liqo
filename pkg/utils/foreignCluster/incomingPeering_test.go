package foreigncluster

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
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

		var enabledClusterConfig = func() *configv1alpha1.ClusterConfig {
			return &configv1alpha1.ClusterConfig{
				Spec: configv1alpha1.ClusterConfigSpec{
					DiscoveryConfig: configv1alpha1.DiscoveryConfig{
						IncomingPeeringEnabled: true,
					},
				},
			}
		}
		var disabledClusterConfig = func() *configv1alpha1.ClusterConfig {
			return &configv1alpha1.ClusterConfig{
				Spec: configv1alpha1.ClusterConfigSpec{
					DiscoveryConfig: configv1alpha1.DiscoveryConfig{
						IncomingPeeringEnabled: false,
					},
				},
			}
		}

		type allowIncomingPeeringTestcase struct {
			foreignCluster *discoveryv1alpha1.ForeignCluster
			clusterConfig  *configv1alpha1.ClusterConfig
			expectedResult OmegaMatcher
		}

		DescribeTable("AllowIncomingPeering table",
			func(c allowIncomingPeeringTestcase) {
				phase := AllowIncomingPeering(c.foreignCluster, c.clusterConfig)
				Expect(phase).To(c.expectedResult)
			},

			Entry("incoming peering enabled and default enabled", allowIncomingPeeringTestcase{
				foreignCluster: enabledForeignCluster(),
				clusterConfig:  enabledClusterConfig(),
				expectedResult: BeTrue(),
			}),

			Entry("incoming peering enabled and default disabled", allowIncomingPeeringTestcase{
				foreignCluster: enabledForeignCluster(),
				clusterConfig:  disabledClusterConfig(),
				expectedResult: BeTrue(),
			}),

			Entry("incoming peering disabled and default enabled", allowIncomingPeeringTestcase{
				foreignCluster: disabledForeignCluster(),
				clusterConfig:  enabledClusterConfig(),
				expectedResult: BeFalse(),
			}),

			Entry("incoming peering disabled and default disabled", allowIncomingPeeringTestcase{
				foreignCluster: disabledForeignCluster(),
				clusterConfig:  disabledClusterConfig(),
				expectedResult: BeFalse(),
			}),

			Entry("incoming peering automatic and default enabled", allowIncomingPeeringTestcase{
				foreignCluster: autoForeignCluster(),
				clusterConfig:  enabledClusterConfig(),
				expectedResult: BeTrue(),
			}),

			Entry("incoming peering automatic and default disabled", allowIncomingPeeringTestcase{
				foreignCluster: autoForeignCluster(),
				clusterConfig:  disabledClusterConfig(),
				expectedResult: BeFalse(),
			}),
		)
	})

})
