// Copyright 2019-2022 The Liqo Authors
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
