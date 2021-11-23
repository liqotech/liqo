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

package namespaceoffloadingctrl

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutils "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	offv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreignCluster"
)

var _ = Describe("Namespace controller", func() {

	const (
		timeout  = time.Second * 20
		interval = time.Millisecond * 500
	)

	Context("Create NamespaceOffloading and check NamespaceMap DesiredMapping", func() {

		BeforeEach(func() {

			// Set NamespaceOffloading at initial status
			By(" 0 - BEFORE_EACH -> Clean NamespaceMap DesiredMapping")

			// 0.1 - Clean namespaceMaps DesiredMapping
			Eventually(func() bool {
				if err := homeClient.List(context.TODO(), nms); err != nil {
					return false
				}
				Expect(len(nms.Items) == mapNumber).To(BeTrue())
				for i := range nms.Items {
					nms.Items[i].Spec.DesiredMapping = nil
					if err := homeClient.Update(context.TODO(), nms.Items[i].DeepCopy()); err != nil {
						return false
					}
				}
				return true
			}, timeout, interval).Should(BeTrue())

			// 0.2 - Check that they are cleaned
			Eventually(func() bool {
				if err := homeClient.List(context.TODO(), nms); err != nil {
					return false
				}
				Expect(len(nms.Items) == mapNumber).To(BeTrue())
				for i := range nms.Items {
					if nms.Items[i].Spec.DesiredMapping != nil {
						return false
					}
				}
				return true
			}, timeout, interval).Should(BeTrue())

		})

		It(" TEST 1: Create a NamespaceOffloading resource and check desiredMapping of NamespaceMaps", func() {

			namespace1Name := "namespace1"
			namespace1 := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace1Name,
				},
			}

			namespaceOffloading1 := &offv1alpha1.NamespaceOffloading{
				ObjectMeta: metav1.ObjectMeta{
					Name:      liqoconst.DefaultNamespaceOffloadingName,
					Namespace: namespace1Name,
				},
				Spec: offv1alpha1.NamespaceOffloadingSpec{
					NamespaceMappingStrategy: offv1alpha1.EnforceSameNameMappingStrategyType,
					PodOffloadingStrategy:    offv1alpha1.LocalAndRemotePodOffloadingStrategyType,
					ClusterSelector: corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{
						MatchExpressions: []corev1.NodeSelectorRequirement{{
							Key:      regionLabel,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{regionA},
						}},
						MatchFields: nil,
					},
					},
					},
				},
			}

			By(fmt.Sprintf(" 1 - Create NamespaceOffloading resource in Namespace '%s'", namespace1Name))
			Expect(homeClient.Create(context.TODO(), namespace1)).To(Succeed())
			Eventually(func() bool {
				err := homeClient.Create(context.TODO(), namespaceOffloading1)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By(" 2 - Check NamespaceMap of virtual nodes 1 ")
			Eventually(func() bool {
				Expect(homeClient.List(context.TODO(), nms, client.MatchingLabels{liqoconst.RemoteClusterID: remoteCluster1.ClusterID})).To(Succeed())
				Expect(len(nms.Items) == 1).To(BeTrue())
				return nms.Items[0].Spec.DesiredMapping[namespace1Name] == namespace1Name
			}, timeout, interval).Should(BeTrue())

			By(" 2 - Check NamespaceMap of virtual nodes 2 ")
			Eventually(func() bool {
				Expect(homeClient.List(context.TODO(), nms, client.MatchingLabels{liqoconst.RemoteClusterID: remoteCluster2.ClusterID})).To(Succeed())
				Expect(len(nms.Items) == 1).To(BeTrue())
				_, ok := nms.Items[0].Spec.DesiredMapping[namespace1Name]
				return !ok
			}, timeout, interval).Should(BeTrue())

			By(" 3 - Check NamespaceMap of virtual node 3 ")
			Eventually(func() bool {
				Expect(homeClient.List(context.TODO(), nms, client.MatchingLabels{liqoconst.RemoteClusterID: remoteCluster3.ClusterID})).To(Succeed())
				Expect(len(nms.Items) == 1).To(BeTrue())
				return nms.Items[0].Spec.DesiredMapping[namespace1Name] == namespace1Name
			}, timeout, interval).Should(BeTrue())

			By(" 4 - Check presence of the finalizer on the NamespaceOffloading")
			Eventually(func() bool {
				if err := homeClient.Get(context.TODO(), types.NamespacedName{
					Name:      liqoconst.DefaultNamespaceOffloadingName,
					Namespace: namespace1Name}, namespaceOffloading1); err != nil {
					return false
				}
				return ctrlutils.ContainsFinalizer(namespaceOffloading1, namespaceOffloadingControllerFinalizer)
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf(" 5 - Check scheduling label on the namespace %s", namespace1Name))
			Eventually(func() bool {
				if err := homeClient.Get(context.TODO(), types.NamespacedName{Name: namespace1Name}, namespace1); err != nil {
					return false
				}
				return namespace1.Labels[liqoconst.SchedulingLiqoLabel] == liqoconst.SchedulingLiqoLabelValue
			}, timeout, interval).Should(BeTrue())

			By(" 6 - Delete NamespaceOffloading resource")
			Eventually(func() bool {
				if err := homeClient.Get(context.TODO(), types.NamespacedName{
					Name:      liqoconst.DefaultNamespaceOffloadingName,
					Namespace: namespace1Name}, namespaceOffloading1); err != nil {
					return false
				}
				err := homeClient.Delete(context.TODO(), namespaceOffloading1)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By(" 7 - Check if there are no DesiredMapping")
			Eventually(func() bool {
				if err := homeClient.List(context.TODO(), nms); err != nil {
					return false
				}
				Expect(len(nms.Items) == mapNumber).To(BeTrue())
				return nms.Items[0].Spec.DesiredMapping == nil && nms.Items[1].Spec.DesiredMapping == nil &&
					nms.Items[2].Spec.DesiredMapping == nil
			}, timeout, interval).Should(BeTrue())

			By(" 8 - Check that scheduling Label is removed from Namespace")
			Eventually(func() bool {
				if err := homeClient.Get(context.TODO(), types.NamespacedName{Name: namespace1Name}, namespace1); err != nil {
					return false
				}
				return namespace1.Labels == nil
			}, timeout, interval).Should(BeTrue())

		})

		It(" TEST 2: Create a NamespaceOffloading resource with a wrong clusterSelector", func() {

			namespace2Name := "namespace2"
			namespace2 := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace2Name,
				},
			}

			namespaceOffloading2 := &offv1alpha1.NamespaceOffloading{
				ObjectMeta: metav1.ObjectMeta{
					Name:      liqoconst.DefaultNamespaceOffloadingName,
					Namespace: namespace2Name,
				},
				Spec: offv1alpha1.NamespaceOffloadingSpec{
					NamespaceMappingStrategy: offv1alpha1.EnforceSameNameMappingStrategyType,
					PodOffloadingStrategy:    offv1alpha1.LocalAndRemotePodOffloadingStrategyType,
					ClusterSelector: corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{
						MatchExpressions: []corev1.NodeSelectorRequirement{{
							Key:      regionLabel,
							Operator: corev1.NodeSelectorOpExists,
							Values:   []string{regionA},
						}},
						MatchFields: nil,
					},
					},
					},
				},
			}

			By(fmt.Sprintf(" 1 - Create NamespaceOffloading resource in Namespace '%s'", namespace2Name))
			Expect(homeClient.Create(context.TODO(), namespace2)).To(Succeed())
			Eventually(func() bool {
				err := homeClient.Create(context.TODO(), namespaceOffloading2)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By(" 2 - Check presence of annotation for the user, on NamespaceOffloading")
			Eventually(func() bool {
				if err := homeClient.Get(context.TODO(), types.NamespacedName{
					Name:      liqoconst.DefaultNamespaceOffloadingName,
					Namespace: namespace2Name}, namespaceOffloading2); err != nil {
					return false
				}
				value, ok := namespaceOffloading2.Annotations[liqoconst.SchedulingLiqoLabel]
				return ok && value != ""
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf(" 3 - Check absence of scheduling label on the namespace %s", namespace2Name))
			Consistently(func() bool {
				if err := homeClient.Get(context.TODO(), types.NamespacedName{Name: namespace2Name}, namespace2); err != nil {
					return false
				}
				return namespace2.Labels == nil
			}, timeout/5, interval).Should(BeTrue())

			By(" 4 - Check presence of the finalizer on the NamespaceOffloading")
			Eventually(func() bool {
				if err := homeClient.Get(context.TODO(), types.NamespacedName{
					Name:      liqoconst.DefaultNamespaceOffloadingName,
					Namespace: namespace2Name}, namespaceOffloading2); err != nil {
					return false
				}
				return ctrlutils.ContainsFinalizer(namespaceOffloading2, namespaceOffloadingControllerFinalizer)
			}, timeout, interval).Should(BeTrue())

			By(" 5 - Check NamespaceMaps to be empty ")
			Consistently(func() bool {
				if err := homeClient.List(context.TODO(), nms); err != nil {
					return false
				}
				Expect(len(nms.Items) == mapNumber).To(BeTrue())
				return nms.Items[0].Spec.DesiredMapping == nil && nms.Items[1].Spec.DesiredMapping == nil &&
					nms.Items[2].Spec.DesiredMapping == nil
			}, timeout/10, interval).Should(BeTrue())

			By(" 6 - Add a remote condition in order to deny deletion of the NamespaceOffloading")
			Eventually(func() bool {
				if err := homeClient.Get(context.TODO(), types.NamespacedName{
					Name:      liqoconst.DefaultNamespaceOffloadingName,
					Namespace: namespace2Name}, namespaceOffloading2); err != nil {
					return false
				}
				namespaceOffloading2.Status.RemoteNamespacesConditions = map[string]offv1alpha1.RemoteNamespaceConditions{}
				namespaceOffloading2.Status.RemoteNamespacesConditions[remoteCluster1.ClusterID] =
					append(namespaceOffloading2.Status.RemoteNamespacesConditions[remoteCluster1.ClusterID], offv1alpha1.RemoteNamespaceCondition{
						Type:               offv1alpha1.NamespaceReady,
						Status:             corev1.ConditionFalse,
						LastTransitionTime: metav1.Now(),
						Reason:             "test",
						Message:            "test",
					})
				err := homeClient.Update(context.TODO(), namespaceOffloading2)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf(" 7 - Delete NamespaceOffloading in the namespace %s", namespace2Name))
			Eventually(func() bool {
				if err := homeClient.Get(context.TODO(), types.NamespacedName{
					Name:      liqoconst.DefaultNamespaceOffloadingName,
					Namespace: namespace2Name}, namespaceOffloading2); err != nil {
					return false
				}
				err := homeClient.Delete(context.TODO(), namespaceOffloading2)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By(" 8 - Check if the NamespaceOffloading resource is still there ")
			Eventually(func() bool {
				if err := homeClient.Get(context.TODO(), types.NamespacedName{
					Name:      liqoconst.DefaultNamespaceOffloadingName,
					Namespace: namespace2Name}, namespaceOffloading2); err != nil {
					return false
				}
				return ctrlutils.ContainsFinalizer(namespaceOffloading2, namespaceOffloadingControllerFinalizer)
			}, timeout, interval).Should(BeTrue())

			By(" 9 - Clean NamespaceOffloading Status")
			Eventually(func() bool {
				if err := homeClient.Get(context.TODO(), types.NamespacedName{
					Name:      liqoconst.DefaultNamespaceOffloadingName,
					Namespace: namespace2Name}, namespaceOffloading2); err != nil {
					return false
				}
				patch := namespaceOffloading2.DeepCopy()
				delete(namespaceOffloading2.Status.RemoteNamespacesConditions, remoteCluster1.ClusterID)
				err := homeClient.Patch(context.TODO(), namespaceOffloading2, client.MergeFrom(patch))
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By(" 10 - Check if NamespaceOffloading has been deleted")
			Eventually(func() bool {
				err := homeClient.Get(context.TODO(), types.NamespacedName{
					Name:      liqoconst.DefaultNamespaceOffloadingName,
					Namespace: namespace2Name}, namespaceOffloading2)
				return apierrors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())

		})

		It(" TEST 3: Create a NamespaceOffloading resource with an empty clusterSelector", func() {

			namespace3Name := "namespace3"
			remoteNamespace3Name := fmt.Sprintf("%s-%s", namespace3Name, foreignclusterutils.UniqueName(&localCluster))
			namespace3 := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace3Name,
				},
			}

			namespaceOffloading3 := &offv1alpha1.NamespaceOffloading{
				ObjectMeta: metav1.ObjectMeta{
					Name:      liqoconst.DefaultNamespaceOffloadingName,
					Namespace: namespace3Name,
				},
				Spec: offv1alpha1.NamespaceOffloadingSpec{
					NamespaceMappingStrategy: offv1alpha1.DefaultNameMappingStrategyType,
					PodOffloadingStrategy:    offv1alpha1.LocalAndRemotePodOffloadingStrategyType,
					ClusterSelector:          corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{}},
				},
			}

			By(fmt.Sprintf(" 1 - Create NamespaceOffloading resource in Namespace '%s'", namespace3Name))
			Expect(homeClient.Create(context.TODO(), namespace3)).To(Succeed())
			Eventually(func() bool {
				err := homeClient.Create(context.TODO(), namespaceOffloading3)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By(" 2 - Check NamespaceMaps DesiredMapping ")
			Eventually(func() bool {
				if err := homeClient.List(context.TODO(), nms); err != nil {
					return false
				}
				Expect(len(nms.Items) == mapNumber).To(BeTrue())
				for i := range nms.Items {
					if value, ok := nms.Items[i].Spec.DesiredMapping[namespace3Name]; !ok || value != remoteNamespace3Name {
						return false
					}
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By(" 3 - Check presence of the finalizer on the NamespaceOffloading")
			Eventually(func() bool {
				if err := homeClient.Get(context.TODO(), types.NamespacedName{
					Name:      liqoconst.DefaultNamespaceOffloadingName,
					Namespace: namespace3Name}, namespaceOffloading3); err != nil {
					return false
				}
				return ctrlutils.ContainsFinalizer(namespaceOffloading3, namespaceOffloadingControllerFinalizer)
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf(" 4 - Check scheduling label on the namespace %s", namespace3Name))
			Eventually(func() bool {
				if err := homeClient.Get(context.TODO(), types.NamespacedName{Name: namespace3Name}, namespace3); err != nil {
					return false
				}
				return namespace3.Labels[liqoconst.SchedulingLiqoLabel] == liqoconst.SchedulingLiqoLabelValue
			}, timeout, interval).Should(BeTrue())

			By(" 6 - Delete NamespaceOffloading resource")
			Eventually(func() bool {
				if err := homeClient.Get(context.TODO(), types.NamespacedName{
					Name:      liqoconst.DefaultNamespaceOffloadingName,
					Namespace: namespace3Name}, namespaceOffloading3); err != nil {
					return false
				}
				err := homeClient.Delete(context.TODO(), namespaceOffloading3)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By(" 7 - Check if NamespaceOffloading has been deleted")
			Eventually(func() bool {
				err := homeClient.Get(context.TODO(), types.NamespacedName{
					Name:      liqoconst.DefaultNamespaceOffloadingName,
					Namespace: namespace3Name}, namespaceOffloading3)
				return apierrors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())

		})

		It(" TEST 4: Create a NamespaceOffloading resource that doesn't select any cluster", func() {

			namespace4Name := "namespace4"
			namespace4 := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace4Name,
				},
			}

			namespaceOffloading4 := &offv1alpha1.NamespaceOffloading{
				ObjectMeta: metav1.ObjectMeta{
					Name:      liqoconst.DefaultNamespaceOffloadingName,
					Namespace: namespace4Name,
				},
				Spec: offv1alpha1.NamespaceOffloadingSpec{
					NamespaceMappingStrategy: offv1alpha1.EnforceSameNameMappingStrategyType,
					PodOffloadingStrategy:    offv1alpha1.LocalAndRemotePodOffloadingStrategyType,
					ClusterSelector: corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{
						MatchExpressions: []corev1.NodeSelectorRequirement{{
							Key:      regionLabel,
							Operator: corev1.NodeSelectorOpNotIn,
							Values:   []string{regionA, regionB},
						}},
						MatchFields: nil,
					},
					},
					},
				},
			}

			By(fmt.Sprintf(" 1 - Create NamespaceOffloading resource in Namespace '%s'", namespace4Name))
			Expect(homeClient.Create(context.TODO(), namespace4)).To(Succeed())
			Eventually(func() bool {
				err := homeClient.Create(context.TODO(), namespaceOffloading4)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf(" 2 - Check absence of scheduling label on the namespace %s", namespace4Name))
			Consistently(func() bool {
				if err := homeClient.Get(context.TODO(), types.NamespacedName{Name: namespace4Name}, namespace4); err != nil {
					return false
				}
				return namespace4.Labels == nil
			}, timeout/5, interval).Should(BeTrue())

			By(" 3 - Check presence of the finalizer on the NamespaceOffloading")
			Eventually(func() bool {
				if err := homeClient.Get(context.TODO(), types.NamespacedName{
					Name:      liqoconst.DefaultNamespaceOffloadingName,
					Namespace: namespace4Name}, namespaceOffloading4); err != nil {
					return false
				}
				return ctrlutils.ContainsFinalizer(namespaceOffloading4, namespaceOffloadingControllerFinalizer)
			}, timeout, interval).Should(BeTrue())

			By(" 4 - Check NamespaceMaps to be empty ")
			Consistently(func() bool {
				if err := homeClient.List(context.TODO(), nms); err != nil {
					return false
				}
				Expect(len(nms.Items) == mapNumber).To(BeTrue())
				return nms.Items[0].Spec.DesiredMapping == nil && nms.Items[1].Spec.DesiredMapping == nil &&
					nms.Items[2].Spec.DesiredMapping == nil
			}, timeout/10, interval).Should(BeTrue())

			By(fmt.Sprintf(" 5 - Delete NamespaceOffloading in the namespace %s", namespace4Name))
			Eventually(func() bool {
				if err := homeClient.Get(context.TODO(), types.NamespacedName{
					Name:      liqoconst.DefaultNamespaceOffloadingName,
					Namespace: namespace4Name}, namespaceOffloading4); err != nil {
					return false
				}
				err := homeClient.Delete(context.TODO(), namespaceOffloading4)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By(" 6 - Check if NamespaceOffloading has been deleted")
			Eventually(func() bool {
				err := homeClient.Get(context.TODO(), types.NamespacedName{
					Name:      liqoconst.DefaultNamespaceOffloadingName,
					Namespace: namespace4Name}, namespaceOffloading4)
				return apierrors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())

		})

		It(" TEST 5: Create 3 NamespaceOffloading resource and check desiredMapping of NamespaceMaps", func() {

			namespace5Name := "namespace5"
			namespace5 := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace5Name,
				},
			}

			namespace6Name := "namespace6"
			namespace6 := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace6Name,
				},
			}

			namespace7Name := "namespace7"
			namespace7 := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace7Name,
				},
			}

			namespaceOffloading5 := &offv1alpha1.NamespaceOffloading{
				ObjectMeta: metav1.ObjectMeta{
					Name:      liqoconst.DefaultNamespaceOffloadingName,
					Namespace: namespace5Name,
				},
				Spec: offv1alpha1.NamespaceOffloadingSpec{
					NamespaceMappingStrategy: offv1alpha1.EnforceSameNameMappingStrategyType,
					PodOffloadingStrategy:    offv1alpha1.LocalAndRemotePodOffloadingStrategyType,
					ClusterSelector: corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{
						MatchExpressions: []corev1.NodeSelectorRequirement{{
							Key:      regionLabel,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{regionA},
						}},
						MatchFields: nil,
					},
					},
					},
				},
			}

			namespaceOffloading6 := &offv1alpha1.NamespaceOffloading{
				ObjectMeta: metav1.ObjectMeta{
					Name:      liqoconst.DefaultNamespaceOffloadingName,
					Namespace: namespace6Name,
				},
				Spec: offv1alpha1.NamespaceOffloadingSpec{
					NamespaceMappingStrategy: offv1alpha1.EnforceSameNameMappingStrategyType,
					PodOffloadingStrategy:    offv1alpha1.LocalAndRemotePodOffloadingStrategyType,
					ClusterSelector: corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{
						MatchExpressions: []corev1.NodeSelectorRequirement{{
							Key:      regionLabel,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{regionB},
						}},
						MatchFields: nil,
					},
					},
					},
				},
			}

			namespaceOffloading7 := &offv1alpha1.NamespaceOffloading{
				ObjectMeta: metav1.ObjectMeta{
					Name:      liqoconst.DefaultNamespaceOffloadingName,
					Namespace: namespace7Name,
				},
				Spec: offv1alpha1.NamespaceOffloadingSpec{
					NamespaceMappingStrategy: offv1alpha1.EnforceSameNameMappingStrategyType,
					PodOffloadingStrategy:    offv1alpha1.LocalAndRemotePodOffloadingStrategyType,
					ClusterSelector: corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{
						MatchExpressions: []corev1.NodeSelectorRequirement{{
							Key:      providerLabel,
							Operator: corev1.NodeSelectorOpNotIn,
							Values:   []string{providerAWS},
						}},
						MatchFields: nil,
					},
					},
					},
				},
			}

			By(" 1 - Create NamespaceOffloading resources")
			Expect(homeClient.Create(context.TODO(), namespace5)).To(Succeed())
			Expect(homeClient.Create(context.TODO(), namespace6)).To(Succeed())
			Expect(homeClient.Create(context.TODO(), namespace7)).To(Succeed())

			Eventually(func() bool {
				err := homeClient.Create(context.TODO(), namespaceOffloading5)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				err := homeClient.Create(context.TODO(), namespaceOffloading6)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				err := homeClient.Create(context.TODO(), namespaceOffloading7)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By(" 2 - Check NamespaceMap of virtual nodes 1 ")
			Eventually(func() bool {
				Expect(homeClient.List(context.TODO(), nms, client.MatchingLabels{liqoconst.RemoteClusterID: remoteCluster1.ClusterID})).To(Succeed())
				Expect(len(nms.Items) == 1).To(BeTrue())
				if value, ok := nms.Items[0].Spec.DesiredMapping[namespace5Name]; !ok || value != namespace5Name {
					return false
				}
				if _, ok := nms.Items[0].Spec.DesiredMapping[namespace6Name]; ok {
					return false
				}
				if _, ok := nms.Items[0].Spec.DesiredMapping[namespace7Name]; ok {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By(" 3 - Check NamespaceMap of virtual nodes 2 ")
			Eventually(func() bool {
				Expect(homeClient.List(context.TODO(), nms, client.MatchingLabels{liqoconst.RemoteClusterID: remoteCluster2.ClusterID})).To(Succeed())
				Expect(len(nms.Items) == 1).To(BeTrue())
				if _, ok := nms.Items[0].Spec.DesiredMapping[namespace5Name]; ok {
					return false
				}
				if value, ok := nms.Items[0].Spec.DesiredMapping[namespace6Name]; !ok || value != namespace6Name {
					return false
				}
				if value, ok := nms.Items[0].Spec.DesiredMapping[namespace7Name]; !ok || value != namespace7Name {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By(" 4 - Check NamespaceMap of virtual node 3 ")
			Eventually(func() bool {
				Expect(homeClient.List(context.TODO(), nms, client.MatchingLabels{liqoconst.RemoteClusterID: remoteCluster3.ClusterID})).To(Succeed())
				Expect(len(nms.Items) == 1).To(BeTrue())
				if value, ok := nms.Items[0].Spec.DesiredMapping[namespace5Name]; !ok || value != namespace5Name {
					return false
				}
				if _, ok := nms.Items[0].Spec.DesiredMapping[namespace6Name]; ok {
					return false
				}
				if value, ok := nms.Items[0].Spec.DesiredMapping[namespace7Name]; !ok || value != namespace7Name {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By(" 5 - Delete NamespaceOffloading resources")
			Eventually(func() bool {
				if err := homeClient.Get(context.TODO(), types.NamespacedName{
					Name:      liqoconst.DefaultNamespaceOffloadingName,
					Namespace: namespace5Name}, namespaceOffloading5); err != nil {
					return false
				}
				if err := homeClient.Get(context.TODO(), types.NamespacedName{
					Name:      liqoconst.DefaultNamespaceOffloadingName,
					Namespace: namespace6Name}, namespaceOffloading6); err != nil {
					return false
				}
				if err := homeClient.Get(context.TODO(), types.NamespacedName{
					Name:      liqoconst.DefaultNamespaceOffloadingName,
					Namespace: namespace7Name}, namespaceOffloading7); err != nil {
					return false
				}
				if err := homeClient.Delete(context.TODO(), namespaceOffloading5); err != nil {
					return false
				}
				if err := homeClient.Delete(context.TODO(), namespaceOffloading6); err != nil {
					return false
				}
				if err := homeClient.Delete(context.TODO(), namespaceOffloading7); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By(" 7 - Check if there are no DesiredMapping")
			Eventually(func() bool {
				if err := homeClient.List(context.TODO(), nms); err != nil {
					return false
				}
				Expect(len(nms.Items) == mapNumber).To(BeTrue())
				for i := range nms.Items {
					if nms.Items[i].Spec.DesiredMapping != nil {
						return false
					}
				}
				return true
			}, timeout, interval).Should(BeTrue())

		})

	})

})
