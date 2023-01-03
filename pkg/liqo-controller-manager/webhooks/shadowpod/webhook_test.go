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
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	vkv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
)

var _ = Describe("Validating webhook", func() {

	var (
		spValidator              *Validator
		spValidatorWithResources *Validator
		request                  admission.Request
		response                 admission.Response
		fakeNewShadowPod         *vkv1alpha1.ShadowPod
		fakeNamespace            *corev1.Namespace
		fakeClient               client.Client
		spvClient                client.Client
		containers               []containerResource
		peeringInfo              *peeringInfo
		errClient                error
	)

	BeforeEach(func() {

		fakeNamespace = forgeNamespaceWithClusterID(clusterID)

		fakeClient = fake.NewClientBuilder().WithScheme(scheme).WithObjects(fakeNamespace, foreignCluster, resourceOffer).Build()

		spValidator = webhook.Admission{Handler: NewValidator(fakeClient, false)}.Handler.(*Validator)

		spValidatorWithResources = webhook.Admission{Handler: NewValidator(fakeClient, true)}.Handler.(*Validator)

		spValidatorWithResources.PeeringCache = &peeringCache{
			ready: true,
		}
		Expect(spValidator.InjectDecoder(decoder)).To(Succeed())
		Expect(spValidatorWithResources.InjectDecoder(decoder)).To(Succeed())

		spvClient = spValidatorWithResources.client
	})

	Describe("Validating ShadowPod without resource validation", func() {
		JustBeforeEach(func() {
			response = spValidator.Handle(ctx, request)
		})

		When("the shadowpod has an invalid clusterID label", func() {
			BeforeEach(func() {
				fakeNewShadowPod = forgeShadowPodWithClusterID(clusterIDInvalid, testNamespace)
				request = forgeRequest(admissionv1.Create, fakeNewShadowPod, nil)
			})
			It("should return a forbidden response", func() {
				Expect(response.Allowed).To(BeFalse())
				Expect(response.Result.Code).To(BeNumerically("==", http.StatusForbidden))
			})
		})
		When("the shadowpod has a valid clusterID label", func() {
			BeforeEach(func() {
				fakeNewShadowPod = forgeShadowPodWithClusterID(clusterID, testNamespace)
				request = forgeRequest(admissionv1.Create, fakeNewShadowPod, nil)
			})
			It("should admit the request", func() {
				Expect(response.Allowed).To(BeTrue())
			})
		})
		When("the shadowpod namespace not exists", func() {
			BeforeEach(func() {
				fakeNewShadowPod = forgeShadowPodWithClusterID(clusterID, testNamespaceInvalid)
				request = forgeRequest(admissionv1.Create, fakeNewShadowPod, nil)
			})
			It("should return a bad request response", func() {
				Expect(response.Allowed).To(BeFalse())
				Expect(response.Result.Code).To(BeNumerically("==", http.StatusBadRequest))
			})
		})
	})

	Describe("Handle creation ShadowPod with resource validation", func() {
		JustBeforeEach(func() {
			response = spValidatorWithResources.Handle(ctx, request)
		})

		When("The ResourceOffer exists and required ShadowPod resource limits are available", func() {
			BeforeEach(func() {
				containers = nil
				containers = append(containers, containerResource{cpu: int64(resourceCPU), memory: int64(resourceMemory)})
				fakeNewShadowPod = forgeShadowPodWithResourceLimits(containers, nil)
				request = forgeRequest(admissionv1.Create, fakeNewShadowPod, nil)
			})
			It("request is allowed", func() {
				Expect(response.Allowed).To(BeTrue())
			})
		})
		When("The ResourceOffer exists but required ShadowPod resource limits are not available", func() {
			BeforeEach(func() {
				containers = nil
				containers = append(containers, containerResource{cpu: int64(resourceCPU * 2), memory: int64(resourceMemory)})
				fakeNewShadowPod = forgeShadowPodWithResourceLimits(containers, nil)
				request = forgeRequest(admissionv1.Create, fakeNewShadowPod, nil)
			})
			It("request is denied with error 403", func() {
				Expect(response.Allowed).To(BeFalse())
				Expect(response.Result.Code).To(BeNumerically("==", http.StatusForbidden))
			})
		})
		When("The ResourceOffer does not exist", func() {
			BeforeEach(func() {
				errClient = spvClient.Delete(ctx, forgeResourceOfferWithLabel(clusterName, tenantNamespace, clusterID))
				Expect(errClient).ToNot(HaveOccurred())
				containers = nil
				containers = append(containers, containerResource{cpu: int64(resourceCPU * 2), memory: int64(resourceMemory)})
				fakeNewShadowPod = forgeShadowPodWithResourceLimits(containers, nil)
				request = forgeRequest(admissionv1.Create, fakeNewShadowPod, nil)
			})
			It("request is denied with error 500", func() {
				Expect(response.Allowed).To(BeFalse())
				Expect(response.Result.Code).To(BeNumerically("==", http.StatusInternalServerError))
			})
		})
	})

	Describe("Handle deletion ShadowPod with resource validation", func() {
		JustBeforeEach(func() {
			response = spValidatorWithResources.Handle(ctx, request)
		})

		When("The ResourceOffer exists and the ShadowPod Description exists and is running", func() {
			BeforeEach(func() {
				peeringInfo = createPeeringInfo(*clusterIdentity, *resourceQuota)
				peeringInfo.addShadowPod(
					createShadowPodDescription(testShadowPodName, testNamespace, testShadowPodUID, *resourceQuota))
				spValidatorWithResources.PeeringCache.peeringInfo.Store(clusterID, peeringInfo)
				containers = nil
				containers = append(containers, containerResource{cpu: int64(resourceCPU), memory: int64(resourceMemory)})
				fakeNewShadowPod = forgeShadowPodWithResourceLimits(containers, nil)
				request = forgeRequest(admissionv1.Delete, nil, fakeNewShadowPod)
			})
			It("request is allowed without errors and all resources are restored", func() {
				Expect(response.Allowed).To(BeTrue())
				Expect(response.Result.Code).To(BeNumerically("==", http.StatusOK))
				Expect(peeringInfo.usedQuota).To(Equal(*freeQuotaZero))
				ns := types.NamespacedName{Name: fakeNewShadowPod.Name, Namespace: fakeNewShadowPod.Namespace}
				Expect(peeringInfo.shadowPods[ns.String()].running).To(BeFalse())
			})
		})
		When("The ResourceOffer exists but the ShadowPod Description does not exist", func() {
			BeforeEach(func() {
				peeringInfo = createPeeringInfo(*clusterIdentity, *resourceQuota)
				spValidatorWithResources.PeeringCache.peeringInfo.Store(clusterID, peeringInfo)
				containers = nil
				containers = append(containers, containerResource{cpu: int64(resourceCPU), memory: int64(resourceMemory)})
				fakeNewShadowPod = forgeShadowPodWithResourceLimits(containers, nil)
				request = forgeRequest(admissionv1.Delete, nil, fakeNewShadowPod)
			})
			It("request is allowed with error (Cache Problem)", func() {
				Expect(response.Allowed).To(BeTrue())
				Expect(response.Result.Reason).To(Equal((metav1.StatusReason("ShadowPod " + testShadowPodName + " not found (Maybe Cache problem)"))))
			})
		})
		When("The PeeringInfo does not exist", func() {
			BeforeEach(func() {
				containers = nil
				containers = append(containers, containerResource{cpu: int64(resourceCPU), memory: int64(resourceMemory)})
				fakeNewShadowPod = forgeShadowPodWithResourceLimits(containers, nil)
				request = forgeRequest(admissionv1.Delete, nil, fakeNewShadowPod)
			})
			It("request is allowed with error (PeeringInfo not found)", func() {
				Expect(response.Allowed).To(BeTrue())
				Expect(response.Result.Reason).To(Equal((metav1.StatusReason(fmt.Sprintf("Peering not found in cache for cluster %q", clusterName)))))
			})
		})
	})
})
