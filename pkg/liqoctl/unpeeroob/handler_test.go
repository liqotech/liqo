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

package unpeeroob

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	gtype "github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrlfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
)

const (
	foreignClusterName        = "foreign-cluster-1"
	invalidForeignClusterName = "foreign-cluster-2"
)

var _ = Describe("Test Unpeer Command", func() {
	var (
		ctx     context.Context
		options *Options
	)

	BeforeEach(func() {
		ctx = context.Background()
		fc := &discoveryv1alpha1.ForeignCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: foreignClusterName,
			},
			Spec: discoveryv1alpha1.ForeignClusterSpec{
				OutgoingPeeringEnabled: discoveryv1alpha1.PeeringEnabledYes,
			},
		}

		options = &Options{Factory: &factory.Factory{
			CRClient: ctrlfake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(fc).Build(),
		}}
	})

	When("disabling the outgoing peering for a ForeignCluster", func() {
		type removeTestcase struct {
			clusterName   string
			expectedError gtype.GomegaMatcher
			expectedFlag  discoveryv1alpha1.PeeringEnabledType
		}

		DescribeTable("unpeering table",
			func(c removeTestcase) {
				options.ClusterName = c.clusterName
				_, err := options.unpeer(ctx)
				Expect(err).To(c.expectedError)

				var fc discoveryv1alpha1.ForeignCluster
				Expect(options.CRClient.Get(ctx, types.NamespacedName{Name: foreignClusterName}, &fc)).To(Succeed())
				Expect(fc.Spec.OutgoingPeeringEnabled).To(Equal(c.expectedFlag))
			},

			Entry("valid foreign cluster name", removeTestcase{
				clusterName:   foreignClusterName,
				expectedError: Not(HaveOccurred()),
				expectedFlag:  discoveryv1alpha1.PeeringEnabledNo,
			}),

			Entry("invalid foreign cluster name", removeTestcase{
				clusterName:   invalidForeignClusterName,
				expectedError: HaveOccurred(),
				expectedFlag:  discoveryv1alpha1.PeeringEnabledYes,
			}),
		)

	})
})
