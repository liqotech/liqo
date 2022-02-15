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

package namespacemap

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	vkv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	liqoclient "github.com/liqotech/liqo/pkg/client/clientset/versioned/fake"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/namespacemap/fake"
)

var _ = Describe("NamespaceMapEventHandler tests", func() {
	var (
		nmh          *Handler
		fakeManager  *fake.NamespaceStartStopper
		namespaceMap *vkv1alpha1.NamespaceMap
	)

	BeforeEach(func() {
		fakeManager = fake.NewNamespaceStartStopper()
		fakeLiqoClient := liqoclient.NewSimpleClientset()

		nmh = NewHandler(fakeLiqoClient, "ns", 0)
		nmh.Start(context.Background(), fakeManager)

		namespaceMap = &vkv1alpha1.NamespaceMap{
			Status: vkv1alpha1.NamespaceMapStatus{
				CurrentMapping: map[string]vkv1alpha1.RemoteNamespaceStatus{
					"mappingAcceptedlocalNs1": {
						RemoteNamespace: "remoteNs1",
						Phase:           vkv1alpha1.MappingAccepted,
					},
					"creationLoopBackOfflocalNs": {
						RemoteNamespace: "remoteNs2",
						Phase:           vkv1alpha1.MappingCreationLoopBackOff,
					},
					"terminating": {
						RemoteNamespace: "remoteNs3",
						Phase:           vkv1alpha1.MappingTerminating,
					},
					"mappingAcceptedlocalNs2": {
						RemoteNamespace: "remoteNs4",
						Phase:           vkv1alpha1.MappingAccepted,
					},
				},
			},
		}
	})

	Describe("NewNamespaceMapEventHandler", func() {
		It("should set the informer factory", func() {
			Expect(nmh.informerFactory).ToNot(BeNil())
		})

		It("should set the lister", func() {
			Expect(nmh.lister).ToNot(BeNil())
		})
	})

	Describe("Start", func() {
		It("should set the reflection manager", func() {
			Expect(nmh.namespaceStartStopper).ToNot(BeNil())
		})
	})

	Describe("addNamespaceMap", func() {
		BeforeEach(func() {
			nmh.onAddNamespaceMap(namespaceMap)
		})

		It("should call manager.StartNamespace for every accepted namespace mapping", func() {
			Expect(fakeManager.StartNamespaceCalled).To(BeIdenticalTo(2))
			Expect(fakeManager.StartNamespaceArgumentsCall).To(HaveKeyWithValue("mappingAcceptedlocalNs1", "remoteNs1"))
			Expect(fakeManager.StartNamespaceArgumentsCall).To(HaveKeyWithValue("mappingAcceptedlocalNs2", "remoteNs4"))
		})
	})

	Describe("deleteNamespaceMap", func() {
		BeforeEach(func() {
			nmh.onDeleteNamespaceMap(namespaceMap)
		})

		It("should call manager.StopNamespace for every deleted accepted namespace mapping", func() {
			Expect(fakeManager.StopNamespaceCalled).To(BeIdenticalTo(2))
			Expect(fakeManager.StopNamespaceArgumentsCall).To(HaveKeyWithValue("mappingAcceptedlocalNs1", "remoteNs1"))
			Expect(fakeManager.StopNamespaceArgumentsCall).To(HaveKeyWithValue("mappingAcceptedlocalNs2", "remoteNs4"))
		})
	})

	Describe("updateNamespaceMap", func() {
		var oldNamespaceMap, newNamespaceMap *vkv1alpha1.NamespaceMap

		BeforeEach(func() {
			oldNamespaceMap = &vkv1alpha1.NamespaceMap{
				Status: vkv1alpha1.NamespaceMapStatus{
					CurrentMapping: map[string]vkv1alpha1.RemoteNamespaceStatus{
						"mappingAcceptedlocalNs1": {
							RemoteNamespace: "remoteNs1",
							Phase:           vkv1alpha1.MappingAccepted,
						},
						"creationLoopBackOfflocalNs": {
							RemoteNamespace: "remoteNs2",
							Phase:           vkv1alpha1.MappingCreationLoopBackOff,
						},
						"terminating": {
							RemoteNamespace: "remoteNs3",
							Phase:           vkv1alpha1.MappingTerminating,
						},
						"oldMappingAcceptedlocalNs2": {
							RemoteNamespace: "remoteNs4",
							Phase:           vkv1alpha1.MappingAccepted,
						},
						"creationLoopBackOfflocalNsThatWillRecover": {
							RemoteNamespace: "remoteNs5",
							Phase:           vkv1alpha1.MappingCreationLoopBackOff,
						},
						"mappingAcceptedlocalNsThatWillCrash": {
							RemoteNamespace: "remoteNs6",
							Phase:           vkv1alpha1.MappingAccepted,
						},
						"mappingAcceptedlocalNsThatWillTerminate": {
							RemoteNamespace: "remoteNs7",
							Phase:           vkv1alpha1.MappingAccepted,
						},
					},
				},
			}

			newNamespaceMap = &vkv1alpha1.NamespaceMap{
				Status: vkv1alpha1.NamespaceMapStatus{
					CurrentMapping: map[string]vkv1alpha1.RemoteNamespaceStatus{
						"mappingAcceptedlocalNs1": {
							RemoteNamespace: "remoteNs1",
							Phase:           vkv1alpha1.MappingAccepted,
						},
						"creationLoopBackOfflocalNs": {
							RemoteNamespace: "remoteNs2",
							Phase:           vkv1alpha1.MappingCreationLoopBackOff,
						},
						"terminating": {
							RemoteNamespace: "remoteNs3",
							Phase:           vkv1alpha1.MappingTerminating,
						},
						"creationLoopBackOfflocalNsThatWillRecover": {
							RemoteNamespace: "remoteNs5",
							Phase:           vkv1alpha1.MappingAccepted,
						},
						"mappingAcceptedlocalNsThatWillCrash": {
							RemoteNamespace: "remoteNs6",
							Phase:           vkv1alpha1.MappingCreationLoopBackOff,
						},
						"mappingAcceptedlocalNsThatWillTerminate": {
							RemoteNamespace: "remoteNs7",
							Phase:           vkv1alpha1.MappingTerminating,
						},
						"newMappingAcceptedlocalNs2": {
							RemoteNamespace: "remoteNs8",
							Phase:           vkv1alpha1.MappingAccepted,
						},
					},
				},
			}
		})

		JustBeforeEach(func() {
			nmh.onUpdateNamespaceMap(oldNamespaceMap, newNamespaceMap)
		})

		It("should call manager.StartNamespace for every namespace that is not in the old NamespaceMap", func() {
			Expect(fakeManager.StartNamespaceArgumentsCall).To(HaveKeyWithValue("newMappingAcceptedlocalNs2", "remoteNs8"))
		})

		It("should call manager.StartNamespace for every namespace that has just transitioned to MappingAccepted phase", func() {
			Expect(fakeManager.StartNamespaceArgumentsCall).To(HaveKeyWithValue("creationLoopBackOfflocalNsThatWillRecover", "remoteNs5"))
		})

		It("should call manager.StartNamespace for every namespace to start only", func() {
			Expect(fakeManager.StartNamespaceCalled).To(BeIdenticalTo(2))
		})

		It("should call manager.StopNamespace for every namespace that is not in the new NamespaceMap", func() {
			Expect(fakeManager.StopNamespaceArgumentsCall).To(HaveKeyWithValue("oldMappingAcceptedlocalNs2", "remoteNs4"))
		})

		It("should call manager.StopNamespace for every namespace that transitioned from the MappingAccepted to the MappingTerminating phase", func() {
			Expect(fakeManager.StopNamespaceArgumentsCall).To(HaveKeyWithValue("mappingAcceptedlocalNsThatWillTerminate", "remoteNs7"))
		})

		It("should call manager.StopNamespace for every namespace to stop only", func() {
			Expect(fakeManager.StopNamespaceCalled).To(BeIdenticalTo(2))
		})

		Describe("namespaces transitioning from the MappingAccepted to the MappingCreationLoopBackOff phase", func() {
			It("should not call manager.StopNamespace", func() {
				Expect(fakeManager.StopNamespaceArgumentsCall).ToNot(HaveKeyWithValue("mappingAcceptedlocalNsThatWillCrash", "remoteNs6"))
			})

			Describe("the failing namespace transitions to a different phase", func() {
				var transitionPhase vkv1alpha1.MappingPhase

				JustBeforeEach(func() {
					oldNamespaceMap = newNamespaceMap.DeepCopy()
					newNamespaceMap.Status.CurrentMapping["mappingAcceptedlocalNsThatWillCrash"] = vkv1alpha1.RemoteNamespaceStatus{
						RemoteNamespace: "remoteNs6", Phase: transitionPhase,
					}
					nmh.onUpdateNamespaceMap(oldNamespaceMap, newNamespaceMap)
				})

				When("the phase is MappingAccepted", func() {
					BeforeEach(func() { transitionPhase = vkv1alpha1.MappingAccepted })

					It("should not call manager.StartNamespace", func() {
						Expect(fakeManager.StartNamespaceArgumentsCall).ToNot(HaveKeyWithValue("mappingAcceptedlocalNsThatWillCrash", "remoteNs6"))
					})
				})

				When("the phase is MappingTerminating", func() {
					BeforeEach(func() { transitionPhase = vkv1alpha1.MappingTerminating })

					It("should call manager.StopNamespace", func() {
						Expect(fakeManager.StopNamespaceArgumentsCall).To(HaveKeyWithValue("mappingAcceptedlocalNsThatWillCrash", "remoteNs6"))
					})
				})
			})
		})
	})
})
