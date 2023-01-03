// Copyright 2019-2023 The Liqo Authors
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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrlfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	. "github.com/liqotech/liqo/pkg/utils/testutil"
)

const foreignClusterName = "foreign-cluster"

var _ = Describe("Test Unpeer Command", func() {
	var (
		ctx     context.Context
		options *Options

		fc discoveryv1alpha1.ForeignCluster
	)

	BeforeEach(func() {
		ctx = context.Background()
		fc = discoveryv1alpha1.ForeignCluster{
			ObjectMeta: metav1.ObjectMeta{Name: foreignClusterName},
			Spec:       discoveryv1alpha1.ForeignClusterSpec{OutgoingPeeringEnabled: discoveryv1alpha1.PeeringEnabledYes},
		}
		options = &Options{Factory: &factory.Factory{}}
	})

	JustBeforeEach(func() {
		options.Factory.CRClient = ctrlfake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(&fc).Build()
	})

	Context("disabling the outgoing peering", func() {
		var (
			err error
		)

		PeeringEnabledBody := func(expected discoveryv1alpha1.PeeringEnabledType) func() {
			return func() {
				Expect(options.CRClient.Get(ctx, types.NamespacedName{Name: foreignClusterName}, &fc)).To(Succeed())
				Expect(fc.Spec.OutgoingPeeringEnabled).To(Equal(expected))
			}
		}

		JustBeforeEach(func() { _, err = options.unpeer(ctx) })

		When("the foreign cluster does not exist", func() {
			BeforeEach(func() { options.ClusterName = "invalid" })
			It("should fail with an error", func() { Expect(err).To(HaveOccurred()) })
			It("should not disable the peering", PeeringEnabledBody(discoveryv1alpha1.PeeringEnabledYes))
		})

		When("the foreign cluster does exists, and has type out-of-band", func() {
			BeforeEach(func() {
				options.ClusterName = foreignClusterName
				fc.Spec.PeeringType = discoveryv1alpha1.PeeringTypeOutOfBand
			})

			When("unpeer out-of-band mode is not set", func() {
				BeforeEach(func() { options.UnpeerOOBMode = false })
				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("should correctly disable the peering", PeeringEnabledBody(discoveryv1alpha1.PeeringEnabledNo))
			})

			When("unpeer out-of-band mode is set", func() {
				BeforeEach(func() { options.UnpeerOOBMode = true })
				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("should correctly disable the peering", PeeringEnabledBody(discoveryv1alpha1.PeeringEnabledNo))
			})
		})

		When("the foreign cluster does exists, and has type in-band", func() {
			BeforeEach(func() {
				options.ClusterName = foreignClusterName
				fc.Spec.PeeringType = discoveryv1alpha1.PeeringTypeInBand
			})

			When("unpeer out-of-band mode is not set", func() {
				BeforeEach(func() { options.UnpeerOOBMode = false })
				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("should correctly disable the peering", PeeringEnabledBody(discoveryv1alpha1.PeeringEnabledNo))
			})

			When("unpeer out-of-band mode is set", func() {
				BeforeEach(func() { options.UnpeerOOBMode = true })
				It("should fail with an error", func() { Expect(err).To(HaveOccurred()) })
				It("should not disable the peering", PeeringEnabledBody(discoveryv1alpha1.PeeringEnabledYes))
			})
		})
	})

	Context("deleting a foreign cluster", func() {
		var (
			deleted bool
			err     error
		)

		BeforeEachBody := func(status discoveryv1alpha1.PeeringConditionStatusType) func() {
			return func() {
				options.ClusterName = foreignClusterName
				fc.Status.PeeringConditions = []discoveryv1alpha1.PeeringCondition{
					{Type: discoveryv1alpha1.IncomingPeeringCondition, Status: status},
				}
			}
		}

		JustBeforeEach(func() { deleted, err = options.delete(ctx, &fc) })

		When("incoming peering is disabled", func() {
			BeforeEach(BeforeEachBody(discoveryv1alpha1.PeeringConditionStatusNone))

			It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
			It("should claim to have deleted the foreign cluster", func() { Expect(deleted).To(BeTrue()) })
			It("should correctly delete the foreign cluster", func() {
				Expect(options.CRClient.Get(ctx, types.NamespacedName{Name: options.ClusterName}, &fc)).To(BeNotFound())
			})
		})

		When("incoming peering is enabled", func() {
			BeforeEach(BeforeEachBody(discoveryv1alpha1.PeeringConditionStatusEstablished))

			It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
			It("should not claim to have deleted the foreign cluster", func() { Expect(deleted).To(BeFalse()) })
			It("should not delete the foreign cluster", func() {
				Expect(options.CRClient.Get(ctx, types.NamespacedName{Name: options.ClusterName}, &fc)).To(Succeed())
			})
		})
	})
})
