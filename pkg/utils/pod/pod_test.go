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

package pod_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/pointer"

	"github.com/liqotech/liqo/pkg/utils/pod"
)

var _ = Describe("Pod utility functions", func() {

	Describe("The IsPodReady function", func() {
		type IsPodReadyCase struct {
			Pod      *corev1.Pod
			Expected bool
		}

		PodGenerator := func(status corev1.ConditionStatus) *corev1.Pod {
			return &corev1.Pod{
				Status: corev1.PodStatus{Conditions: []corev1.PodCondition{
					{Type: "foo", Status: corev1.ConditionFalse},
					{Type: "bar", Status: corev1.ConditionTrue},
					{Type: corev1.PodReady, Status: status},
				}},
			}
		}

		PodGeneratorWithoutConditions := func() *corev1.Pod {
			return &corev1.Pod{}
		}

		DescribeTable("Should return the correct output",
			func(c IsPodReadyCase) {
				ready, _ := pod.IsPodReady(c.Pod)
				Expect(ready).To(BeIdenticalTo(c.Expected))
			},
			Entry("When the pod is ready", IsPodReadyCase{Pod: PodGenerator(corev1.ConditionTrue), Expected: true}),
			Entry("When the pod is not ready", IsPodReadyCase{Pod: PodGenerator(corev1.ConditionFalse), Expected: false}),
			Entry("When the pod has no conditions", IsPodReadyCase{Pod: PodGeneratorWithoutConditions(), Expected: false}),
		)
	})

	Describe("The IsPodSpecEqual function", func() {
		type TestCase struct {
			previous corev1.PodSpec
			updated  corev1.PodSpec
			expected types.GomegaMatcher
		}

		DescribeTable("tests table",
			func(c TestCase) {
				Expect(pod.IsPodSpecEqual(&c.previous, &c.updated)).To(c.expected)
			},
			Entry("both specs are empty", TestCase{expected: BeTrue()}),
			Entry("all fields are equal", TestCase{
				previous: corev1.PodSpec{
					Containers:            []corev1.Container{{Name: "foo", Image: "bar"}, {Name: "bar", Image: "baz"}},
					InitContainers:        []corev1.Container{{Name: "initfoo", Image: "initbar"}, {Name: "initbar", Image: "initbaz"}},
					Tolerations:           []corev1.Toleration{{Key: "foo"}},
					ActiveDeadlineSeconds: pointer.Int64(5),
				},
				updated: corev1.PodSpec{
					Containers:            []corev1.Container{{Name: "foo", Image: "bar"}, {Name: "bar", Image: "baz"}},
					InitContainers:        []corev1.Container{{Name: "initfoo", Image: "initbar"}, {Name: "initbar", Image: "initbaz"}},
					Tolerations:           []corev1.Toleration{{Key: "foo"}},
					ActiveDeadlineSeconds: pointer.Int64(5),
				},
				expected: BeTrue(),
			}),
			Entry("containers are different", TestCase{
				previous: corev1.PodSpec{Containers: []corev1.Container{{Name: "foo", Image: "bar"}, {Name: "bar", Image: "baz"}}},
				updated:  corev1.PodSpec{Containers: []corev1.Container{{Name: "foo", Image: "bar"}, {Name: "bar", Image: "dif"}}},
				expected: BeFalse(),
			}),
			Entry("init containers are different", TestCase{
				previous: corev1.PodSpec{InitContainers: []corev1.Container{{Name: "foo", Image: "bar"}, {Name: "bar", Image: "baz"}}},
				updated:  corev1.PodSpec{InitContainers: []corev1.Container{{Name: "foo", Image: "bar"}, {Name: "bar", Image: "dif"}}},
				expected: BeFalse(),
			}),
			Entry("more tolerations are present", TestCase{
				previous: corev1.PodSpec{Tolerations: []corev1.Toleration{{Key: "foo"}}},
				updated:  corev1.PodSpec{Tolerations: []corev1.Toleration{{Key: "foo"}, {Key: "bar"}}},
				expected: BeFalse(),
			}),
			Entry("active deadline seconds are different", TestCase{
				previous: corev1.PodSpec{ActiveDeadlineSeconds: pointer.Int64(5)},
				updated:  corev1.PodSpec{ActiveDeadlineSeconds: pointer.Int64(8)},
				expected: BeFalse(),
			}),
			Entry("active deadline seconds are different (one nil)", TestCase{
				previous: corev1.PodSpec{ActiveDeadlineSeconds: pointer.Int64(5)},
				updated:  corev1.PodSpec{ActiveDeadlineSeconds: nil},
				expected: BeFalse(),
			}),
		)
	})

	Describe("The AreContainersReady function", func() {
		type TestCase struct {
			previous []corev1.Container
			updated  []corev1.Container
			expected types.GomegaMatcher
		}

		DescribeTable("tests table",
			func(c TestCase) {
				Expect(pod.AreContainersEqual(c.previous, c.updated)).To(c.expected)
			},
			Entry("both lists are nil", TestCase{expected: BeTrue()}),
			Entry("both lists are empty, but only one is nil", TestCase{updated: []corev1.Container{}, expected: BeTrue()}),
			Entry("the two lists have the same elements in the same order", TestCase{
				previous: []corev1.Container{{Name: "foo", Image: "bar"}, {Name: "bar", Image: "baz"}},
				updated:  []corev1.Container{{Name: "foo", Image: "bar"}, {Name: "bar", Image: "baz"}},
				expected: BeTrue(),
			}),
			Entry("the two lists have the same elements in a different order", TestCase{
				previous: []corev1.Container{{Name: "foo", Image: "bar"}, {Name: "bar", Image: "baz"}},
				updated:  []corev1.Container{{Name: "bar", Image: "baz"}, {Name: "foo", Image: "bar"}},
				expected: BeTrue(),
			}),
			Entry("the two lists have different elements", TestCase{
				previous: []corev1.Container{{Name: "foo", Image: "bar"}, {Name: "bar", Image: "baz"}},
				updated:  []corev1.Container{{Name: "bar", Image: "baz"}, {Name: "foo", Image: "dif"}},
				expected: BeFalse(),
			}),
			Entry("the two lists have different lengths", TestCase{
				previous: []corev1.Container{{Name: "foo", Image: "bar"}, {Name: "bar", Image: "baz"}},
				updated:  []corev1.Container{{Name: "bar", Image: "baz"}},
				expected: BeFalse(),
			}),
		)
	})

	Describe("The ForgeContainerResources function", func() {
		DescribeTable("tests table",
			func(resources corev1.ResourceRequirements) {
				Expect(pod.ForgeContainerResources(
					resources.Requests[corev1.ResourceCPU],
					resources.Limits[corev1.ResourceCPU],
					resources.Requests[corev1.ResourceMemory],
					resources.Limits[corev1.ResourceMemory],
				)).To(Equal(resources))
			},
			Entry("no resources are set", corev1.ResourceRequirements{
				Requests: corev1.ResourceList{},
				Limits:   corev1.ResourceList{},
			}),
			Entry("only some resources are set", corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
				Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("200Mi")},
			}),
			Entry("all resources are set", corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m"), corev1.ResourceMemory: resource.MustParse("100Mi")},
				Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("200m"), corev1.ResourceMemory: resource.MustParse("200Mi")},
			}),
		)
	})
})
