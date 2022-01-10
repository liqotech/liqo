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

		ForgeForeignCluster := func(auth, incoming, outgoing discoveryv1alpha1.PeeringConditionStatusType) *discoveryv1alpha1.ForeignCluster {
			return &discoveryv1alpha1.ForeignCluster{
				Status: discoveryv1alpha1.ForeignClusterStatus{
					PeeringConditions: []discoveryv1alpha1.PeeringCondition{
						{
							Type:               discoveryv1alpha1.AuthenticationStatusCondition,
							Status:             auth,
							LastTransitionTime: metav1.Now(),
						},
						{
							Type:               discoveryv1alpha1.IncomingPeeringCondition,
							Status:             incoming,
							LastTransitionTime: metav1.Now(),
						},
						{
							Type:               discoveryv1alpha1.OutgoingPeeringCondition,
							Status:             outgoing,
							LastTransitionTime: metav1.Now(),
						},
					},
				},
			}
		}

		DescribeTable("GetPeeringPhase table",
			func(c getPeeringPhaseTestcase) {
				phase := GetPeeringPhase(c.foreignCluster)
				Expect(phase).To(Equal(c.expectedPhase))
			},

			Entry("bidirectional (established)", getPeeringPhaseTestcase{
				foreignCluster: ForgeForeignCluster(
					discoveryv1alpha1.PeeringConditionStatusEstablished,
					discoveryv1alpha1.PeeringConditionStatusEstablished,
					discoveryv1alpha1.PeeringConditionStatusEstablished),
				expectedPhase: consts.PeeringPhaseBidirectional,
			}),

			Entry("bidirectional (disconnecting)", getPeeringPhaseTestcase{
				foreignCluster: ForgeForeignCluster(
					discoveryv1alpha1.PeeringConditionStatusEstablished,
					discoveryv1alpha1.PeeringConditionStatusDisconnecting,
					discoveryv1alpha1.PeeringConditionStatusDisconnecting),
				expectedPhase: consts.PeeringPhaseBidirectional,
			}),

			Entry("incoming", getPeeringPhaseTestcase{
				foreignCluster: ForgeForeignCluster(
					discoveryv1alpha1.PeeringConditionStatusEstablished,
					discoveryv1alpha1.PeeringConditionStatusPending,
					discoveryv1alpha1.PeeringConditionStatusNone),
				expectedPhase: consts.PeeringPhaseIncoming,
			}),

			Entry("outgoing", getPeeringPhaseTestcase{
				foreignCluster: ForgeForeignCluster(
					discoveryv1alpha1.PeeringConditionStatusEstablished,
					discoveryv1alpha1.PeeringConditionStatusNone,
					discoveryv1alpha1.PeeringConditionStatusEstablished),
				expectedPhase: consts.PeeringPhaseOutgoing,
			}),

			Entry("authenticated", getPeeringPhaseTestcase{
				foreignCluster: ForgeForeignCluster(
					discoveryv1alpha1.PeeringConditionStatusEstablished,
					discoveryv1alpha1.PeeringConditionStatusNone,
					discoveryv1alpha1.PeeringConditionStatusNone),
				expectedPhase: consts.PeeringPhaseAuthenticated,
			}),

			Entry("none", getPeeringPhaseTestcase{
				foreignCluster: ForgeForeignCluster(
					discoveryv1alpha1.PeeringConditionStatusNone,
					discoveryv1alpha1.PeeringConditionStatusNone,
					discoveryv1alpha1.PeeringConditionStatusNone),
				expectedPhase: consts.PeeringPhaseNone,
			}),
		)
	})
})
