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

package shadowpod

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

var _ = Describe("Peering Info", func() {
	var (
		spValidator                  *Validator
		found                        bool
		dryRun                       bool
		cache                        *peeringCache
		fakeNamespace                *corev1.Namespace
		peeringInfo, peeringInfoTest *peeringInfo
		spd                          *Description
		err                          error
		errTest                      error
		fakeClient                   client.Client
		shadowPod                    *offloadingv1beta1.ShadowPod
		containers                   []containerResource
	)

	BeforeEach(func() {

		fakeNamespace = testutil.FakeNamespaceWithClusterID(clusterID, testNamespace)

		fakeClient = fake.NewClientBuilder().WithScheme(scheme).WithObjects(fakeNamespace).Build()

		spValidator = webhook.Admission{Handler: NewValidator(fakeClient, false)}.Handler.(*Validator)

		cache = spValidator.PeeringCache

		peeringInfoTest = createPeeringInfo(userName, *resourceQuota)

		containers = []containerResource{{cpu: int64(resourceCPU), memory: int64(resourceMemory)}}

		shadowPod = forgeShadowPodWithResourceRequests(containers, nil)
	})

	Describe("Get or Create a PeeringInfo", func() {
		JustBeforeEach(func() {
			peeringInfo = cache.getOrCreatePeeringInfo(userName, *resourceQuota)
		})

		When("The Peering Info exists", func() {
			BeforeEach(func() {
				cache.peeringInfo.Store(userName, createPeeringInfo(userName, *forgeResourceList(int64(resourceCPU*2), int64(resourceMemory*2))))
			})
			It("should return a peering info", func() {
				Expect(peeringInfo).To(Equal(peeringInfoTest))
			})
		})
		When("The Peering Info does not exist", func() {
			It("should return a peering info", func() {
				Expect(peeringInfo).To(Equal(peeringInfoTest))
			})
		})
	})

	Describe("Get a PeeringInfo", func() {
		JustBeforeEach(func() {
			peeringInfo, found = cache.getPeeringInfo(userName)
		})

		When("The Peering Info exists", func() {
			BeforeEach(func() {
				cache.peeringInfo.Store(userName, createPeeringInfo(userName, *resourceQuota))
			})
			It("should return a peering info and found is true", func() {
				Expect(found).To(BeTrue())
				Expect(peeringInfo).To(Equal(peeringInfoTest))
			})
		})
		When("The Peering Info does not exist", func() {
			It("should not return a peering info and found is false", func() {
				Expect(peeringInfo).To(BeNil())
				Expect(found).To(BeFalse())
			})
		})
	})

	Describe("Check Resources in a PeeringInfo", func() {
		JustBeforeEach(func() {
			err = peeringInfo.checkResources(spd)
		})

		When("resources are available", func() {
			BeforeEach(func() {
				spd = createShadowPodDescription(testShadowPodName, testNamespace, testShadowPodUID, *resourceQuota)
				peeringInfo = createPeeringInfo(userName, *resourceQuota)
			})
			It("should not return any error", func() {
				Expect(err).To(BeNil())
			})
		})
		When("CPU resources are not available", func() {
			BeforeEach(func() {
				resourceQuotaLower := forgeResourceList(int64(resourceCPU/2), int64(resourceMemory))
				peeringInfo = createPeeringInfo(userName, *resourceQuotaLower)
				spd = createShadowPodDescription(testShadowPodName, testNamespace, testShadowPodUID, *resourceQuota)
				freeQuota := peeringInfo.getFreeQuota()
				errTest = fmt.Errorf("peering %s quota usage exceeded - free %s / requested %s",
					corev1.ResourceCPU, freeQuota.Cpu().String(), resourceQuota.Cpu().String())
			})
			It("should return an error", func() {
				Expect(err).To(Equal(errTest))
			})
		})
		When("A requested resource quota is not defined for a specific peering", func() {
			BeforeEach(func() {
				peeringInfo = createPeeringInfo(userName, *resourceQuota)
				resourcesWithGpu := forgeResourceList(int64(resourceCPU), int64(resourceMemory), 1000)
				spd = createShadowPodDescription(testShadowPodName, testNamespace, testShadowPodUID, *resourcesWithGpu)
				errTest = fmt.Errorf("nvidia.com/gpu quota limit not found for this peering")
			})
			It("should return an error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(errTest))
			})
		})
	})

	Describe("Test and update creation", func() {
		JustBeforeEach(func() {
			err = peeringInfo.testAndUpdateCreation(ctx, fakeClient, shadowPod, offloadingv1beta1.SoftLimitsEnforcement, dryRun)
		})

		When("resources are available and dryRun flag is false", func() {
			BeforeEach(func() {
				dryRun = false
				peeringInfo = createPeeringInfo(userName, *resourceQuota)
			})
			It("should not return any error and available resources will be decremented", func() {
				Expect(err).To(BeNil())
				Expect(peeringInfo.usedQuota).To(Equal(*resourceQuota))
				Expect(peeringInfo.getFreeQuota()).To(Equal(*freeQuotaZero))
			})
		})
		When("resources are available and dryRun flag is true", func() {
			BeforeEach(func() {
				dryRun = true
				peeringInfo = createPeeringInfo(userName, *resourceQuota)
			})
			It("should not return any error and available resources will not be decremented", func() {
				Expect(err).To(BeNil())
				Expect(peeringInfo.usedQuota.Cpu().Value()).To(Equal(freeQuotaZero.Cpu().Value()))
				freeQuota := peeringInfo.getFreeQuota()
				Expect(freeQuota.Cpu().Value()).To(Equal(int64(resourceCPU)))
			})
		})
		When("resources are not available and dryRun flag is false", func() {
			BeforeEach(func() {
				dryRun = false
				resourceQuotaLower := forgeResourceList(int64(resourceCPU/2), int64(resourceMemory))
				peeringInfo = createPeeringInfo(userName, *resourceQuotaLower)
			})
			It("should return an error and available resources will not be decremented", func() {
				Expect(err).ToNot(BeNil())
				Expect(peeringInfo.usedQuota.Cpu().Value()).To(Equal(freeQuotaZero.Cpu().Value()))
			})
		})
		When("resources are not available and dryRun flag is true", func() {
			BeforeEach(func() {
				dryRun = true
				resourceQuotaLower := forgeResourceList(int64(resourceCPU/2), int64(resourceMemory))
				peeringInfo = createPeeringInfo(userName, *resourceQuotaLower)
			})
			It("should return an error and available resources will not be decremented", func() {
				Expect(err).ToNot(BeNil())
				Expect(peeringInfo.usedQuota.Cpu().Value()).To(Equal(freeQuotaZero.Cpu().Value()))
			})
		})
	})

	Describe("Update deletion", func() {
		JustBeforeEach(func() {
			err = peeringInfo.updateDeletion(shadowPod, dryRun)
		})

		When("Shadow pod description exists and dryRun flag is false", func() {
			BeforeEach(func() {
				dryRun = false
				peeringInfo = createPeeringInfo(userName, *resourceQuota)
				peeringInfo.addShadowPod(createShadowPodDescription(testShadowPodName, testNamespace, testShadowPodUID, *resourceQuota))
			})
			It("should not return any error and available resources will be incremented", func() {
				Expect(err).To(BeNil())
				Expect(peeringInfo.usedQuota).To(Equal(*freeQuotaZero))
				Expect(peeringInfo.getFreeQuota()).To(Equal(*resourceQuota))
			})
		})
		When("Shadow pod description exists and dryRun flag is true", func() {
			BeforeEach(func() {
				dryRun = true
				peeringInfo = createPeeringInfo(userName, *resourceQuota)
				peeringInfo.addShadowPod(createShadowPodDescription(testShadowPodName, testNamespace, testShadowPodUID, *resourceQuota))
			})
			It("should not return any error and available resources will not be incremented", func() {
				Expect(err).To(BeNil())
				Expect(peeringInfo.usedQuota).To(Equal(*resourceQuota))
				Expect(peeringInfo.getFreeQuota()).To(Equal(*freeQuotaZero))
			})
		})
		When("Shadow pod description does not exist", func() {
			BeforeEach(func() {
				peeringInfo = createPeeringInfo(userName, *resourceQuota)
			})
			It("should return an error", func() {
				Expect(err).ToNot(BeNil())
			})
		})
	})
})
