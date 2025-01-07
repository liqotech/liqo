// Copyright 2019-2025 The Liqo Authors
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

package resources_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	"github.com/liqotech/liqo/pkg/client/clientset/versioned/scheme"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/resources"
)

var _ = Describe("Shared Resources utility functions", func() {

	var (
		clientBuilder                              fake.ClientBuilder
		ctx                                        context.Context
		cl                                         client.Client
		resExpected                                corev1.ResourceList
		cpuQuantityAcquired, cpuQuantityShared     *resource.Quantity
		memQuantityAcquired, memQuantityShared     *resource.Quantity
		podsQuantityAcquired, podsQuantityShared   *resource.Quantity
		otherQuantityAcquired, otherQuantityShared *resource.Quantity
	)
	const (
		clusterID = "ID1"
	)

	BeforeEach(func() {
		cpuQuantityAcquired = resource.NewScaledQuantity(1000, resource.Kilo)
		memQuantityAcquired = resource.NewQuantity(2000, resource.BinarySI)
		podsQuantityAcquired = resource.NewQuantity(1, resource.DecimalSI)
		otherQuantityAcquired = resource.NewQuantity(4000, resource.DecimalSI)
		cpuQuantityShared = resource.NewScaledQuantity(1100, resource.Kilo)
		memQuantityShared = resource.NewQuantity(2100, resource.BinarySI)
		podsQuantityShared = resource.NewQuantity(2, resource.DecimalSI)
		otherQuantityShared = resource.NewQuantity(4100, resource.DecimalSI)
		ctx = context.Background()
		clientBuilder = *fake.NewClientBuilder().WithScheme(scheme.Scheme)
		clientBuilder.WithObjects(
			&authv1beta1.ResourceSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sliceLocal",
					Labels: map[string]string{
						consts.ReplicationDestinationLabel: clusterID,
						consts.ReplicationRequestedLabel:   "true",
					},
				},
				Spec: authv1beta1.ResourceSliceSpec{},
				Status: authv1beta1.ResourceSliceStatus{
					Resources: corev1.ResourceList{
						corev1.ResourceCPU:    *cpuQuantityAcquired,
						corev1.ResourceMemory: *memQuantityAcquired,
						corev1.ResourcePods:   *podsQuantityAcquired,
						"other":               *otherQuantityAcquired,
					},
				},
			},
			&authv1beta1.ResourceSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sliceRemote",
					Labels: map[string]string{
						consts.ReplicationOriginLabel: clusterID,
						consts.ReplicationStatusLabel: "true",
					},
				},
				Spec: authv1beta1.ResourceSliceSpec{},
				Status: authv1beta1.ResourceSliceStatus{
					Resources: corev1.ResourceList{
						corev1.ResourceCPU:    *cpuQuantityShared,
						corev1.ResourceMemory: *memQuantityShared,
						corev1.ResourcePods:   *podsQuantityShared,
						"other":               *otherQuantityShared,
					},
				},
			},
		)
		cl = clientBuilder.Build()
	})

	When("Total Acquired resources are retrieved", func() {
		JustBeforeEach(func() {
			resExpected = corev1.ResourceList{
				corev1.ResourceCPU:    *cpuQuantityAcquired,
				corev1.ResourceMemory: *memQuantityAcquired,
				corev1.ResourcePods:   *podsQuantityAcquired,
				"other":               *otherQuantityAcquired,
			}
		})
		It("Should return the correct resources", func() {
			res, err := resources.GetAcquiredTotal(ctx, cl, clusterID)
			Expect(err).ToNot(HaveOccurred())
			Expect(resources.CPU(res)).To(Equal(resources.CPU(resExpected)))
			Expect(resources.Memory(res)).To(Equal(resources.Memory(resExpected)))
			Expect(resources.Pods(res)).To(Equal(resources.Pods(resExpected)))
			Expect(resources.Others(res)).To(Equal(resources.Others(resExpected)))
		})
	})
	When("Total Shared resources are retrieved", func() {
		JustBeforeEach(func() {
			resExpected = corev1.ResourceList{
				corev1.ResourceCPU:    *cpuQuantityShared,
				corev1.ResourceMemory: *memQuantityShared,
				corev1.ResourcePods:   *podsQuantityShared,
				"other":               *otherQuantityShared,
			}
		})
		It("Should return the correct resources", func() {
			res, err := resources.GetSharedTotal(ctx, cl, clusterID)
			Expect(err).ToNot(HaveOccurred())
			Expect(resources.CPU(res)).To(Equal(resources.CPU(resExpected)))
			Expect(resources.Memory(res)).To(Equal(resources.Memory(resExpected)))
			Expect(resources.Pods(res)).To(Equal(resources.Pods(resExpected)))
			Expect(resources.Others(res)).To(Equal(resources.Others(resExpected)))
		})
	})
})
