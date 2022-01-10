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

package remove

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	machinerytypes "k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
)

const (
	foreignClusterName        = "foreign-cluster-1"
	invalidForeignClusterName = "foreign-cluster-2"

	timeout  = time.Second * 30
	interval = time.Millisecond * 250
)

var (
	k8sClient client.Client
)

func TestRemoveCommand(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Remove Command")
}

var _ = BeforeSuite(func() {
	utilruntime.Must(discoveryv1alpha1.AddToScheme(scheme.Scheme))
})

var _ = Describe("Test Remove Command", func() {

	BeforeEach(func() {
		k8sClient = setUpEnvironment()
	})

	When("Disable the outgoing peering fore a ForeignCluster", func() {

		type removeTestcase struct {
			clusterName   string
			expectedError types.GomegaMatcher
			expectedFlag  types.GomegaMatcher
		}

		DescribeTable("Credential Validator table",
			func(c removeTestcase) {
				ctx := context.TODO()

				err := processRemoveCluster(ctx, &ClusterArgs{
					ClusterName: c.clusterName,
				}, k8sClient)
				Expect(err).To(c.expectedError)

				Eventually(func() discoveryv1alpha1.PeeringEnabledType {
					var foreignCluster discoveryv1alpha1.ForeignCluster
					Expect(k8sClient.Get(ctx, machinerytypes.NamespacedName{Name: foreignClusterName}, &foreignCluster)).To(Succeed())

					return foreignCluster.Spec.OutgoingPeeringEnabled
				}, timeout, interval).Should(c.expectedFlag)
			},

			Entry("valid foreign cluster name", removeTestcase{
				clusterName:   foreignClusterName,
				expectedError: Not(HaveOccurred()),
				expectedFlag:  Equal(discoveryv1alpha1.PeeringEnabledNo),
			}),

			Entry("invalid foreign cluster name", removeTestcase{
				clusterName:   invalidForeignClusterName,
				expectedError: HaveOccurred(),
				expectedFlag:  Equal(discoveryv1alpha1.PeeringEnabledYes),
			}),
		)

	})

})

func setUpEnvironment() client.Client {
	foreignCluster := &discoveryv1alpha1.ForeignCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: foreignClusterName,
		},
		Spec: discoveryv1alpha1.ForeignClusterSpec{
			OutgoingPeeringEnabled: discoveryv1alpha1.PeeringEnabledYes,
		},
	}

	k8sClient := ctrlfake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(foreignCluster).Build()
	return k8sClient
}
