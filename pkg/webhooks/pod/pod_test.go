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

package pod

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils"
	"github.com/liqotech/liqo/pkg/utils/testutil"
	"github.com/liqotech/liqo/pkg/webhooks/pod/testutils"
)

func TestWebhookManager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Webhook Suite")
}

var _ = BeforeSuite(func() {
	testutil.LogsToGinkgoWriter()
})

var _ = Describe("Webhook", func() {

	var (
		virtualNodeToleration = corev1.Toleration{
			Key:      liqoconst.VirtualNodeTolerationKey,
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoExecute,
		}
	)

	Context("Check the new created toleration according to the PodOffloadingStrategy", func() {
		emptyToleration := corev1.Toleration{}
		DescribeTable("Test for each PodOffloadingStrategy",
			func(strategy offloadingv1beta1.PodOffloadingStrategyType, expectedToleration corev1.Toleration) {
				By(fmt.Sprintf("Testing %s", strategy))
				toleration, err := createTolerationFromNamespaceOffloading(strategy)
				if strategy == "" {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
				Expect(toleration.MatchToleration(&expectedToleration)).To(BeTrue())
			},
			Entry("", offloadingv1beta1.LocalPodOffloadingStrategyType, emptyToleration),
			Entry("LocalPodOffloadingStrategyType", offloadingv1beta1.LocalPodOffloadingStrategyType, emptyToleration),
			Entry("RemotePodOffloadingStrategyType", offloadingv1beta1.RemotePodOffloadingStrategyType, virtualNodeToleration),
			Entry("LocalAndRemotePodOffloadingStrategyType", offloadingv1beta1.LocalAndRemotePodOffloadingStrategyType, virtualNodeToleration),
		)
	})

	Context("Check the NodeSelector imposed by the NamespaceOffloading", func() {
		localNamespaceOffloading := testutils.GetNamespaceOffloading(offloadingv1beta1.LocalPodOffloadingStrategyType)
		remoteNamespaceOffloading := testutils.GetNamespaceOffloading(offloadingv1beta1.RemotePodOffloadingStrategyType)
		localAndRemoteNamespaceOffloading := testutils.GetNamespaceOffloading(offloadingv1beta1.LocalAndRemotePodOffloadingStrategyType)

		remoteNodeSelector := testutils.GetImposedNodeSelector(offloadingv1beta1.RemotePodOffloadingStrategyType)
		localAndRemoteNodeSelector := testutils.GetImposedNodeSelector(offloadingv1beta1.LocalAndRemotePodOffloadingStrategyType)

		DescribeTable("Test for each PodOffloadingStrategy",
			func(namespaceOffloading offloadingv1beta1.NamespaceOffloading, expectedNodeSelector *corev1.NodeSelector) {
				By(fmt.Sprintf("Testing %s", namespaceOffloading.Spec.PodOffloadingStrategy))
				nodeSelector, err := createNodeSelectorFromNamespaceOffloading(&namespaceOffloading)
				Expect(err).ToNot(HaveOccurred())
				if expectedNodeSelector == nil {
					Expect(nodeSelector).To(BeNil(), "Expected the node selectors to be nil")
				} else {
					Expect(nodeSelector).To(
						PointTo(Equal(*expectedNodeSelector)),
						"Node selectors are not the expected ones",
					)
				}
			},
			Entry("LocalPodOffloadingStrategyType", localNamespaceOffloading, nil),
			Entry("RemotePodOffloadingStrategyType", remoteNamespaceOffloading, &remoteNodeSelector),
			Entry("LocalAndRemotePodOffloadingStrategyType", localAndRemoteNamespaceOffloading, &localAndRemoteNodeSelector),
		)
	})

	Context("Check if the pod NodeSelector is correctly merged with the NamespaceOffloading NodeSelector", func() {
		It("Check the merged NodeSelector", func() {
			podNodeSelector := testutils.GetPodNodeSelector()
			imposedNodeSelector := testutils.GetImposedNodeSelector("")
			mergedNodeSelector := utils.MergeNodeSelector(&podNodeSelector, &imposedNodeSelector)
			expectedMergedNodeSelector := testutils.GetMergedNodeSelector("")
			Expect(mergedNodeSelector).To(Equal(expectedMergedNodeSelector))
		})
	})

	Context("Check how the new NodeSelector is inserted into the pod", func() {
		// imposedNodeSelector that all Pod without NodeAffinity specified by user must have
		imposedNodeSelector := testutils.GetImposedNodeSelector("")
		// mergedNodeSelector is a merge of the NodeSelectors specified in NamespaceOffloading and NodeSelector
		// specified by the user
		mergedNodeSelector := testutils.GetMergedNodeSelector("")
		// A fake PodAffinity to test if it is preserved.
		podAffinity := corev1.PodAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
				TopologyKey: "fake-value",
			}},
		}
		podNodeSelector := testutils.GetPodNodeSelector()
		// There are 6 pods:
		// 0 - Pod without Affinity.
		// 1 - Pod with Affinity but no NodeAffinity.
		// 2 - Pod with Affinity and PodAffinity, but no NodeAffinity.
		// 3 - Pod with Affinity and NodeAffinity but no RequiredDuringSchedulingIgnoredDuringExecution.
		// 4 - Pod with Affinity and NodeAffinity and RequiredDuringSchedulingIgnoredDuringExecution but with 0 NodeSelectorTerms.
		// 5 - Pod with Affinity and NodeAffinity and RequiredDuringSchedulingIgnoredDuringExecution specified by the user.
		pods := []corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod",
					Namespace: "test",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod",
					Namespace: "test",
				},
				Spec: corev1.PodSpec{
					Affinity: &corev1.Affinity{
						NodeAffinity:    nil,
						PodAffinity:     nil,
						PodAntiAffinity: nil,
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod",
					Namespace: "test",
				},
				Spec: corev1.PodSpec{
					Affinity: &corev1.Affinity{
						NodeAffinity:    nil,
						PodAffinity:     &podAffinity,
						PodAntiAffinity: nil,
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod",
					Namespace: "test",
				},
				Spec: corev1.PodSpec{
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: nil,
						},
						PodAntiAffinity: nil,
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod",
					Namespace: "test",
				},
				Spec: corev1.PodSpec{
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{}},
						},
						PodAntiAffinity: nil,
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod",
					Namespace: "test",
				},
				Spec: corev1.PodSpec{
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: podNodeSelector.DeepCopy(),
						},
						PodAntiAffinity: nil,
					},
				},
			},
		}

		DescribeTable("6 Pods with different Affinity",
			func(imposedNodeSelector corev1.NodeSelector, pod corev1.Pod, expectedNodeSelector corev1.NodeSelector) {
				newPod := pod.DeepCopy()
				newNodeSelector := imposedNodeSelector.DeepCopy()
				fillPodWithTheNewNodeSelector(newNodeSelector, newPod)
				By("Checking the NodeSelector is not changed")
				Expect(*newNodeSelector).To(Equal(imposedNodeSelector))
				By("Checking the NodeSelector inserted in the Pod")
				Expect(*newPod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution).To(Equal(expectedNodeSelector))
			},
			Entry("0 - Pod without Affinity", imposedNodeSelector, pods[0], imposedNodeSelector),
			Entry("1 - Pod with Affinity but no NodeAffinity", imposedNodeSelector, pods[1], imposedNodeSelector),
			Entry("2 - Pod with Affinity and PodAffinity, but no NodeAffinity", imposedNodeSelector, pods[2], imposedNodeSelector),
			Entry("3 - Pod with Affinity and NodeAffinity but no RequiredDuringSchedulingIgnoredDuringExecution",
				imposedNodeSelector, pods[3], imposedNodeSelector),
			Entry("4 - Pod with Affinity and NodeAffinity and RequiredDuringSchedulingIgnoredDuringExecution but with 0 NodeSelectorTerms",
				imposedNodeSelector, pods[4], imposedNodeSelector),
			Entry("5 - Pod with Affinity and NodeAffinity and RequiredDuringSchedulingIgnoredDuringExecution specified by the user",
				imposedNodeSelector, pods[5], mergedNodeSelector),
		)

		It("Test that the PodAffinity in the case 2 is preserved", func() {
			newPod := pods[2].DeepCopy()
			newNodeSelector := imposedNodeSelector.DeepCopy()
			fillPodWithTheNewNodeSelector(newNodeSelector, newPod)
			Expect(*newPod.Spec.Affinity.PodAffinity).To(Equal(podAffinity))
		})
	})

	Context("Test the mutatePod function and observe the pod is correctly mutated", func() {

		podNodeSelector := testutils.GetPodNodeSelector()
		pod := corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod",
				Namespace: "test",
			},
			Spec: corev1.PodSpec{
				Tolerations: []corev1.Toleration{{
					Key:               "test",
					Operator:          "",
					Value:             "",
					TolerationSeconds: nil,
				}},
				Affinity: &corev1.Affinity{
					NodeAffinity: &corev1.NodeAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: podNodeSelector.DeepCopy(),
					},
					PodAntiAffinity: nil,
				},
			},
		}

		type offloadingTestCase struct {
			policy          offloadingv1beta1.PodOffloadingStrategyType
			hasRuntimeClass bool
		}

		DescribeTable("Test different combinations of offloading strategies and runtimeclass",
			func(testCase offloadingTestCase) {
				var expectedNodeSelectors corev1.NodeSelector
				var expectedTolerations []corev1.Toleration
				runtimeClassName := "my-liqo-runtime"

				namespaceOffloading := testutils.GetNamespaceOffloading(testCase.policy)
				podTest := pod.DeepCopy()

				resultingPolicy := testCase.policy
				if testCase.hasRuntimeClass {
					podTest.Spec.RuntimeClassName = &runtimeClassName
					// The runtimeclass forces the policy to Remote
					resultingPolicy = offloadingv1beta1.LocalPodOffloadingStrategyType
				}

				switch resultingPolicy {
				case offloadingv1beta1.LocalPodOffloadingStrategyType:
					expectedTolerations = pod.Spec.Tolerations
					expectedNodeSelectors = *pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
				default:
					expectedNodeSelectors = testutils.GetMergedNodeSelector(testCase.policy)
					expectedTolerations = append(expectedTolerations, pod.Spec.Tolerations[0], virtualNodeToleration)
				}

				// Mutate the pod according to the policy
				err := mutatePod(&namespaceOffloading, podTest, runtimeClassName)
				Expect(err).ToNot(HaveOccurred())

				// Compare tolerations
				Expect(len(podTest.Spec.Tolerations)).To(Equal(len(expectedTolerations)), "Unexpected number of tolerations")
				Expect(podTest.Spec.Tolerations).To(Equal(expectedTolerations), "No changes expected in tolerations")

				// Compare node selectors
				Expect(podTest.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms).To(
					ConsistOf(expectedNodeSelectors.NodeSelectorTerms),
					"Not matching node selectors",
				)
			},
			Entry("Local", offloadingTestCase{
				policy:          offloadingv1beta1.LocalPodOffloadingStrategyType,
				hasRuntimeClass: false}),
			Entry("Local + RC", offloadingTestCase{
				policy:          offloadingv1beta1.LocalPodOffloadingStrategyType,
				hasRuntimeClass: true}),
			Entry("Local&Remote", offloadingTestCase{
				policy:          offloadingv1beta1.LocalAndRemotePodOffloadingStrategyType,
				hasRuntimeClass: false}),
			Entry("Local&Remote + RC", offloadingTestCase{
				policy:          offloadingv1beta1.LocalAndRemotePodOffloadingStrategyType,
				hasRuntimeClass: true}),
			Entry("Remote", offloadingTestCase{
				policy:          offloadingv1beta1.RemotePodOffloadingStrategyType,
				hasRuntimeClass: false}),
			Entry("Remote + RC", offloadingTestCase{
				policy:          offloadingv1beta1.RemotePodOffloadingStrategyType,
				hasRuntimeClass: true}),
		)

		It("Checks that pod is mutated when a runtime class different than Liqo is defined ", func() {
			namespaceOffloading := testutils.GetNamespaceOffloading(offloadingv1beta1.LocalAndRemotePodOffloadingStrategyType)
			podTest := pod.DeepCopy()
			runtimeClassName := "my-custom-runtime"
			podTest.Spec.RuntimeClassName = &runtimeClassName

			err := mutatePod(&namespaceOffloading, podTest, "liqo")
			Expect(err).ToNot(HaveOccurred())
			Expect(len(podTest.Spec.Tolerations)).To(Equal(2), "Unexpected number of tolerations")
			Expect(podTest.Spec.Tolerations[1].MatchToleration(&virtualNodeToleration)).To(
				BeTrue(),
				"Added tolerations do not match the expected ones",
			)
			mergedNodeSelector := testutils.GetMergedNodeSelector(offloadingv1beta1.LocalAndRemotePodOffloadingStrategyType)
			Expect(*podTest.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution).To(
				Equal(mergedNodeSelector),
				"Node affinities do not match the expected ones",
			)
		})
	})
})
