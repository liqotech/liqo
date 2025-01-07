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

package virtualnodectrl

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

func ForgeFakeVirtualNode(nameVirtualNode, tenantNamespaceName string,
	remoteClusterID liqov1beta1.ClusterID) *offloadingv1beta1.VirtualNode {
	return &offloadingv1beta1.VirtualNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nameVirtualNode,
			Namespace: tenantNamespaceName,
			Labels: map[string]string{
				liqoconst.RemoteClusterID: string(remoteClusterID),
			},
		},
		Spec: offloadingv1beta1.VirtualNodeSpec{
			ClusterID:  remoteClusterID,
			CreateNode: ptr.To(true),
			Template: &offloadingv1beta1.DeploymentTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nameVirtualNode,
					Namespace: tenantNamespaceName,
					Labels: map[string]string{
						"virtual-node": nameVirtualNode,
					},
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"virtual-node": nameVirtualNode,
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"virtual-node": nameVirtualNode,
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "virtual-kubelet",
									Image: "virtual-kubelet-image",
								},
							},
						},
					},
				},
			},
		},
	}
}

var _ = Describe("VirtualNode controller", func() {

	Context("Check if resources VirtualNodes and NamespaceMaps are correctly initialized", func() {

		BeforeEach(func() {
			virtualNode1 = ForgeFakeVirtualNode(nameVirtualNode1, tenantNamespace1.Name, remoteClusterID1)

			virtualNode2 = ForgeFakeVirtualNode(nameVirtualNode2, tenantNamespace2.Name, remoteClusterID2)

			time.Sleep(2 * time.Second)
			By(fmt.Sprintf("Create the virtual-node '%s'", nameVirtualNode1))
			Expect(k8sClient.Create(ctx, virtualNode1)).Should(Succeed())
			By(fmt.Sprintf("Create the virtual-node '%s'", nameVirtualNode2))
			Expect(k8sClient.Create(ctx, virtualNode2)).Should(Succeed())
		})

		AfterEach(func() {
			vn := &offloadingv1beta1.VirtualNode{}
			By(fmt.Sprintf("Delete the virtual-node '%s'", nameVirtualNode1))
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: nameVirtualNode1, Namespace: tenantNamespace1.Name}, vn)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, vn)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: nameVirtualNode1, Namespace: tenantNamespace1.Name}, virtualNode1)
				return apierrors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
			By(fmt.Sprintf("Delete the virtual-node '%s'", nameVirtualNode2))
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: nameVirtualNode2, Namespace: tenantNamespace2.Name}, vn)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, vn)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: nameVirtualNode2, Namespace: tenantNamespace2.Name}, virtualNode2)
				return apierrors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})

		It("Check NamespaceMaps presence", func() {

			By(fmt.Sprintf("Try to get NamespaceMap associated to: %s", remoteClusterID1))
			Eventually(func() bool {
				if err := k8sClient.List(ctx, nms, client.InNamespace(tenantNamespace1.Name),
					client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterID1}); err != nil {
					return false
				}
				return len(nms.Items) == 1
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf("Try to get NamespaceMap associated to: %s", remoteClusterID2))
			Eventually(func() bool {
				if err := k8sClient.List(ctx, nms, client.InNamespace(tenantNamespace2.Name),
					client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterID2}); err != nil {
					return false
				}
				return len(nms.Items) == 1
			}, timeout, interval).Should(BeTrue())

		})

		It(fmt.Sprintf("Check if finalizers are correctly created for %s", nameVirtualNode1), func() {

			By(fmt.Sprintf("Try to get virtual-node: %s", nameVirtualNode1))
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: nameVirtualNode1, Namespace: tenantNamespace1.Name}, virtualNode1)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf("Try to get NamespaceMap associated to: %s", remoteClusterID1))
			Eventually(func() bool {
				if err := k8sClient.List(ctx, nms, client.InNamespace(tenantNamespace1.Name),
					client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterID1}); err != nil {
					return false
				}
				return len(nms.Items) == 1
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf("Try to check presence of finalizer on the virtual-Node: %s", virtualNode1.GetName()))
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: nameVirtualNode1, Namespace: tenantNamespace1.Name},
					virtualNode1); err != nil {
					return false
				}
				return controllerutil.ContainsFinalizer(virtualNode1, virtualNodeControllerFinalizer)
			}, timeout, interval).Should(BeTrue())

		})

		It(fmt.Sprintf("Check if finalizers are correctly created for %s", nameVirtualNode2), func() {

			By(fmt.Sprintf("Try to get virtual-node: %s", nameVirtualNode2))
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: nameVirtualNode2, Namespace: tenantNamespace2.Name}, virtualNode2)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf("Try to get NamespaceMap associated to: %s", remoteClusterID2))
			Eventually(func() bool {
				if err := k8sClient.List(ctx, nms, client.InNamespace(tenantNamespace2.Name),
					client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterID2}); err != nil {
					return false
				}
				return len(nms.Items) == 1
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf("Try to check presence of finalizer on the virtual-Node: %s", virtualNode2.GetName()))
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: nameVirtualNode2, Namespace: tenantNamespace2.Name},
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
			Expect(k8sClient.Create(ctx, simpleNode)).Should(Succeed())

			By(fmt.Sprintf("Try to get not virtual-node: %s", nameSimpleNode))
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: nameSimpleNode}, simpleNode)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf("Check absence of finalizer %s: ", virtualNodeControllerFinalizer))
			Consistently(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: nameSimpleNode},
					simpleNode); err != nil {
					return false
				}
				return !controllerutil.ContainsFinalizer(simpleNode, virtualNodeControllerFinalizer)
			}, timeout/5, interval).Should(BeTrue())

			By(fmt.Sprintf("Delete the simple-node '%s'", nameSimpleNode))
			Expect(k8sClient.Delete(ctx, simpleNode)).Should(Succeed())

		})

	})

	Context("Check deletion lifecycle of Namespacemaps associated with virtual-node 1 ", func() {

		It(fmt.Sprintf("Check regeneration of NamespaceMap associated to %s", remoteClusterID1), func() {

			virtualNode1 = ForgeFakeVirtualNode(nameVirtualNode1, tenantNamespace1.Name, remoteClusterID1)
			By(fmt.Sprintf("Create the virtual-node '%s'", nameVirtualNode1))
			Expect(k8sClient.Create(ctx, virtualNode1)).Should(Succeed())

			var oldUUID types.UID
			By(fmt.Sprintf("Try to delete NamespaceMap associated to: %s", remoteClusterID1))
			Eventually(func() bool {
				if err := k8sClient.List(ctx, nms,
					client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterID1}); err != nil {
					return false
				}
				if len(nms.Items) != 1 {
					return false
				}
				oldUUID = nms.Items[0].UID
				err := k8sClient.Delete(ctx, &nms.Items[0])
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf("Try to get new NamespaceMap associated to: %s", remoteClusterID1))
			Eventually(func() bool {
				if err := k8sClient.List(ctx, nms, client.InNamespace(tenantNamespace1.Name),
					client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterID1}); err != nil {
					return false
				}
				return len(nms.Items) == 1 && oldUUID != nms.Items[0].UID
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf("Delete the virtual-node '%s'", nameVirtualNode1))
			Expect(k8sClient.Delete(ctx, virtualNode1)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: nameVirtualNode1}, virtualNode1)
				return apierrors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})

	})

})
