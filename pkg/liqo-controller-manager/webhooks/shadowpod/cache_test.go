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

package shadowpod

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	vkv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
)

var _ = Describe("Webhook Cache", func() {
	var (
		spValidator    *Validator
		fakeShadowPod3 *vkv1alpha1.ShadowPod
		fakeShadowPod4 *vkv1alpha1.ShadowPod
		fakeClient     client.Client
		fakeCache      *peeringCache
		spvClient      client.Client
		peering        *peeringInfo
		err            error
		errClient      error
		spList         *vkv1alpha1.ShadowPodList
	)

	BeforeEach(func() {

		fakeClient = fake.NewClientBuilder().WithScheme(scheme).WithObjects(fakeShadowPod, fakeShadowPod2, foreignCluster, resourceOffer).Build()

		spValidator = webhook.Admission{Handler: NewValidator(fakeClient, true)}.Handler.(*Validator)

		fakeCache = spValidator.PeeringCache

		spvClient = spValidator.client

	})

	Describe("Initialize Cache", func() {
		JustBeforeEach(func() {
			err = spValidator.initializeCache(ctx)
		})

		When("Align existing ResourceOffers and ShadowPod", func() {
			It("should align correctly the cache info", func() {
				Expect(err).ToNot(HaveOccurred())
				pi, found := fakeCache.peeringInfo.Load(clusterID)
				Expect(found).To(BeTrue())
				sp1 := pi.(*peeringInfo).shadowPods[nsName.String()]
				sp2 := pi.(*peeringInfo).shadowPods[nsName2.String()]
				Expect(sp1).ToNot(BeNil())
				Expect(sp2).ToNot(BeNil())
				peering = pi.(*peeringInfo)
				Expect(peering.usedQuota.Cpu().Value()).To(Equal(resourceQuota2.Cpu().Value()))
				Expect(peering.usedQuota.Memory().Value()).To(Equal(resourceQuota2.Memory().Value()))
				Expect(fakeCache.ready).To(BeTrue())
			})
		})
	})

	Describe("Align existing ShadowPod", func() {
		JustBeforeEach(func() {
			peering.alignExistingShadowPods(spList)
		})

		When("Some ShadowPods do not exist in a specific peeringInfo", func() {
			BeforeEach(func() {
				spList = forgeShadowPodList(fakeShadowPod, fakeShadowPod2)
				sp1 := createShadowPodDescription(testShadowPodName, testNamespace, testShadowPodUID, *resourceQuota4)
				peering = createPeeringInfo(*clusterIdentity, *resourceQuota)
				peering.addShadowPod(sp1)
			})
			It("should align correctly the peering info", func() {
				Expect(peering.shadowPods).To(HaveLen(2))
				Expect(peering.shadowPods[nsName2.String()]).ToNot(BeNil())
				Expect(peering.usedQuota.Cpu().Value()).To(Equal(resourceQuota2.Cpu().Value()))
				Expect(peering.usedQuota.Memory().Value()).To(Equal(resourceQuota2.Memory().Value()))
			})
		})
	})

	Describe("Align terminating or not existing ShadowPods", func() {
		JustBeforeEach(func() {
			peering.alignTerminatingOrNotExistingShadowPods(spList)
		})

		When("Some ShadowPods do not exist in a specific peeringInfo some others are in terminating", func() {
			BeforeEach(func() {
				spList = forgeShadowPodList(fakeShadowPod, fakeShadowPod2)
				sp1 := createShadowPodDescription(testShadowPodName, testNamespace, testShadowPodUID, *resourceQuota4)
				sp3 := createShadowPodDescription(testShadowPodName+"-3", testNamespace, testShadowPodUID+"-3", *resourceQuota4)
				sp4 := createShadowPodDescription(testShadowPodName+"-4", testNamespace, testShadowPodUID+"-4", *resourceQuota4)
				sp4.creationTimestamp = time.Now().Add(time.Duration(-40) * time.Second)
				sp5 := createShadowPodDescription(testShadowPodName+"-5", testNamespace, testShadowPodUID+"-5", *resourceQuota4)
				peering = createPeeringInfo(*clusterIdentity, *resourceQuota)
				peering.addShadowPod(sp1)
				peering.addShadowPod(sp3)
				peering.addShadowPod(sp4)
				peering.addShadowPod(sp5)
				peering.terminateShadowPod(sp3)
				Expect(peering.shadowPods).To(HaveLen(4))
				Expect(peering.shadowPods[nsName2.String()]).To(BeNil())
			})
			It("should correctly add and/or delete ShadowPods Description from Peering info", func() {
				Expect(peering.shadowPods).To(HaveLen(3))
				Expect(peering.shadowPods[nsName2.String()]).ToNot(BeNil())
				Expect(peering.shadowPods[nsName.String()]).ToNot(BeNil())
				nsName5 := types.NamespacedName{Name: testShadowPodName + "-5", Namespace: testNamespace}
				Expect(peering.shadowPods[nsName5.String()]).ToNot(BeNil())
				Expect(peering.shadowPods[nsName4.String()]).To(BeNil())
				Expect(peering.shadowPods[nsName3.String()]).To(BeNil())
				Expect(peering.usedQuota.Cpu().Value()).To(Equal(3 * resourceQuota4.Cpu().Value()))
			})
		})
	})

	Describe("Check alignment ResourceOffer - PeeringInfo", func() {
		JustBeforeEach(func() {
			err = spValidator.checkAlignmentResourceOfferPeeringInfo(ctx)
		})

		When("Some ResourceOffer are not yet aligned in cache or any PeeringInfo have not anymore corresponding ResourceOffer in the system", func() {
			BeforeEach(func() {
				errClient = spvClient.Create(ctx, resourceOffer2)
				Expect(errClient).ToNot(HaveOccurred())
				errClient = spvClient.Create(ctx, foreignCluster2)
				Expect(errClient).ToNot(HaveOccurred())
				fakeShadowPod3 = forgeShadowPod(nsName3.Name, nsName3.Namespace, string(testShadowPodUID3), clusterID2)
				fakeShadowPod4 = forgeShadowPod(nsName4.Name, nsName4.Namespace, string(testShadowPodUID4), clusterID2)
				errClient = spvClient.Create(ctx, fakeShadowPod3)
				Expect(errClient).ToNot(HaveOccurred())
				errClient = spvClient.Create(ctx, fakeShadowPod4)
				Expect(errClient).ToNot(HaveOccurred())
				sp1 := createShadowPodDescription(testShadowPodName, testNamespace, testShadowPodUID, *resourceQuota4)
				sp2 := createShadowPodDescription(testShadowPodName2, testNamespace, testShadowPodUID2, *resourceQuota4)
				sp5 := createShadowPodDescription(testShadowPodName+"-5", testNamespace+"-3", testShadowPodUID+"-5", *resourceQuota4)
				sp6 := createShadowPodDescription(testShadowPodName+"-6", testNamespace+"-3", testShadowPodUID+"-6", *resourceQuota4)
				peering = createPeeringInfo(*clusterIdentity, *resourceQuota)
				peeringToBeDeleted := createPeeringInfo(*forgeClusterIdentity(clusterName3, clusterID3), *resourceQuota)
				peering.addShadowPod(sp1)
				peering.addShadowPod(sp2)
				peeringToBeDeleted.addShadowPod(sp5)
				peeringToBeDeleted.addShadowPod(sp6)
				fakeCache.peeringInfo.Store(clusterID, peering)
				fakeCache.peeringInfo.Store(clusterID3, peeringToBeDeleted)

				spList, _ = spValidator.getShadowPodListByClusterID(ctx, clusterID)
				Expect(spList.Items).To(HaveLen(2))
				spList, _ = spValidator.getShadowPodListByClusterID(ctx, clusterID2)
				Expect(spList.Items).To(HaveLen(2))
				spList, _ = spValidator.getShadowPodListByClusterID(ctx, clusterID3)
				Expect(spList.Items).To(HaveLen(0))

				_, ok := fakeCache.getPeeringInfo(*clusterIdentity)
				Expect(ok).To(BeTrue())
				_, ok = fakeCache.getPeeringInfo(*clusterIdentity2)
				Expect(ok).To(BeFalse())
				_, ok = fakeCache.getPeeringInfo(*forgeClusterIdentity(clusterName3, clusterID3))
				Expect(ok).To(BeTrue())
			})
			It("should correctly add and/or delete PeeringInfo and corresponding ShadowPods from cache", func() {
				_, ok := fakeCache.getPeeringInfo(*clusterIdentity)
				Expect(ok).To(BeTrue())
				peering, ok = fakeCache.getPeeringInfo(*clusterIdentity2)
				Expect(ok).To(BeTrue())
				_, ok = fakeCache.getPeeringInfo(*forgeClusterIdentity(clusterName3, clusterID3))
				Expect(ok).To(BeFalse())
				Expect(peering.shadowPods[nsName3.String()]).ToNot(BeNil())
				Expect(peering.shadowPods[nsName4.String()]).ToNot(BeNil())
			})
		})
	})
})
