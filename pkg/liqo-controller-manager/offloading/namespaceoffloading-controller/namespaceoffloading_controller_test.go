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

package nsoffctrl

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutils "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreigncluster"
)

var _ = Describe("Namespace controller", func() {
	StatusCheck := func(remoteNamespaceName string, phase offloadingv1beta1.OffloadingPhaseType,
		conditionsChecker func(string, offloadingv1beta1.RemoteNamespaceConditions) error) {
		Eventually(func() error {
			Expect(cl.Get(ctx, client.ObjectKeyFromObject(nsoff), nsoff)).To(Succeed())

			if nsoff.Status.ObservedGeneration != nsoff.Generation {
				return fmt.Errorf("ObservedGeneration and generation do not match, actual: %v, observed: %q",
					nsoff.Generation, nsoff.Status.ObservedGeneration)
			}

			if nsoff.Status.RemoteNamespaceName != remoteNamespaceName {
				return fmt.Errorf("NamespaceOffloading remote namespace name is not correct, actual: %q, expected: %q",
					nsoff.Status.RemoteNamespaceName, remoteNamespaceName)
			}

			if nsoff.Status.OffloadingPhase != phase {
				return fmt.Errorf("NamespaceOffloading phase is not correct, actual: %q, expected: %q",
					nsoff.Status.OffloadingPhase, phase)
			}

			for nmname, conditions := range nsoff.Status.RemoteNamespacesConditions {
				if err := conditionsChecker(nmname, conditions); err != nil {
					return err
				}
			}

			return nil
		}).Should(Succeed())
	}

	ConditionsReady := func(nmname string, conditions offloadingv1beta1.RemoteNamespaceConditions) error {
		if len(conditions) != 2 {
			return fmt.Errorf("NamespaceOffloading conditions for NamespaceMap %q are not correct, actual len: %d", nmname, len(conditions))
		}

		if conditions[0].Type != offloadingv1beta1.NamespaceOffloadingRequired || conditions[0].Status != corev1.ConditionTrue {
			return fmt.Errorf("NamespaceOffloading conditions for NamespaceMap %q are not correct, actual: %v", nmname, conditions)
		}

		if conditions[1].Type != offloadingv1beta1.NamespaceReady || conditions[1].Status != corev1.ConditionTrue {
			return fmt.Errorf("NamespaceOffloading conditions for NamespaceMap %q are not correct, actual: %v", nmname, conditions)
		}

		return nil
	}

	ConditionsNotReady := func(nmname string, conditions offloadingv1beta1.RemoteNamespaceConditions) error {
		if len(conditions) != 2 {
			return fmt.Errorf("NamespaceOffloading conditions for NamespaceMap %q are not correct, actual len: %d", nmname, len(conditions))
		}

		if conditions[0].Type != offloadingv1beta1.NamespaceOffloadingRequired || conditions[0].Status != corev1.ConditionTrue {
			return fmt.Errorf("NamespaceOffloading conditions for NamespaceMap %q are not correct, actual: %v", nmname, conditions)
		}

		if conditions[1].Type != offloadingv1beta1.NamespaceReady || conditions[1].Status != corev1.ConditionFalse {
			return fmt.Errorf("NamespaceOffloading conditions for NamespaceMap %q are not correct, actual: %v", nmname, conditions)
		}

		return nil
	}

	ConditionsNotSelected := func(nmname string, conditions offloadingv1beta1.RemoteNamespaceConditions) error {
		if len(conditions) != 1 {
			return fmt.Errorf("NamespaceOffloading conditions for NamespaceMap %q are not correct, actual len: %d", nmname, len(conditions))
		}

		if conditions[0].Type != offloadingv1beta1.NamespaceOffloadingRequired || conditions[0].Status != corev1.ConditionFalse {
			return fmt.Errorf("NamespaceOffloading conditions for NamespaceMap %q are not correct, actual: %v", nmname, conditions)
		}

		return nil
	}

	BeforeEach(func() {
		nsoff = &offloadingv1beta1.NamespaceOffloading{}
		nm1.Status = offloadingv1beta1.NamespaceMapStatus{}
		nm2.Status = offloadingv1beta1.NamespaceMapStatus{}
		nm3.Status = offloadingv1beta1.NamespaceMapStatus{}

		// Delete NamespaceOffloading resources
		Expect(client.IgnoreNotFound(cl.DeleteAllOf(ctx, &offloadingv1beta1.NamespaceOffloading{}, client.InNamespace(namespaceName)))).Should(Succeed())

		// Clean NamespaceMaps
		var nms offloadingv1beta1.NamespaceMapList
		Eventually(func() error {
			Expect(cl.List(ctx, &nms)).To(Succeed())
			Expect(nms.Items).To(HaveLen(mapNumber))

			for i := range nms.Items {
				nms.Items[i].Spec.DesiredMapping = nil
				if err := cl.Update(ctx, &nms.Items[i]); err != nil {
					return err
				}

				nms.Items[i].Status = offloadingv1beta1.NamespaceMapStatus{}
				if err := cl.Status().Update(ctx, &nms.Items[i]); err != nil {
					return err
				}
			}

			return nil
		}).Should(Succeed())

		// Check that they are cleaned
		Eventually(func() error {
			Expect(cl.List(ctx, &nms)).To(Succeed())
			Expect(nms.Items).To(HaveLen(mapNumber))

			for i := range nms.Items {
				if nms.Items[i].Spec.DesiredMapping != nil || nms.Items[i].Status.CurrentMapping != nil {
					return fmt.Errorf("NamespaceMap %s is not cleaned", nms.Items[i].Name)
				}
			}
			return nil
		}).Should(Succeed())

		// Check that the NamespaceOffloading has been deleted
		Eventually(func() ([]offloadingv1beta1.NamespaceOffloading, error) {
			var nsoffs offloadingv1beta1.NamespaceOffloadingList
			err := cl.List(ctx, &nsoffs)
			return nsoffs.Items, err
		}).Should(HaveLen(0))
	})

	Context("Create a NamespaceOffloading resource with an empty clusterSelector", func() {
		var (
			nm                  offloadingv1beta1.NamespaceMap
			remoteNamespaceName string
		)

		BeforeEach(func() {
			remoteNamespaceName = fmt.Sprintf("%s-%s", namespaceName, foreignclusterutils.UniqueName(localCluster))
			nsoff = &offloadingv1beta1.NamespaceOffloading{
				ObjectMeta: metav1.ObjectMeta{Name: liqoconst.DefaultNamespaceOffloadingName, Namespace: namespaceName},
				Spec: offloadingv1beta1.NamespaceOffloadingSpec{
					NamespaceMappingStrategy: offloadingv1beta1.DefaultNameMappingStrategyType,
					PodOffloadingStrategy:    offloadingv1beta1.LocalAndRemotePodOffloadingStrategyType,
					ClusterSelector:          corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{}},
				},
			}

			By(fmt.Sprintf("Create NamespaceOffloading resource in Namespace %q", namespaceName))
			Expect(cl.Create(ctx, nsoff)).To(Succeed())
		})

		It("NamespaceMaps of virtual nodes should be updated", func() {
			for _, obj := range []*offloadingv1beta1.NamespaceMap{nm1, nm2, nm3} {
				Eventually(func() map[string]string {
					Expect(cl.Get(ctx, client.ObjectKeyFromObject(obj), &nm)).To(Succeed())
					return nm.Spec.DesiredMapping
				}).Should(HaveKeyWithValue(namespaceName, remoteNamespaceName))
			}
		})

		It("A finalizer shall be added to the NamespaceOffloading", func() {
			Eventually(func() bool {
				Expect(cl.Get(ctx, client.ObjectKeyFromObject(nsoff), nsoff)).To(Succeed())
				return ctrlutils.ContainsFinalizer(nsoff, namespaceOffloadingControllerFinalizer)
			}).Should(BeTrue())
		})

		It("The scheduling label shall be added to the namespace", func() {
			Eventually(func() map[string]string {
				Expect(cl.Get(ctx, client.ObjectKeyFromObject(namespace), namespace)).To(Succeed())
				return namespace.Labels
			}).Should(HaveKeyWithValue(liqoconst.SchedulingLiqoLabel, liqoconst.SchedulingLiqoLabelValue))
		})

		Context("Status propagation checks", func() {
			ForgeCurrentMapping := func(phase offloadingv1beta1.MappingPhase) map[string]offloadingv1beta1.RemoteNamespaceStatus {
				return map[string]offloadingv1beta1.RemoteNamespaceStatus{
					namespaceName: {RemoteNamespace: remoteNamespaceName, Phase: phase}}
			}

			JustBeforeEach(func() {
				for _, obj := range []*offloadingv1beta1.NamespaceMap{nm1, nm2, nm3} {
					status := obj.Status.DeepCopy()
					Eventually(func() error {
						Expect(cl.Get(ctx, client.ObjectKeyFromObject(obj), &nm)).To(Succeed())
						nm.Status = *status
						return cl.Status().Update(ctx, &nm)
					}).Should(Succeed())
				}
			})

			When("All remote namespaces have been correctly created", func() {
				BeforeEach(func() {
					nm1.Status.CurrentMapping = ForgeCurrentMapping(offloadingv1beta1.MappingAccepted)
					nm2.Status.CurrentMapping = ForgeCurrentMapping(offloadingv1beta1.MappingAccepted)
					nm3.Status.CurrentMapping = ForgeCurrentMapping(offloadingv1beta1.MappingAccepted)
				})

				It("Should converge to the ready status", func() {
					StatusCheck(remoteNamespaceName, offloadingv1beta1.ReadyOffloadingPhaseType, ConditionsReady)
				})
			})

			When("Some remote namespace creations are still in progress", func() {
				BeforeEach(func() {
					nm1.Status.CurrentMapping = ForgeCurrentMapping(offloadingv1beta1.MappingAccepted)
					nm3.Status.CurrentMapping = ForgeCurrentMapping(offloadingv1beta1.MappingAccepted)
				})

				It("Should converge to the in progress status", func() {
					StatusCheck(remoteNamespaceName, offloadingv1beta1.InProgressOffloadingPhaseType,
						func(nmname string, conditions offloadingv1beta1.RemoteNamespaceConditions) error {
							if nmname == nm2.Name {
								return ConditionsNotReady(nmname, conditions)
							}
							return ConditionsReady(nmname, conditions)
						})
				})
			})

			When("All remote namespace creations have failed", func() {
				BeforeEach(func() {
					nm1.Status.CurrentMapping = ForgeCurrentMapping(offloadingv1beta1.MappingCreationLoopBackOff)
					nm2.Status.CurrentMapping = ForgeCurrentMapping(offloadingv1beta1.MappingCreationLoopBackOff)
					nm3.Status.CurrentMapping = ForgeCurrentMapping(offloadingv1beta1.MappingCreationLoopBackOff)
				})

				It("Should converge to the all failed status", func() {
					StatusCheck(remoteNamespaceName, offloadingv1beta1.AllFailedOffloadingPhaseType, ConditionsNotReady)
				})
			})

			When("Some remote namespace creations have failed", func() {
				BeforeEach(func() {
					nm1.Status.CurrentMapping = ForgeCurrentMapping(offloadingv1beta1.MappingAccepted)
					nm2.Status.CurrentMapping = ForgeCurrentMapping(offloadingv1beta1.MappingCreationLoopBackOff)
					nm3.Status.CurrentMapping = ForgeCurrentMapping(offloadingv1beta1.MappingAccepted)
				})

				It("Should converge to the some failed status", func() {
					StatusCheck(remoteNamespaceName, offloadingv1beta1.SomeFailedOffloadingPhaseType,
						func(nmname string, conditions offloadingv1beta1.RemoteNamespaceConditions) error {
							if nmname == nm2.Name {
								return ConditionsNotReady(nmname, conditions)
							}
							return ConditionsReady(nmname, conditions)
						})
				})
			})
		})
	})

	It("Create a NamespaceOffloading with a valid cluster selector", func() {
		var nm offloadingv1beta1.NamespaceMap
		nsoff = &offloadingv1beta1.NamespaceOffloading{
			ObjectMeta: metav1.ObjectMeta{Name: liqoconst.DefaultNamespaceOffloadingName, Namespace: namespaceName},
			Spec: offloadingv1beta1.NamespaceOffloadingSpec{
				NamespaceMappingStrategy: offloadingv1beta1.EnforceSameNameMappingStrategyType,
				PodOffloadingStrategy:    offloadingv1beta1.LocalAndRemotePodOffloadingStrategyType,
				ClusterSelector: corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{
					MatchExpressions: []corev1.NodeSelectorRequirement{{
						Key:      liqoconst.TopologyRegionClusterLabel,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{regionA},
					}},
				}}},
			},
		}

		By(fmt.Sprintf("Create NamespaceOffloading resource in Namespace %q", namespaceName))
		Expect(cl.Create(ctx, nsoff)).To(Succeed())

		By("Check NamespaceMap of virtual nodes 1")
		Eventually(func() map[string]string {
			Expect(cl.Get(ctx, client.ObjectKeyFromObject(nm1), &nm)).To(Succeed())
			return nm.Spec.DesiredMapping
		}).Should(HaveKeyWithValue(namespaceName, namespaceName))

		By("Check NamespaceMap of virtual nodes 2")
		Eventually(func() map[string]string {
			Expect(cl.Get(ctx, client.ObjectKeyFromObject(nm2), &nm)).To(Succeed())
			return nm.Spec.DesiredMapping
		}).ShouldNot(HaveKey(namespaceName))

		By("Check NamespaceMap of virtual node 3")
		Eventually(func() map[string]string {
			Expect(cl.Get(ctx, client.ObjectKeyFromObject(nm3), &nm)).To(Succeed())
			return nm.Spec.DesiredMapping
		}).Should(HaveKeyWithValue(namespaceName, namespaceName))

		By("Check presence of the finalizer on the NamespaceOffloading")
		Eventually(func() bool {
			Expect(cl.Get(ctx, client.ObjectKeyFromObject(nsoff), nsoff)).To(Succeed())
			return ctrlutils.ContainsFinalizer(nsoff, namespaceOffloadingControllerFinalizer)
		}).Should(BeTrue())

		By(fmt.Sprintf("Check presence of the scheduling label on the namespace %q", namespaceName))
		Eventually(func() map[string]string {
			Expect(cl.Get(ctx, client.ObjectKeyFromObject(namespace), namespace)).To(Succeed())
			return namespace.Labels
		}).Should(HaveKeyWithValue(liqoconst.SchedulingLiqoLabel, liqoconst.SchedulingLiqoLabelValue))

		By("Fill the NamespaceMap status")
		for _, obj := range []*offloadingv1beta1.NamespaceMap{nm1, nm3} {
			Eventually(func() error {
				Expect(cl.Get(ctx, client.ObjectKeyFromObject(obj), &nm)).To(Succeed())
				nm.Status.CurrentMapping = map[string]offloadingv1beta1.RemoteNamespaceStatus{
					namespaceName: {RemoteNamespace: namespaceName, Phase: offloadingv1beta1.MappingAccepted}}
				return cl.Status().Update(ctx, &nm)
			}).Should(Succeed())
		}

		By("Check that the NamespaceOffloading status is correct")
		StatusCheck(namespaceName, offloadingv1beta1.ReadyOffloadingPhaseType,
			func(nmname string, conditions offloadingv1beta1.RemoteNamespaceConditions) error {
				if nmname == nm2.Name {
					return ConditionsNotSelected(nmname, conditions)
				}
				return ConditionsReady(nmname, conditions)
			})

		By("Delete NamespaceOffloading resource")
		Expect(cl.Delete(ctx, nsoff)).To(Succeed())

		By("Check if there are no DesiredMapping")
		for _, obj := range []*offloadingv1beta1.NamespaceMap{nm1, nm2, nm3} {
			Eventually(func() map[string]string {
				Expect(cl.Get(ctx, client.ObjectKeyFromObject(obj), &nm)).To(Succeed())
				return nm.Spec.DesiredMapping
			}).Should(BeEmpty())
		}

		By("Check that scheduling Label is removed from Namespace")
		Eventually(func() map[string]string {
			Expect(cl.Get(ctx, client.ObjectKeyFromObject(namespace), namespace)).To(Succeed())
			return namespace.Labels
		}).ShouldNot(HaveKey(liqoconst.SchedulingLiqoLabel))

	})

	It("Create a NamespaceOffloading resource with a wrong clusterSelector", func() {
		var nm offloadingv1beta1.NamespaceMap
		nsoff = &offloadingv1beta1.NamespaceOffloading{
			ObjectMeta: metav1.ObjectMeta{Name: liqoconst.DefaultNamespaceOffloadingName, Namespace: namespaceName},
			Spec: offloadingv1beta1.NamespaceOffloadingSpec{
				NamespaceMappingStrategy: offloadingv1beta1.EnforceSameNameMappingStrategyType,
				PodOffloadingStrategy:    offloadingv1beta1.LocalAndRemotePodOffloadingStrategyType,
				ClusterSelector: corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{
					MatchExpressions: []corev1.NodeSelectorRequirement{{
						Key:      liqoconst.TopologyRegionClusterLabel,
						Operator: corev1.NodeSelectorOpExists, // OpExists requires that no value is specified
						Values:   []string{regionA},
					}},
				}}},
			},
		}

		By(fmt.Sprintf("Create NamespaceOffloading resource in Namespace %q", namespaceName))
		Expect(cl.Create(ctx, nsoff)).To(Succeed())

		By("Check presence of the event for the user")
		Eventually(func() []corev1.Event {
			var el corev1.EventList
			Expect(cl.List(ctx, &el)).To(Succeed())
			return el.Items
		}).Should(HaveLen(1))

		By(fmt.Sprintf("Check absence of scheduling label on the namespace %s", namespaceName))
		Consistently(func() map[string]string {
			Expect(cl.Get(ctx, client.ObjectKeyFromObject(namespace), namespace)).To(Succeed())
			return namespace.Labels
		}).ShouldNot(HaveKey(liqoconst.SchedulingLiqoLabel))

		By("Check presence of the finalizer on the NamespaceOffloading")
		Eventually(func() bool {
			Expect(cl.Get(ctx, client.ObjectKeyFromObject(nsoff), nsoff)).To(Succeed())
			return ctrlutils.ContainsFinalizer(nsoff, namespaceOffloadingControllerFinalizer)
		}).Should(BeTrue())

		By("Check NamespaceMaps to be empty")
		for _, obj := range []*offloadingv1beta1.NamespaceMap{nm1, nm2, nm3} {
			Consistently(func() map[string]string {
				Expect(cl.Get(ctx, client.ObjectKeyFromObject(obj), &nm)).To(Succeed())
				return nm.Spec.DesiredMapping
			}).Should(BeEmpty())
		}

		By("Check that the NamespaceOffloading status is correct")
		StatusCheck(namespaceName, offloadingv1beta1.NoClusterSelectedOffloadingPhaseType, ConditionsNotSelected)
	})

	It("Create a NamespaceOffloading resource that doesn't select any cluster", func() {
		var nm offloadingv1beta1.NamespaceMap
		nsoff = &offloadingv1beta1.NamespaceOffloading{
			ObjectMeta: metav1.ObjectMeta{Name: liqoconst.DefaultNamespaceOffloadingName, Namespace: namespaceName},
			Spec: offloadingv1beta1.NamespaceOffloadingSpec{
				NamespaceMappingStrategy: offloadingv1beta1.EnforceSameNameMappingStrategyType,
				PodOffloadingStrategy:    offloadingv1beta1.LocalAndRemotePodOffloadingStrategyType,
				ClusterSelector: corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{
					MatchExpressions: []corev1.NodeSelectorRequirement{{
						Key:      liqoconst.TopologyRegionClusterLabel,
						Operator: corev1.NodeSelectorOpNotIn,
						Values:   []string{regionA, regionB},
					}},
				}}},
			},
		}

		By(fmt.Sprintf("Create NamespaceOffloading resource in Namespace %q", namespaceName))
		Expect(cl.Create(ctx, nsoff)).To(Succeed())

		By(fmt.Sprintf("Check presence of the scheduling label on the namespace %q", namespaceName))
		Eventually(func() map[string]string {
			Expect(cl.Get(ctx, client.ObjectKeyFromObject(namespace), namespace)).To(Succeed())
			return namespace.Labels
		}).Should(HaveKeyWithValue(liqoconst.SchedulingLiqoLabel, liqoconst.SchedulingLiqoLabelValue))

		By("Check presence of the finalizer on the NamespaceOffloading")
		Eventually(func() bool {
			Expect(cl.Get(ctx, client.ObjectKeyFromObject(nsoff), nsoff)).To(Succeed())
			return ctrlutils.ContainsFinalizer(nsoff, namespaceOffloadingControllerFinalizer)
		}).Should(BeTrue())

		By("Check NamespaceMaps to be empty")
		for _, obj := range []*offloadingv1beta1.NamespaceMap{nm1, nm2, nm3} {
			Consistently(func() map[string]string {
				Expect(cl.Get(ctx, client.ObjectKeyFromObject(obj), &nm)).To(Succeed())
				return nm.Spec.DesiredMapping
			}).Should(BeEmpty())
		}
	})
})
