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

package namespacectrl

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	offv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

var _ = Describe("Namespace controller", func() {

	Context("Add and remove liqo Label from namespace-test", func() {

		BeforeEach(func() {
			By("0 - BeforeEach -> Delete NamespaceOffloading resource if present")
			Eventually(func() bool {
				if err := k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: nameNamespaceTest,
					Name:      liqoconst.DefaultNamespaceOffloadingName,
				}, namespaceOffloading); err != nil {
					return apierrors.IsNotFound(err)
				}
				_ = k8sClient.Delete(context.TODO(), namespaceOffloading)
				return false
			}, timeout, interval).Should(BeTrue())
		})

		It("Add liqo label when NamespaceOffloading is not present, and check if it is deleted", func() {

			By(fmt.Sprintf("1 - Get namespace '%s' and add liqo label", nameNamespaceTest))
			Eventually(func() bool {
				if err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: nameNamespaceTest}, namespace); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
			if namespace.Labels == nil {
				namespace.Labels = map[string]string{}
			}
			namespace.Labels[liqoconst.EnablingLiqoLabel] = liqoconst.EnablingLiqoLabelValue
			Eventually(func() bool {
				if err := k8sClient.Update(context.TODO(), namespace); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("2 - Try to get NamespaceOffloading Resource associated with this namespace")
			Eventually(func() bool {
				if err := k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: nameNamespaceTest,
					Name:      liqoconst.DefaultNamespaceOffloadingName,
				}, namespaceOffloading); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("3 - Check if the ownership annotation is present")
			value, ok := namespaceOffloading.Annotations[nsCtrlAnnotationKey]
			Expect(ok && value == nsCtrlAnnotationValue).To(BeTrue())

			By("4 - Remove liqo label")
			delete(namespace.Labels, liqoconst.EnablingLiqoLabel)
			Eventually(func() bool {
				if err := k8sClient.Update(context.TODO(), namespace); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("5 - Check if NamespaceOffloading is deleted")
			Eventually(func() bool {
				err := k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: nameNamespaceTest,
					Name:      liqoconst.DefaultNamespaceOffloadingName,
				}, namespaceOffloading)
				if err != nil && apierrors.IsNotFound(err) {
					return true
				}
				return false
			}, timeout, interval).Should(BeTrue())

		})

		It("Add liqo label when NamespaceOffloading is present, and check that the resource is not deleted", func() {

			By(fmt.Sprintf("1 - Create NamespaceOffloading resource for namespace '%s'", nameNamespaceTest))
			namespaceOffloading = &offv1alpha1.NamespaceOffloading{
				ObjectMeta: metav1.ObjectMeta{
					Name:      liqoconst.DefaultNamespaceOffloadingName,
					Namespace: nameNamespaceTest,
				},
				Spec: offv1alpha1.NamespaceOffloadingSpec{
					NamespaceMappingStrategy: offv1alpha1.DefaultNameMappingStrategyType,
					PodOffloadingStrategy:    offv1alpha1.LocalAndRemotePodOffloadingStrategyType,
					ClusterSelector:          corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{}},
				},
			}
			Expect(k8sClient.Create(context.TODO(), namespaceOffloading)).Should(Succeed())

			By(fmt.Sprintf("2 - Get namespace '%s' and add liqo label", nameNamespaceTest))
			Eventually(func() bool {
				if err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: nameNamespaceTest}, namespace); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
			if namespace.Labels == nil {
				namespace.Labels = map[string]string{}
			}
			namespace.Labels[liqoconst.EnablingLiqoLabel] = liqoconst.EnablingLiqoLabelValue
			Eventually(func() bool {
				if err := k8sClient.Update(context.TODO(), namespace); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("3 - Check if the ownership annotation is present")
			value, ok := namespaceOffloading.Annotations[nsCtrlAnnotationKey]
			Expect(ok && value == nsCtrlAnnotationValue).To(BeFalse())

			By("4 - Remove liqo label")
			delete(namespace.Labels, liqoconst.EnablingLiqoLabel)
			Eventually(func() bool {
				if err := k8sClient.Update(context.TODO(), namespace); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("5 - Check if NamespaceOffloading is deleted")
			Consistently(func() bool {
				err := k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: nameNamespaceTest,
					Name:      liqoconst.DefaultNamespaceOffloadingName,
				}, namespaceOffloading)
				return err == nil
			}, timeout/5, interval).Should(BeTrue())

		})

	})

})
