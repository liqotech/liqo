// Copyright 2019-2026 The Liqo Authors
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

package shadowpod

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/event"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
)

var _ = Describe("ShadowPod quota releaser", func() {
	var (
		spValidator *Validator
		peering     *peeringInfo
		shadowPod   *offloadingv1beta1.ShadowPod
		nsname      types.NamespacedName
	)

	BeforeEach(func() {
		spValidator = &Validator{PeeringCache: &peeringCache{ready: true}}
		peering = createPeeringInfo(userName, *resourceQuota)
		spValidator.PeeringCache.peeringInfo.Store(userName, peering)

		shadowPod = forgeShadowPod(testShadowPodName, testNamespace, string(testShadowPodUID), userName)
		nsname = types.NamespacedName{Name: testShadowPodName, Namespace: testNamespace}

		// The ShadowPod is accounted in the used quota, as if it had been admitted while running.
		peering.addShadowPod(createShadowPodDescription(testShadowPodName, testNamespace, testShadowPodUID, *resourceQuota4))
		Expect(peering.usedQuota.Cpu().Value()).To(Equal(resourceQuota4.Cpu().Value()))
	})

	When("the ShadowPod reaches a terminal phase", func() {
		It("should release its quota without waiting for the cache refresh", func() {
			shadowPod.Status.Phase = corev1.PodSucceeded
			spValidator.releaseQuotaOfTerminatedShadowPod(shadowPod)

			Expect(peering.usedQuota.Cpu().Value()).To(Equal(int64(0)))
			Expect(peering.usedQuota.Memory().Value()).To(Equal(int64(0)))
			Expect(peering.shadowPods[nsname.String()].active).To(BeFalse())
		})
	})

	When("the ShadowPod is still running", func() {
		It("should keep its quota accounted", func() {
			shadowPod.Status.Phase = corev1.PodRunning
			spValidator.releaseQuotaOfTerminatedShadowPod(shadowPod)

			Expect(peering.usedQuota.Cpu().Value()).To(Equal(resourceQuota4.Cpu().Value()))
			Expect(peering.shadowPods[nsname.String()].active).To(BeTrue())
		})
	})

	When("the same terminal ShadowPod is processed more than once", func() {
		It("should not release the quota twice", func() {
			shadowPod.Status.Phase = corev1.PodFailed
			spValidator.releaseQuotaOfTerminatedShadowPod(shadowPod)
			spValidator.releaseQuotaOfTerminatedShadowPod(shadowPod)

			Expect(peering.usedQuota.Cpu().Value()).To(Equal(int64(0)))
		})
	})

	When("the ShadowPod is not accounted in cache", func() {
		It("should be a no-op", func() {
			shadowPod = forgeShadowPod("unknown", testNamespace, "unknown-uid", userName)
			shadowPod.Status.Phase = corev1.PodSucceeded

			spValidator.releaseQuotaOfTerminatedShadowPod(shadowPod)

			Expect(peering.usedQuota.Cpu().Value()).To(Equal(resourceQuota4.Cpu().Value()))
		})
	})

	When("the creator has no PeeringInfo in cache", func() {
		It("should be a no-op", func() {
			shadowPod = forgeShadowPod("unknown-user", testNamespace, "unknown-uid", userName3)
			shadowPod.Status.Phase = corev1.PodSucceeded

			spValidator.releaseQuotaOfTerminatedShadowPod(shadowPod)

			Expect(peering.usedQuota.Cpu().Value()).To(Equal(resourceQuota4.Cpu().Value()))
		})
	})

	When("the ShadowPod has no creator label", func() {
		It("should be a no-op", func() {
			shadowPod = forgeShadowPod("no-creator", testNamespace, "no-creator-uid", userName)
			delete(shadowPod.Labels, consts.CreatorLabelKey)
			shadowPod.Status.Phase = corev1.PodSucceeded

			spValidator.releaseQuotaOfTerminatedShadowPod(shadowPod)

			Expect(peering.usedQuota.Cpu().Value()).To(Equal(resourceQuota4.Cpu().Value()))
		})
	})

	Describe("terminal phase predicate", func() {
		phasePredicate := terminalPhasePredicate()

		It("should ignore non-terminal transitions", func() {
			oldShadowPod := forgeShadowPod(testShadowPodName, testNamespace, string(testShadowPodUID), userName)
			newShadowPod := oldShadowPod.DeepCopy()

			Expect(phasePredicate.Update(event.UpdateEvent{ObjectOld: oldShadowPod, ObjectNew: newShadowPod})).To(BeFalse())

			newShadowPod.Status.Phase = corev1.PodRunning
			Expect(phasePredicate.Update(event.UpdateEvent{ObjectOld: oldShadowPod, ObjectNew: newShadowPod})).To(BeFalse())
		})

		It("should trigger on the transition to a terminal phase", func() {
			oldShadowPod := forgeShadowPod(testShadowPodName, testNamespace, string(testShadowPodUID), userName)
			oldShadowPod.Status.Phase = corev1.PodRunning

			for _, phase := range []corev1.PodPhase{corev1.PodSucceeded, corev1.PodFailed} {
				newShadowPod := oldShadowPod.DeepCopy()
				newShadowPod.Status.Phase = phase
				Expect(phasePredicate.Update(event.UpdateEvent{ObjectOld: oldShadowPod, ObjectNew: newShadowPod})).To(BeTrue())
			}
		})

		It("should trigger on the creation of an already terminal ShadowPod", func() {
			shadowPod.Status.Phase = corev1.PodSucceeded
			Expect(phasePredicate.Create(event.CreateEvent{Object: shadowPod})).To(BeTrue())

			shadowPod.Status.Phase = corev1.PodRunning
			Expect(phasePredicate.Create(event.CreateEvent{Object: shadowPod})).To(BeFalse())
		})
	})
})
