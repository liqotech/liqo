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

package resourcerequestoperator

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

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

				phase, err := controller.getResourceRequestPhase(foreignCluster, c.resourceRequest)
				Expect(err).ToNot(HaveOccurred())
				Expect(phase).To(c.expectedResult)
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
