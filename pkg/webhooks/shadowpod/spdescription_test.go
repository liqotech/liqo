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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

var _ = Describe("ShadowPod Description", func() {
	var (
		spValidator       *Validator
		err               error
		errTest           error
		shadowPod         *offloadingv1beta1.ShadowPod
		spDescription     *Description
		spDescriptionTest *Description
		spNamespacedName  types.NamespacedName
		fakeNamespace     *corev1.Namespace
		peeringInfo       *peeringInfo
		fakeClient        client.Client
		quota             *corev1.ResourceList
		containers        []containerResource
		initContainers    []containerResource
	)

	BeforeEach(func() {

		fakeNamespace = testutil.FakeNamespaceWithClusterID(clusterID, tenantNamespace)

		fakeClient = fake.NewClientBuilder().WithScheme(scheme).WithObjects(fakeNamespace).Build()

		spValidator = webhook.Admission{Handler: NewValidator(fakeClient, false)}.Handler.(*Validator)

		peeringInfo = createPeeringInfo(userName, *resourceQuota)

		spNamespacedName = types.NamespacedName{Name: testShadowPodName, Namespace: testNamespace}
	})

	Describe("Get or Create a ShadowPod Description", func() {
		JustBeforeEach(func() {
			containers = append(containers, containerResource{cpu: 100, memory: 100})
			spDescription, err = peeringInfo.getOrCreateShadowPodDescription(ctx, spValidator.client,
				forgeShadowPodWithResourceRequests(containers, nil), offloadingv1beta1.SoftLimitsEnforcement)
		})

		When("The ShadowPod Description does not exist", func() {
			BeforeEach(func() {
				delete(peeringInfo.shadowPods, testShadowPodName)
			})
			It("should return a new one without errors", func() {
				Expect(spDescription.running).To(BeTrue())
				Expect(err).To(BeNil())
			})
		})
		When("The ShadowPod Description exists in cache and is running", func() {
			BeforeEach(func() {
				spDescriptionTest = createShadowPodDescription(testShadowPodName, testNamespace, testShadowPodUID, *forgeResourceList(100, 100))
				errTest = fmt.Errorf("ShadowPod %s is already running", spDescriptionTest.namespacedName.Name)
				err = nil
				peeringInfo.shadowPods[spNamespacedName.String()] = spDescriptionTest
			})
			It("should return an error", func() {
				Expect(spDescription).To(BeNil())
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal(errTest.Error()))
			})
		})
		When("The ShadowPod Description exists in cache and is not running. The Shadow Pod exists in system snapshot", func() {
			BeforeEach(func() {
				spDescriptionTest = createShadowPodDescription(testShadowPodName, testNamespace, testShadowPodUID, *forgeResourceList(100, 100))
				spDescriptionTest.terminate()
				errTest = fmt.Errorf("ShadowPod still exists in the system")
				peeringInfo.shadowPods[spNamespacedName.String()] = spDescriptionTest
				containers = append(containers, containerResource{cpu: 100, memory: 100})
				Expect(fakeClient.Create(ctx, forgeShadowPodWithResourceRequests(containers, nil))).ToNot(HaveOccurred())
			})
			It("should return an error", func() {
				Expect(spDescription).To(BeNil())
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal(errTest.Error()))
			})
		})
		When("The ShadowPod Description exists in cache but is not running and the Shadow Pod does not exist in system snapshot anymore", func() {
			BeforeEach(func() {
				spDescriptionTest = createShadowPodDescription(testShadowPodName, testNamespace, testShadowPodUID, *forgeResourceList(100, 100))
				spDescriptionTest.terminate()
				peeringInfo.shadowPods[spNamespacedName.String()] = spDescriptionTest
			})
			It("should return a new running one without errors", func() {
				Expect(spDescription.running).To(BeTrue())
				Expect(err).To(BeNil())
			})
		})
	})

	Describe("Get a ShadowPod Description", func() {
		JustBeforeEach(func() {
			containers = append(containers, containerResource{cpu: 100, memory: 100})
			spDescription, err = peeringInfo.getShadowPodDescription(forgeShadowPodWithResourceRequests(containers, nil))
		})

		When("The ShadowPod Description exists", func() {
			BeforeEach(func() {
				spDescriptionTest = createShadowPodDescription(testShadowPodName, testNamespace, testShadowPodUID, *forgeResourceList(100, 100))
				peeringInfo.shadowPods[spNamespacedName.String()] = spDescriptionTest
			})
			It("should return it without errors", func() {
				Expect(spDescription).ToNot(BeNil())
				Expect(err).To(BeNil())
			})
		})
		When("The ShadowPod Description does not exist", func() {
			BeforeEach(func() {
				errTest = fmt.Errorf("ShadowPod %s not found (Maybe Cache problem)", testShadowPodName)
				delete(peeringInfo.shadowPods, testShadowPodName)
			})
			It("should return an error", func() {
				Expect(spDescription).To(BeNil())
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal(errTest.Error()))
			})
		})
		When("The ShadowPod Description exists but with a UID mismatch", func() {
			BeforeEach(func() {
				spDescriptionTest = createShadowPodDescription(testShadowPodName, testNamespace, testShadowPodUIDInvalid, *forgeResourceList(100, 100))
				errTest = fmt.Errorf("ShadowPod %s: UID mismatch", testShadowPodName)
				peeringInfo.shadowPods[spNamespacedName.String()] = spDescriptionTest
			})
			It("should return an error", func() {
				Expect(spDescription).To(BeNil())
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal(errTest.Error()))
			})
		})
	})

	Describe("Get Quota from a ShadowPod", func() {
		JustBeforeEach(func() {
			quota, err = getQuotaFromShadowPod(shadowPod, offloadingv1beta1.SoftLimitsEnforcement)
		})

		When("at least one ShadowPod container has not cpu or memory requests defined", func() {
			BeforeEach(func() {
				containers = append(containers, containerResource{cpu: 100})
				shadowPod = forgeShadowPodWithResourceRequests(containers, nil)
				errTest = fmt.Errorf("CPU and/or memory requests not set for container test-container")
			})
			It("should return an error", func() {
				Expect(quota).To(BeNil())
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal(errTest.Error()))
			})
		})
		When("at least one ShadowPod initContainer has not cpu or memory requests defined", func() {
			BeforeEach(func() {
				containers = nil
				containers = append(containers, containerResource{cpu: 100, memory: 100})
				initContainers = nil
				initContainers = append(containers, containerResource{cpu: 100})
				shadowPod = forgeShadowPodWithResourceRequests(containers, initContainers)
				errTest = fmt.Errorf("CPU and/or memory requests not set for initContainer test-init-container")
			})
			It("should return an error", func() {
				Expect(quota).To(BeNil())
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal(errTest.Error()))
			})
		})
		When("The ShadowPod has not any container defined", func() {
			BeforeEach(func() {
				containers = nil
				initContainers = nil
				shadowPod = forgeShadowPodWithResourceRequests(containers, initContainers)
				errTest = fmt.Errorf("ShadowPod %s has no containers defined", shadowPod.GetName())
			})
			It("should return an error", func() {
				Expect(quota).To(BeNil())
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal(errTest.Error()))
			})
		})
		When("The Shadow has some valid containers and init containers", func() {
			BeforeEach(func() {
				containers = nil
				containers = append(containers, containerResource{cpu: 100, memory: 100})
				containers = append(containers, containerResource{cpu: 200, memory: 100})
				initContainers = nil
				initContainers = append(containers, containerResource{cpu: 100, memory: 300})
				shadowPod = forgeShadowPodWithResourceRequests(containers, initContainers)
				errTest = fmt.Errorf("ShadowPod %s has no containers defined", shadowPod.GetName())
			})
			It("should return a ResourceList which is the Max between the sum of all containers resources and the Max of initContainers", func() {
				Expect(quota).To(Equal(forgeResourceList(300, 300)))
				Expect(err).To(BeNil())
			})
		})
	})

})
