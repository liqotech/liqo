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

package virtualnodectrl

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

var _ = Describe("VirtualNode controller", func() {

	Context("Check if resources VirtualNodes and NamespaceMaps are correctly initialized", func() {

		BeforeEach(func() {
			virtualNode1 = &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: nameVirtualNode1,
					Labels: map[string]string{
						liqoconst.TypeLabel:       liqoconst.TypeNode,
						liqoconst.RemoteClusterID: remoteClusterID1,
					},
				},
			}
			virtualNode2 = &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: nameVirtualNode2,
					Labels: map[string]string{
						liqoconst.TypeLabel:       liqoconst.TypeNode,
						liqoconst.RemoteClusterID: remoteClusterID2,
					},
				},
			}
			By(fmt.Sprintf("Create the virtual-node '%s'", nameVirtualNode1))
			Expect(k8sClient.Create(context.TODO(), virtualNode1)).Should(Succeed())
			By(fmt.Sprintf("Create the virtual-node '%s'", nameVirtualNode2))
			Expect(k8sClient.Create(context.TODO(), virtualNode2)).Should(Succeed())
		})

		AfterEach(func() {
			By(fmt.Sprintf("Delete the virtual-node '%s'", nameVirtualNode1))
			Expect(k8sClient.Delete(context.TODO(), virtualNode1)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: nameVirtualNode1}, virtualNode1)
				return apierrors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
			By(fmt.Sprintf("Delete the virtual-node '%s'", nameVirtualNode2))
			Expect(k8sClient.Delete(context.TODO(), virtualNode2)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: nameVirtualNode2}, virtualNode2)
				return apierrors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})

		It("Check NamespaceMaps presence", func() {

			By(fmt.Sprintf("Try to get NamespaceMap associated to: %s", remoteClusterID1))
			Eventually(func() bool {
				if err := k8sClient.List(context.TODO(), nms, client.InNamespace(tenantNamespaceNameID1),
					client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterID1}); err != nil {
					return false
				}
				return len(nms.Items) == 1
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf("Try to get NamespaceMap associated to: %s", remoteClusterID2))
			Eventually(func() bool {
				if err := k8sClient.List(context.TODO(), nms, client.InNamespace(tenantNamespaceNameID2),
					client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterID2}); err != nil {
					return false
				}
				return len(nms.Items) == 1
			}, timeout, interval).Should(BeTrue())

		})

		It(fmt.Sprintf("Check if finalizers and ownerReference are correctly created for %s", nameVirtualNode1), func() {

			By(fmt.Sprintf("Try to get virtual-node: %s", nameVirtualNode1))
			Eventually(func() bool {
				err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: nameVirtualNode1}, virtualNode1)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf("Try to get NamespaceMap associated to: %s", remoteClusterID1))
			Eventually(func() bool {
				if err := k8sClient.List(context.TODO(), nms, client.InNamespace(tenantNamespaceNameID1),
					client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterID1}); err != nil {
					return false
				}
				return len(nms.Items) == 1
			}, timeout, interval).Should(BeTrue())

			expectedOwnerReference := metav1.OwnerReference{
				APIVersion:         "v1",
				BlockOwnerDeletion: pointer.BoolPtr(true),
				Kind:               "Node",
				Name:               virtualNode1.GetName(),
				UID:                virtualNode1.GetUID(),
				Controller:         pointer.BoolPtr(true),
			}

			By(fmt.Sprintf("Try to check the ownership of the NamespaceMap: %s", nms.Items[0].GetName()))
			Expect(nms.Items[0].GetOwnerReferences()).To(ContainElement(expectedOwnerReference))

			By(fmt.Sprintf("Try to check presence of finalizer on the virtual-Node: %s", virtualNode1.GetName()))
			Eventually(func() bool {
				if err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: nameVirtualNode1},
					virtualNode1); err != nil {
					return false
				}
				return controllerutil.ContainsFinalizer(virtualNode1, virtualNodeControllerFinalizer)
			}, timeout, interval).Should(BeTrue())

		})

		It(fmt.Sprintf("Check if finalizers and ownerReference are correctly created for %s", nameVirtualNode2), func() {

			By(fmt.Sprintf("Try to get virtual-node: %s", nameVirtualNode2))
			Eventually(func() bool {
				err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: nameVirtualNode2}, virtualNode2)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf("Try to get NamespaceMap associated to: %s", remoteClusterID2))
			Eventually(func() bool {
				if err := k8sClient.List(context.TODO(), nms, client.InNamespace(tenantNamespaceNameID2),
					client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterID2}); err != nil {
					return false
				}
				return len(nms.Items) == 1
			}, timeout, interval).Should(BeTrue())

			expectedOwnerReference := metav1.OwnerReference{
				APIVersion:         "v1",
				BlockOwnerDeletion: pointer.BoolPtr(true),
				Kind:               "Node",
				Name:               virtualNode2.GetName(),
				UID:                virtualNode2.GetUID(),
				Controller:         pointer.BoolPtr(true),
			}

			By(fmt.Sprintf("Try to check ownership of NamespaceMap: %s", nms.Items[0].GetName()))
			Expect(nms.Items[0].GetOwnerReferences()).To(ContainElement(expectedOwnerReference))

			By(fmt.Sprintf("Try to check presence of finalizer in VirtualNode: %s", virtualNode2.GetName()))
			// i have to update my node instance, because finalizer could be updated after my first get
			Eventually(func() bool {
				if err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: nameVirtualNode2},
					virtualNode2); err != nil {
					return false
				}
				return controllerutil.ContainsFinalizer(virtualNode2, virtualNodeControllerFinalizer)
			}, timeout, interval).Should(BeTrue())

		})

	})

	Context("Check if a not virtual node is monitored", func() {

		It("Check absence of NamespaceMap and of finalizer", func() {

			simpleNode = &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: nameSimpleNode,
					Labels: map[string]string{
						liqoconst.RemoteClusterID: remoteClusterIDSimpleNode,
						offloadingCluster1Label1:  "",
						offloadingCluster1Label2:  "",
					},
				},
			}
			By(fmt.Sprintf("Create the simple-node '%s'", nameSimpleNode))
			Expect(k8sClient.Create(context.TODO(), simpleNode)).Should(Succeed())

			By(fmt.Sprintf("Try to get not virtual-node: %s", nameSimpleNode))
			Eventually(func() bool {
				err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: nameSimpleNode}, simpleNode)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf("Check absence of finalizer %s: ", virtualNodeControllerFinalizer))
			Consistently(func() bool {
				if err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: nameSimpleNode},
					simpleNode); err != nil {
					return false
				}
				return !controllerutil.ContainsFinalizer(simpleNode, virtualNodeControllerFinalizer)
			}, timeout/5, interval).Should(BeTrue())

			By(fmt.Sprintf("Delete the simple-node '%s'", nameSimpleNode))
			Expect(k8sClient.Delete(context.TODO(), simpleNode)).Should(Succeed())

		})

	})

	Context("Check deletion lifecycle of Namespacemaps associated with virtual-node 1 ", func() {

		It(fmt.Sprintf("Check regeneration of NamespaceMap associated to %s", remoteClusterID1), func() {

			virtualNode1 = &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: nameVirtualNode1,
					Labels: map[string]string{
						liqoconst.TypeLabel:       liqoconst.TypeNode,
						liqoconst.RemoteClusterID: remoteClusterID1,
					},
				},
			}
			By(fmt.Sprintf("Create the virtual-node '%s'", nameVirtualNode1))
			Expect(k8sClient.Create(context.TODO(), virtualNode1)).Should(Succeed())

			var oldUUID types.UID
			By(fmt.Sprintf("Try to delete NamespaceMap associated to: %s", remoteClusterID1))
			Eventually(func() bool {
				if err := k8sClient.List(context.TODO(), nms,
					client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterID1}); err != nil {
					return false
				}
				if len(nms.Items) != 1 {
					return false
				}
				oldUUID = nms.Items[0].UID
				err := k8sClient.Delete(context.TODO(), &nms.Items[0])
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf("Try to get new NamespaceMap associated to: %s", remoteClusterID1))
			Eventually(func() bool {
				if err := k8sClient.List(context.TODO(), nms, client.InNamespace(tenantNamespaceNameID1),
					client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterID1}); err != nil {
					return false
				}
				return len(nms.Items) == 1 && oldUUID != nms.Items[0].UID
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf("Delete the virtual-node '%s'", nameVirtualNode1))
			Expect(k8sClient.Delete(context.TODO(), virtualNode1)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: nameVirtualNode1}, virtualNode1)
				return apierrors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())

		})

	})

})
