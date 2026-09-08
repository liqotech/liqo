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

package virtualnodectrl

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/vkMachinery"
	vkforge "github.com/liqotech/liqo/pkg/vkMachinery/forge"
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
			ClusterID: remoteClusterID,
			// CreateNode and DisableNetworkCheck are left unset, to inherit their values
			// from the referenced (default) VkOptionsTemplate.
			KubeconfigSecretRef: &corev1.LocalObjectReference{Name: "vn-kubeconfig"},
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

		It(fmt.Sprintf("Check if the auth-delegator ClusterRoleBinding is created for %s", nameVirtualNode1), func() {
			expectedCRBName := vkforge.VirtualKubeletAuthDelegatorClusterRoleBindingName(virtualNode1.Name)

			By(fmt.Sprintf("Try to get the auth-delegator ClusterRoleBinding: %s", expectedCRBName))
			Eventually(func() bool {
				var crb rbacv1.ClusterRoleBinding
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: expectedCRBName}, &crb); err != nil {
					return false
				}
				// Verify the binding points to the built-in system:auth-delegator role.
				if crb.RoleRef.Name != "system:auth-delegator" || crb.RoleRef.Kind != "ClusterRole" {
					return false
				}
				// Verify the subject is the virtual kubelet service account.
				if len(crb.Subjects) != 1 || crb.Subjects[0].Name != vkforge.VirtualKubeletServiceAccountName(virtualNode1.Name) ||
					crb.Subjects[0].Namespace != tenantNamespace1.Name {
					return false
				}
				return true
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
				err := k8sClient.Get(ctx, types.NamespacedName{Name: nameVirtualNode1, Namespace: tenantNamespace1.Name}, virtualNode1)
				return apierrors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})

	})

	Context("Check the virtual-kubelet Deployment is forged from the VkOptionsTemplate", func() {

		var vnDeployment *appsv1.Deployment

		getVnDeployment := func(g Gomega) *appsv1.Deployment {
			vnDeployment = &appsv1.Deployment{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      "vk-" + nameVirtualNode1,
				Namespace: tenantNamespace1.Name,
			}, vnDeployment)).To(Succeed())
			return vnDeployment
		}

		deleteVirtualNode := func(name, namespace string) {
			vn := &offloadingv1beta1.VirtualNode{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, vn)
			if apierrors.IsNotFound(err) {
				return
			}
			ExpectWithOffset(1, err).ToNot(HaveOccurred())
			ExpectWithOffset(1, k8sClient.Delete(ctx, vn)).To(Succeed())
			EventuallyWithOffset(1, func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, vn)
				return apierrors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		}

		AfterEach(func() {
			// delete the virtual-node (and the resources it owns)
			deleteVirtualNode(nameVirtualNode1, tenantNamespace1.Name)
		})

		It("Check the deployment is forged from the default VkOptionsTemplate", func() {
			virtualNode1 = ForgeFakeVirtualNode(nameVirtualNode1, tenantNamespace1.Name, remoteClusterID1)
			Expect(k8sClient.Create(ctx, virtualNode1)).Should(Succeed())

			Eventually(func(g Gomega) {
				vnDeployment = getVnDeployment(g)
				g.Expect(vnDeployment.Spec.Template.Spec.Containers).To(HaveLen(1))
			}, timeout, interval).Should(Succeed())

			Expect(vnDeployment.Spec.Template.Spec.Containers[0].Image).To(Equal("liqo/virtual-kubelet:test"))
			Expect(vnDeployment.Spec.Template.Spec.Containers[0].Args).To(ContainElement("--create-node=true"))
			Expect(vnDeployment.Spec.Template.Spec.Containers[0].Args).To(ContainElement("--node-check-network=true"))
			Expect(vnDeployment.Spec.Template.Spec.Containers[0].Args).To(
				ContainElement("--foreign-kubeconfig-secret-name=vn-kubeconfig"))
			Expect(vnDeployment.Spec.Template.Spec.Containers[0].Args).To(ContainElement("--local-podcidr=10.0.0.0/16"))

			By("The deployment rolls out safely")
			Expect(vnDeployment.Spec.Strategy.Type).To(Equal(appsv1.RollingUpdateDeploymentStrategyType))
			Expect(vnDeployment.Spec.Strategy.RollingUpdate).NotTo(BeNil())
			Expect(vnDeployment.Spec.Strategy.RollingUpdate.MaxSurge.IntVal).To(Equal(int32(1)))
			Expect(vnDeployment.Spec.Strategy.RollingUpdate.MaxUnavailable.IntVal).To(BeZero())
			Expect(vnDeployment.Spec.MinReadySeconds).To(BeZero())
			Expect(vnDeployment.Spec.Template.Spec.Containers[0].ReadinessProbe).NotTo(BeNil())
			Expect(vnDeployment.Spec.Template.Spec.Containers[0].ReadinessProbe.HTTPGet.Path).To(Equal("/readyz"))
			Expect(vnDeployment.Spec.Template.Spec.Containers[0].ReadinessProbe.HTTPGet.Port.IntVal).
				To(Equal(int32(vkMachinery.HealthPort)))

			By("The effective CreateNode is inherited from the template, without any spec write-back")
			Consistently(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name: nameVirtualNode1, Namespace: tenantNamespace1.Name,
				}, virtualNode1)).To(Succeed())
				g.Expect(virtualNode1.Spec.CreateNode).To(BeNil())
				g.Expect(virtualNode1.Spec.DisableNetworkCheck).To(BeNil())
			}, timeout/5, interval).Should(Succeed())

			By("The effective values are published in the status")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name: nameVirtualNode1, Namespace: tenantNamespace1.Name,
				}, virtualNode1)).To(Succeed())
				g.Expect(virtualNode1.Status.EffectiveCreateNode).To(HaveValue(BeTrue()))
				g.Expect(virtualNode1.Status.EffectiveDisableNetworkCheck).To(HaveValue(BeFalse()))
			}, timeout, interval).Should(Succeed())
		})

		It("Check the deployment and the ServiceAccount are owned by the VirtualNode", func() {
			virtualNode1 = ForgeFakeVirtualNode(nameVirtualNode1, tenantNamespace1.Name, remoteClusterID1)
			Expect(k8sClient.Create(ctx, virtualNode1)).Should(Succeed())

			Eventually(func(g Gomega) {
				vnDeployment = getVnDeployment(g)
			}, timeout, interval).Should(Succeed())
			Expect(metav1.GetControllerOf(vnDeployment)).NotTo(BeNil())
			Expect(metav1.GetControllerOf(vnDeployment).Name).To(Equal(nameVirtualNode1))
			Expect(vnDeployment.Spec.Template.Spec.ServiceAccountName).To(
				Equal(vkforge.VirtualKubeletServiceAccountName(nameVirtualNode1)))

			vkServiceAccount := &corev1.ServiceAccount{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name: vkforge.VirtualKubeletServiceAccountName(nameVirtualNode1), Namespace: tenantNamespace1.Name,
				}, vkServiceAccount)).To(Succeed())
			}, timeout, interval).Should(Succeed())
			Expect(metav1.GetControllerOf(vkServiceAccount)).NotTo(BeNil())
			Expect(metav1.GetControllerOf(vkServiceAccount).Name).To(Equal(nameVirtualNode1))
		})

		It("Check legacy ServiceAccounts and ClusterRoleBinding subjects are preserved", func() {
			By("Pre-create a legacy ServiceAccount owned by another controller and a legacy ClusterRoleBinding")
			legacySA := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nameVirtualNode1,
					Namespace: tenantNamespace1.Name,
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: "networking.liqo.io/v1beta1",
						Kind:       "WgGatewayClient",
						Name:       "foreign-owner",
						UID:        "foreign-owner-uid",
						Controller: ptr.To(true),
					}},
				},
			}
			Expect(k8sClient.Create(ctx, legacySA)).Should(Succeed())

			legacyCRB := &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:   vkforge.VirtualKubeletClusterRoleBindingName(nameVirtualNode1),
					Labels: vkforge.ClusterRoleLabels(remoteClusterID1),
				},
				Subjects: []rbacv1.Subject{{
					Kind:      "ServiceAccount",
					APIGroup:  "",
					Name:      nameVirtualNode1,
					Namespace: tenantNamespace1.Name,
				}},
				RoleRef: rbacv1.RoleRef{
					APIGroup: rbacv1.GroupName,
					Kind:     "ClusterRole",
					Name:     vkMachinery.LocalClusterRoleName,
				},
			}
			Expect(k8sClient.Create(ctx, legacyCRB)).Should(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, legacySA))).Should(Succeed())
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, legacyCRB))).Should(Succeed())
			})

			virtualNode1 = ForgeFakeVirtualNode(nameVirtualNode1, tenantNamespace1.Name, remoteClusterID1)
			Expect(k8sClient.Create(ctx, virtualNode1)).Should(Succeed())

			By("The reconcile succeeds, forging the deployment with the new ServiceAccount")
			Eventually(func(g Gomega) {
				vnDeployment = getVnDeployment(g)
				g.Expect(vnDeployment.Spec.Template.Spec.ServiceAccountName).To(
					Equal(vkforge.VirtualKubeletServiceAccountName(nameVirtualNode1)))
			}, timeout, interval).Should(Succeed())

			By("The ClusterRoleBinding carries both the legacy and the new subjects")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name: legacyCRB.Name,
				}, legacyCRB)).To(Succeed())
				g.Expect(legacyCRB.Subjects).To(ContainElement(rbacv1.Subject{
					Kind:      "ServiceAccount",
					APIGroup:  "",
					Name:      nameVirtualNode1,
					Namespace: tenantNamespace1.Name,
				}))
				g.Expect(legacyCRB.Subjects).To(ContainElement(rbacv1.Subject{
					Kind:      "ServiceAccount",
					APIGroup:  "",
					Name:      vkforge.VirtualKubeletServiceAccountName(nameVirtualNode1),
					Namespace: tenantNamespace1.Name,
				}))
			}, timeout, interval).Should(Succeed())

			By("The legacy ServiceAccount is left untouched")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name: nameVirtualNode1, Namespace: tenantNamespace1.Name,
				}, legacySA)).To(Succeed())
				g.Expect(metav1.GetControllerOf(legacySA).Name).To(Equal("foreign-owner"))
			}, timeout, interval).Should(Succeed())

			By("Deleting the VirtualNode leaves the legacy ServiceAccount untouched")
			deleteVirtualNode(nameVirtualNode1, tenantNamespace1.Name)
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name: nameVirtualNode1, Namespace: tenantNamespace1.Name,
			}, legacySA)).To(Succeed())
			Expect(metav1.GetControllerOf(legacySA).Name).To(Equal("foreign-owner"))
		})

		It("Check the VkOptionsTemplate extra labels and annotations are enforced and stale ones removed", func() {
			virtualNode1 = ForgeFakeVirtualNode(nameVirtualNode1, tenantNamespace1.Name, remoteClusterID1)
			Expect(k8sClient.Create(ctx, virtualNode1)).Should(Succeed())

			Eventually(func(g Gomega) {
				vnDeployment = getVnDeployment(g)
			}, timeout, interval).Should(Succeed())

			By("Adding extra labels and annotations to the VkOptionsTemplate")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name: defaultVkOptsName, Namespace: liqoNamespace,
				}, defaultVkOptsTemplate)).To(Succeed())
				defaultVkOptsTemplate.Spec.ExtraLabels = map[string]string{"test-extra-label": "value"}
				defaultVkOptsTemplate.Spec.ExtraAnnotations = map[string]string{"test-extra-annot": "value"}
				g.Expect(k8sClient.Update(ctx, defaultVkOptsTemplate)).To(Succeed())
			}, timeout, interval).Should(Succeed())
			DeferCleanup(func() {
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, types.NamespacedName{
						Name: defaultVkOptsName, Namespace: liqoNamespace,
					}, defaultVkOptsTemplate)).To(Succeed())
					defaultVkOptsTemplate.Spec.ExtraLabels = nil
					defaultVkOptsTemplate.Spec.ExtraAnnotations = nil
					g.Expect(k8sClient.Update(ctx, defaultVkOptsTemplate)).To(Succeed())
				}, timeout, interval).Should(Succeed())
			})

			By("The extra labels and annotations are enforced on the deployment and its pod template")
			Eventually(func(g Gomega) {
				vnDeployment = getVnDeployment(g)
				g.Expect(vnDeployment.Labels).To(HaveKeyWithValue("test-extra-label", "value"))
				g.Expect(vnDeployment.Annotations).To(HaveKeyWithValue("test-extra-annot", "value"))
				g.Expect(vnDeployment.Spec.Template.Annotations).To(HaveKeyWithValue("test-extra-annot", "value"))
			}, timeout, interval).Should(Succeed())

			By("Removing them from the VkOptionsTemplate")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name: defaultVkOptsName, Namespace: liqoNamespace,
				}, defaultVkOptsTemplate)).To(Succeed())
				defaultVkOptsTemplate.Spec.ExtraLabels = nil
				defaultVkOptsTemplate.Spec.ExtraAnnotations = nil
				g.Expect(k8sClient.Update(ctx, defaultVkOptsTemplate)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			By("The stale labels and annotations are removed from the deployment and its pod template")
			Eventually(func(g Gomega) {
				vnDeployment = getVnDeployment(g)
				g.Expect(vnDeployment.Labels).ToNot(HaveKey("test-extra-label"))
				g.Expect(vnDeployment.Annotations).ToNot(HaveKey("test-extra-annot"))
				g.Expect(vnDeployment.Spec.Template.Annotations).ToNot(HaveKey("test-extra-annot"))
			}, timeout, interval).Should(Succeed())
		})

		It("Check operators' rollout actions on the deployment are not reverted", func() {
			virtualNode1 = ForgeFakeVirtualNode(nameVirtualNode1, tenantNamespace1.Name, remoteClusterID1)
			Expect(k8sClient.Create(ctx, virtualNode1)).Should(Succeed())

			Eventually(func(g Gomega) {
				vnDeployment = getVnDeployment(g)
			}, timeout, interval).Should(Succeed())

			By("Simulating kubectl rollout restart and kubectl rollout pause")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      "vk-" + nameVirtualNode1,
					Namespace: tenantNamespace1.Name,
				}, vnDeployment)).To(Succeed())
				if vnDeployment.Spec.Template.Annotations == nil {
					vnDeployment.Spec.Template.Annotations = make(map[string]string)
				}
				vnDeployment.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = "now"
				vnDeployment.Spec.Paused = true
				g.Expect(k8sClient.Update(ctx, vnDeployment)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			By("The reconcile does not revert the restartedAt annotation and the paused flag")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      "vk-" + nameVirtualNode1,
					Namespace: tenantNamespace1.Name,
				}, vnDeployment)).To(Succeed())
				g.Expect(vnDeployment.Spec.Template.Annotations).To(HaveKey("kubectl.kubernetes.io/restartedAt"))
				g.Expect(vnDeployment.Spec.Paused).To(BeTrue())
				g.Expect(vnDeployment.Spec.Template.Annotations).To(HaveKey(offloadingPatchHashAnnotation))
				g.Expect(vnDeployment.Spec.Template.Spec.Containers[0].Image).To(Equal("liqo/virtual-kubelet:test"))
			}, timeout, interval).Should(Succeed())
		})

		It("Check a VkOptionsTemplate change propagates to the existing deployment", func() {
			virtualNode1 = ForgeFakeVirtualNode(nameVirtualNode1, tenantNamespace1.Name, remoteClusterID1)
			Expect(k8sClient.Create(ctx, virtualNode1)).Should(Succeed())

			Eventually(func(g Gomega) {
				vnDeployment = getVnDeployment(g)
				g.Expect(vnDeployment.Spec.Template.Spec.Containers[0].Image).To(Equal("liqo/virtual-kubelet:test"))
			}, timeout, interval).Should(Succeed())

			By("Update the default VkOptionsTemplate image")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name: defaultVkOptsName, Namespace: liqoNamespace,
				}, defaultVkOptsTemplate)).To(Succeed())
				defaultVkOptsTemplate.Spec.ContainerImage = "liqo/virtual-kubelet:updated"
				g.Expect(k8sClient.Update(ctx, defaultVkOptsTemplate)).To(Succeed())
			}, timeout, interval).Should(Succeed())
			DeferCleanup(func() {
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, types.NamespacedName{
						Name: defaultVkOptsName, Namespace: liqoNamespace,
					}, defaultVkOptsTemplate)).To(Succeed())
					defaultVkOptsTemplate.Spec.ContainerImage = "liqo/virtual-kubelet:test"
					g.Expect(k8sClient.Update(ctx, defaultVkOptsTemplate)).To(Succeed())
				}, timeout, interval).Should(Succeed())
			})

			By("The deployment is updated accordingly, without touching the VirtualNode")
			Eventually(func(g Gomega) {
				vnDeployment = getVnDeployment(g)
				g.Expect(vnDeployment.Spec.Template.Spec.Containers[0].Image).To(Equal("liqo/virtual-kubelet:updated"))
			}, timeout, interval).Should(Succeed())
		})

		It("Check the legacy embedded template is removed and the deployment re-rendered", func() {
			virtualNode1 = ForgeFakeVirtualNode(nameVirtualNode1, tenantNamespace1.Name, remoteClusterID1)
			virtualNode1.Spec.CreateNode = ptr.To(true)
			virtualNode1.Spec.Template = &offloadingv1beta1.DeploymentTemplate{ //nolint:staticcheck // setting the deprecated field to test its removal
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vk-" + nameVirtualNode1,
					Namespace: tenantNamespace1.Name,
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: vkforge.VirtualKubeletLabels(virtualNode1),
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: vkforge.VirtualKubeletLabels(virtualNode1),
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{Name: "virtual-kubelet", Image: "legacy-image"}},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, virtualNode1)).Should(Succeed())

			By("The legacy embedded template is removed from the VirtualNode")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name: nameVirtualNode1, Namespace: tenantNamespace1.Name,
				}, virtualNode1)).To(Succeed())
				g.Expect(virtualNode1.Spec.Template).To(BeNil()) //nolint:staticcheck // checking the removal of the deprecated field
			}, timeout, interval).Should(Succeed())

			By("The deployment is re-rendered from the current VkOptionsTemplate")
			Eventually(func(g Gomega) {
				expected := &offloadingv1beta1.VkOptionsTemplate{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name: defaultVkOptsName, Namespace: liqoNamespace,
				}, expected)).To(Succeed())
				vnDeployment = getVnDeployment(g)
				g.Expect(vnDeployment.Spec.Template.Spec.Containers[0].Image).To(Equal(expected.Spec.ContainerImage))
				g.Expect(vnDeployment.Spec.Template.Spec.Containers[0].Image).ToNot(Equal("legacy-image"))
			}, timeout, interval).Should(Succeed())
		})

		It("Check the skip-vk-deployment annotation prevents the deployment creation", func() {
			virtualNode1 = ForgeFakeVirtualNode(nameVirtualNode1, tenantNamespace1.Name, remoteClusterID1)
			if virtualNode1.Annotations == nil {
				virtualNode1.Annotations = make(map[string]string)
			}
			virtualNode1.Annotations[liqoconst.SkipVkDeploymentAnnotation] = "true"
			Expect(k8sClient.Create(ctx, virtualNode1)).Should(Succeed())

			By("Manually cleaning up resources from previous tests (envtest has no garbage collector)")
			Eventually(func(g Gomega) {
				deployList := &appsv1.DeploymentList{}
				g.Expect(k8sClient.List(ctx, deployList, client.InNamespace(tenantNamespace1.Name),
					client.MatchingLabels{liqoconst.VirtualNodeLabel: nameVirtualNode1})).To(Succeed())
				for i := range deployList.Items {
					g.Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, &deployList.Items[i]))).To(Succeed())
				}

				sa := &corev1.ServiceAccount{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      vkforge.VirtualKubeletServiceAccountName(nameVirtualNode1),
					Namespace: tenantNamespace1.Name,
				}, sa)
				if err == nil {
					g.Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, sa))).To(Succeed())
				}
			}, timeout, interval).Should(Succeed())

			By("No deployment or ServiceAccount are created")
			// A reconcile of a previously deleted virtual-node sharing the same name may still be in
			// flight (the controller cache can briefly serve the pre-deletion object), transiently
			// recreating its ServiceAccount. Absence is therefore first awaited, and only then
			// checked for stability.
			expectVkResourcesAbsent := func(g Gomega) {
				deployList := &appsv1.DeploymentList{}
				g.Expect(k8sClient.List(ctx, deployList, client.InNamespace(tenantNamespace1.Name),
					client.MatchingLabels{liqoconst.VirtualNodeLabel: nameVirtualNode1})).To(Succeed())
				g.Expect(deployList.Items).To(BeEmpty())

				sa := &corev1.ServiceAccount{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      vkforge.VirtualKubeletServiceAccountName(nameVirtualNode1),
					Namespace: tenantNamespace1.Name,
				}, sa)
				g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}
			Eventually(expectVkResourcesAbsent, timeout, interval).Should(Succeed())
			Consistently(expectVkResourcesAbsent, timeout/2, interval).Should(Succeed())

			By("The NamespaceMap is still created")
			Eventually(func(g Gomega) {
				nmList := &offloadingv1beta1.NamespaceMapList{}
				g.Expect(k8sClient.List(ctx, nmList, client.InNamespace(tenantNamespace1.Name),
					client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterID1})).To(Succeed())
				g.Expect(nmList.Items).To(HaveLen(1))
			}, timeout, interval).Should(Succeed())

			By("Setting the annotation to false re-enables the deployment creation")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name: nameVirtualNode1, Namespace: tenantNamespace1.Name,
				}, virtualNode1)).To(Succeed())
				virtualNode1.Annotations[liqoconst.SkipVkDeploymentAnnotation] = "false"
				g.Expect(k8sClient.Update(ctx, virtualNode1)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				vnDeployment = getVnDeployment(g)
				g.Expect(vnDeployment.Spec.Template.Spec.Containers[0].Image).To(Equal("liqo/virtual-kubelet:test"))
			}, timeout, interval).Should(Succeed())
		})

		It("Check a custom VkOptionsTemplate and the not-reflected union are rendered", func() {
			customVkOpts := &offloadingv1beta1.VkOptionsTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "custom-vk-opts", Namespace: liqoNamespace},
				Spec: offloadingv1beta1.VkOptionsTemplateSpec{
					CreateNode:          true,
					DisableNetworkCheck: false,
					ContainerImage:      "liqo/virtual-kubelet:custom",
					LabelsNotReflected:  []string{"label-a"},
				},
			}
			Expect(k8sClient.Create(ctx, customVkOpts)).Should(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, customVkOpts)).Should(Succeed())
			})

			virtualNode1 = ForgeFakeVirtualNode(nameVirtualNode1, tenantNamespace1.Name, remoteClusterID1)
			virtualNode1.Spec.VkOptionsTemplateRef = &corev1.ObjectReference{
				Namespace: liqoNamespace,
				Name:      "custom-vk-opts",
			}
			virtualNode1.Spec.OffloadingPatch = &offloadingv1beta1.OffloadingPatch{
				LabelsNotReflected: []string{"label-b"},
			}
			// The spec value overrides the template one (createNode: true) in the effective values.
			virtualNode1.Spec.CreateNode = ptr.To(false)
			Expect(k8sClient.Create(ctx, virtualNode1)).Should(Succeed())

			By("The effective offloading patch is published in the status, merging spec and template")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name: nameVirtualNode1, Namespace: tenantNamespace1.Name,
				}, virtualNode1)).To(Succeed())
				g.Expect(virtualNode1.Status.EffectiveOffloadingPatch).NotTo(BeNil())
				g.Expect(virtualNode1.Status.EffectiveOffloadingPatch.LabelsNotReflected).To(
					Equal([]string{"label-b", "label-a"}))
				g.Expect(virtualNode1.Status.EffectiveCreateNode).To(HaveValue(BeFalse()))
				g.Expect(virtualNode1.Status.EffectiveDisableNetworkCheck).To(HaveValue(BeFalse()))
				// the spec patch is not modified by the controller (no write-backs)
				g.Expect(virtualNode1.Spec.OffloadingPatch.LabelsNotReflected).To(Equal([]string{"label-b"}))
			}, timeout, interval).Should(Succeed())

			By("The deployment is forged from the custom VkOptionsTemplate")
			Eventually(func(g Gomega) {
				vnDeployment = getVnDeployment(g)
				g.Expect(vnDeployment.Spec.Template.Spec.Containers[0].Image).To(Equal("liqo/virtual-kubelet:custom"))
			}, timeout, interval).Should(Succeed())
		})

	})

})
