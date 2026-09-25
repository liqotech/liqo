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

package namespacemap

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	liqoclient "github.com/liqotech/liqo/pkg/client/clientset/versioned/fake"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/namespacemap/fake"
)

var _ = Describe("NamespaceMapEventHandler tests", func() {
	var (
		nmh            *Handler
		fakeManager    *fake.NamespaceStartStopper
		fakeLiqoClient *liqoclient.Clientset
		namespaceMap   *offloadingv1beta1.NamespaceMap
	)

	BeforeEach(func() {
		fakeManager = fake.NewNamespaceStartStopper()
		fakeLiqoClient = liqoclient.NewSimpleClientset() //nolint:staticcheck // NewClientset is not generated in this repo (requires --with-applyconfig).

		nmh = NewHandler(fakeLiqoClient, "ns", 0)
		Expect(nmh.Start(context.Background(), fakeManager)).ToNot(HaveOccurred())

		namespaceMap = &offloadingv1beta1.NamespaceMap{
			Status: offloadingv1beta1.NamespaceMapStatus{
				CurrentMapping: map[string]offloadingv1beta1.RemoteNamespaceStatus{
					"mappingAcceptedlocalNs1": {
						RemoteNamespace: "remoteNs1",
						Phase:           offloadingv1beta1.MappingAccepted,
					},
					"creationLoopBackOfflocalNs": {
						RemoteNamespace: "remoteNs2",
						Phase:           offloadingv1beta1.MappingCreationLoopBackOff,
					},
					"terminating": {
						RemoteNamespace: "remoteNs3",
						Phase:           offloadingv1beta1.MappingTerminating,
					},
					"mappingAcceptedlocalNs2": {
						RemoteNamespace: "remoteNs4",
						Phase:           offloadingv1beta1.MappingAccepted,
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

	Describe("IsNamespaceMapped", func() {
		createNamespaceMap := func(remoteClusterID string, mapping map[string]offloadingv1beta1.RemoteNamespaceStatus) {
			nm := &offloadingv1beta1.NamespaceMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nsmap",
					Namespace: "ns",
					Labels: map[string]string{
						liqoconst.RemoteClusterID:             remoteClusterID,
						liqoconst.ReplicationDestinationLabel: remoteClusterID,
					},
				},
				Status: offloadingv1beta1.NamespaceMapStatus{CurrentMapping: mapping},
			}
			_, err := fakeLiqoClient.OffloadingV1beta1().NamespaceMaps("ns").Create(context.Background(), nm, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
		}

		When("no NamespaceMap is present", func() {
			It("should return an error, as the state is uncertain", func() {
				mapped, err := nmh.IsNamespaceMapped("localNs1")
				Expect(err).To(HaveOccurred())
				Expect(mapped).To(BeFalse())
			})
		})

		When("the only NamespaceMap belongs to a different peering", func() {
			BeforeEach(func() {
				createNamespaceMap("other-cluster", map[string]offloadingv1beta1.RemoteNamespaceStatus{
					"localNs1": {RemoteNamespace: "remoteNs1", Phase: offloadingv1beta1.MappingAccepted},
				})
			})

			It("should return an error, as the state is uncertain", func() {
				Consistently(func() error {
					_, err := nmh.IsNamespaceMapped("localNs1")
					return err
				}).Should(HaveOccurred())
			})
		})

		When("a NamespaceMap is present", func() {
			BeforeEach(func() {
				createNamespaceMap(string(forge.RemoteCluster), map[string]offloadingv1beta1.RemoteNamespaceStatus{
					"localNs1": {RemoteNamespace: "remoteNs1", Phase: offloadingv1beta1.MappingAccepted},
					"localNs2": {RemoteNamespace: "remoteNs2", Phase: offloadingv1beta1.MappingCreationLoopBackOff},
				})
				// Wait for the event to be propagated to the informer cache, so that assertions
				// are deterministic.
				Eventually(func() bool {
					mapped, err := nmh.IsNamespaceMapped("localNs1")
					return err == nil && mapped
				}).Should(BeTrue())
			})

			It("should return true for accepted mappings", func() {
				mapped, err := nmh.IsNamespaceMapped("localNs1")
				Expect(err).ToNot(HaveOccurred())
				Expect(mapped).To(BeTrue())
			})
			It("should return false for non-accepted mappings", func() {
				mapped, err := nmh.IsNamespaceMapped("localNs2")
				Expect(err).ToNot(HaveOccurred())
				Expect(mapped).To(BeFalse())
			})
			It("should return false for not-present mappings", func() {
				mapped, err := nmh.IsNamespaceMapped("localNs3")
				Expect(err).ToNot(HaveOccurred())
				Expect(mapped).To(BeFalse())
			})
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
		BeforeEach(func() {
			oldNamespaceMap := &offloadingv1beta1.NamespaceMap{
				Status: offloadingv1beta1.NamespaceMapStatus{
					CurrentMapping: map[string]offloadingv1beta1.RemoteNamespaceStatus{
						"mappingAcceptedlocalNs1": {
							RemoteNamespace: "remoteNs1",
							Phase:           offloadingv1beta1.MappingAccepted,
						},
						"creationLoopBackOfflocalNs": {
							RemoteNamespace: "remoteNs2",
							Phase:           offloadingv1beta1.MappingCreationLoopBackOff,
						},
						"terminating": {
							RemoteNamespace: "remoteNs3",
							Phase:           offloadingv1beta1.MappingTerminating,
						},
						"oldMappingAcceptedlocalNs2": {
							RemoteNamespace: "remoteNs4",
							Phase:           offloadingv1beta1.MappingAccepted,
						},
						"creationLoopBackOfflocalNsThatWillRecover": {
							RemoteNamespace: "remoteNs5",
							Phase:           offloadingv1beta1.MappingCreationLoopBackOff,
						},
						"mappingAcceptedlocalNsThatWillCrash": {
							RemoteNamespace: "remoteNs6",
							Phase:           offloadingv1beta1.MappingAccepted,
						},
					},
				},
			}
			newNamespaceMap := &offloadingv1beta1.NamespaceMap{
				Status: offloadingv1beta1.NamespaceMapStatus{
					CurrentMapping: map[string]offloadingv1beta1.RemoteNamespaceStatus{
						"mappingAcceptedlocalNs1": {
							RemoteNamespace: "remoteNs1",
							Phase:           offloadingv1beta1.MappingAccepted,
						},
						"creationLoopBackOfflocalNs": {
							RemoteNamespace: "remoteNs2",
							Phase:           offloadingv1beta1.MappingCreationLoopBackOff,
						},
						"terminating": {
							RemoteNamespace: "remoteNs3",
							Phase:           offloadingv1beta1.MappingTerminating,
						},
						"creationLoopBackOfflocalNsThatWillRecover": {
							RemoteNamespace: "remoteNs5",
							Phase:           offloadingv1beta1.MappingAccepted,
						},
						"mappingAcceptedlocalNsThatWillCrash": {
							RemoteNamespace: "remoteNs6",
							Phase:           offloadingv1beta1.MappingCreationLoopBackOff,
						},
						"newMappingAcceptedlocalNs2": {
							RemoteNamespace: "remoteNs7",
							Phase:           offloadingv1beta1.MappingAccepted,
						},
					},
				},
			}
			nmh.onUpdateNamespaceMap(oldNamespaceMap, newNamespaceMap)
		})

		It("should call manager.StartNamespace for every namespace that is not in the old NamespaceMap", func() {
			Expect(fakeManager.StartNamespaceArgumentsCall).To(HaveKeyWithValue("newMappingAcceptedlocalNs2", "remoteNs7"))
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

		It("should call manager.StopNamespace for every namespace that has just transitioned away from MappingAccepted phase", func() {
			Expect(fakeManager.StopNamespaceArgumentsCall).To(HaveKeyWithValue("mappingAcceptedlocalNsThatWillCrash", "remoteNs6"))
		})

		It("should call manager.StopNamespace for every namespace to stop only", func() {
			Expect(fakeManager.StopNamespaceCalled).To(BeIdenticalTo(2))
		})
	})
})
