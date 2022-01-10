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

package mutate

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	offv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	testutils "github.com/liqotech/liqo/pkg/mutate/testUtils"
	"github.com/liqotech/liqo/pkg/utils"
)

func TestWebhookManager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Webhook Suite")
}

var _ = Describe("Webhook", func() {

	var (
		virtualNodeToleration = corev1.Toleration{
			Key:      liqoconst.VirtualNodeTolerationKey,
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoExecute,
		}
	)

	Context("1 - Check the new created toleration according to the PodOffloadingStrategy", func() {
		emptyToleration := corev1.Toleration{}
		DescribeTable("3 Different type of PodOffloadingStrategy",
			func(strategy offv1alpha1.PodOffloadingStrategyType, expectedToleration corev1.Toleration) {
				By(fmt.Sprintf("Testing %s", strategy))
				toleration, err := createTolerationFromNamespaceOffloading(strategy)
				if strategy == offv1alpha1.LocalPodOffloadingStrategyType {
					Expect(err != nil).Should(BeTrue())
				}
				Expect(toleration.MatchToleration(&expectedToleration)).To(BeTrue())
			},
			Entry("LocalPodOffloadingStrategyType", offv1alpha1.LocalPodOffloadingStrategyType, emptyToleration),
			Entry("RemotePodOffloadingStrategyType", offv1alpha1.RemotePodOffloadingStrategyType, virtualNodeToleration),
			Entry("LocalAndRemotePodOffloadingStrategyType", offv1alpha1.LocalAndRemotePodOffloadingStrategyType, virtualNodeToleration),
		)
	})

	Context("2 - Check the NodeSelector imposed by the NamespaceOffloading", func() {
		// slice with 3 namespaceOffloading one for each PodOffloadingStrategy
		namespaceOffloadings := []offv1alpha1.NamespaceOffloading{
			testutils.GetNamespaceOffloading(offv1alpha1.LocalPodOffloadingStrategyType),
			testutils.GetNamespaceOffloading(offv1alpha1.RemotePodOffloadingStrategyType),
			testutils.GetNamespaceOffloading(offv1alpha1.LocalAndRemotePodOffloadingStrategyType),
		}

		nodeSelectors := []corev1.NodeSelector{
			{},
			testutils.GetImposedNodeSelector(offv1alpha1.RemotePodOffloadingStrategyType),
			testutils.GetImposedNodeSelector(offv1alpha1.LocalAndRemotePodOffloadingStrategyType),
		}
		DescribeTable("3 Different type of PodOffloadingStrategy",
			func(namespaceOffloading offv1alpha1.NamespaceOffloading, expectedNodeSelector corev1.NodeSelector) {
				By(fmt.Sprintf("Testing %s", namespaceOffloading.Spec.PodOffloadingStrategy))
				nodeSelector, err := createNodeSelectorFromNamespaceOffloading(&namespaceOffloading)
				if namespaceOffloading.Spec.PodOffloadingStrategy == offv1alpha1.LocalPodOffloadingStrategyType {
					Expect(err != nil).Should(BeTrue())
				}
				Expect(nodeSelector).To(Equal(expectedNodeSelector))
			},
			Entry("LocalPodOffloadingStrategyType", namespaceOffloadings[0], nodeSelectors[0]),
			Entry("RemotePodOffloadingStrategyType", namespaceOffloadings[1], nodeSelectors[1]),
			Entry("LocalAndRemotePodOffloadingStrategyType", namespaceOffloadings[2], nodeSelectors[2]),
		)
	})

	Context("3 - Check if the pod NodeSelector is correctly merged with the NamespaceOffloading NodeSelector", func() {
		It("Check the merged NodeSelector", func() {
			podNodeSelector := testutils.GetPodNodeSelector()
			imposedNodeSelector := testutils.GetImposedNodeSelector("")
			mergedNodeSelector := utils.MergeNodeSelector(&podNodeSelector, &imposedNodeSelector)
			expectedMergedNodeSelector := testutils.GetMergedNodeSelector("")
			Expect(mergedNodeSelector).To(Equal(expectedMergedNodeSelector))
		})
	})

	Context("4 - Check how the new NodeSelector is inserted into the pod", func() {
		// imposedNodeSelector that all Pod without NodeAffinity specified by user must have
		imposedNodeSelector := testutils.GetImposedNodeSelector("")
		// mergedNodeSelector is a merge of NodeSelector specified in NamespaceOffloading and NodeSelector
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

	Context("5 - Call the mutatePod function and observe the pod is correctly mutated", func() {

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

		It("Check the toleration added and the new NodeSelector", func() {
			namespaceOffloading := testutils.GetNamespaceOffloading(offv1alpha1.LocalAndRemotePodOffloadingStrategyType)
			podTest := pod.DeepCopy()
			err := mutatePod(&namespaceOffloading, podTest)
			Expect(err == nil).To(BeTrue())
			Expect(len(podTest.Spec.Tolerations) == 2).To(BeTrue())
			Expect(podTest.Spec.Tolerations[1].MatchToleration(&virtualNodeToleration)).To(BeTrue())
			mergedNodeSelector := testutils.GetMergedNodeSelector(offv1alpha1.LocalAndRemotePodOffloadingStrategyType)
			Expect(*podTest.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution).To(Equal(mergedNodeSelector))
		})

		It("With LocalPodOffloadingStrategy check that pod is not mutated ", func() {
			namespaceOffloading := testutils.GetNamespaceOffloading(offv1alpha1.LocalPodOffloadingStrategyType)
			podTest := pod.DeepCopy()
			oldPodNodeSelector := *podTest.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
			err := mutatePod(&namespaceOffloading, podTest)
			Expect(err == nil).To(BeTrue())
			Expect(len(podTest.Spec.Tolerations) == 1).To(BeTrue())
			Expect(*podTest.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution).To(Equal(oldPodNodeSelector))
		})
	})
})
