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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	vkforge "github.com/liqotech/liqo/pkg/vkMachinery/forge"
)

func ForgeFakeConfiguration(name, namespace string, remoteClusterID liqov1beta1.ClusterID,
	podSpec, podRemap, extSpec, extRemap []networkingv1beta1.CIDR) *networkingv1beta1.Configuration {
	return &networkingv1beta1.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				liqoconst.RemoteClusterID: string(remoteClusterID),
			},
		},
		Spec: networkingv1beta1.ConfigurationSpec{
			Remote: networkingv1beta1.ClusterConfig{
				CIDR: networkingv1beta1.ClusterConfigCIDR{
					Pod:      podSpec,
					External: extSpec,
				},
			},
		},
		Status: networkingv1beta1.ConfigurationStatus{
			Remote: &networkingv1beta1.ClusterConfig{
				CIDR: networkingv1beta1.ClusterConfigCIDR{
					Pod:      podRemap,
					External: extRemap,
				},
			},
			Conditions: []metav1.Condition{
				{
					Type:               networkingv1beta1.ConfigurationConditionNetworkCIDRsConfigured,
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.Now(),
					Reason:             "ConfigurationReady",
					Message:            "Network CIDRs configured",
				},
			},
		},
	}
}

func hasArg(args []string, arg string) bool {
	for i := range args {
		if args[i] == arg {
			return true
		}
	}
	return false
}

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
			By("Creating the network configurations")
			cfg1 := ForgeFakeConfiguration("cfg-vn-1", tenantNamespace1.Name, remoteClusterID1,
				[]networkingv1beta1.CIDR{"10.0.0.0/16"},
				[]networkingv1beta1.CIDR{"192.168.0.0/16"},
				[]networkingv1beta1.CIDR{"172.16.0.0/16"},
				[]networkingv1beta1.CIDR{"10.1.0.0/16"})
			Expect(k8sClient.Create(ctx, cfg1)).Should(Succeed())
			cfg1.Status.Remote = &networkingv1beta1.ClusterConfig{
				CIDR: networkingv1beta1.ClusterConfigCIDR{
					Pod:      []networkingv1beta1.CIDR{"192.168.0.0/16"},
					External: []networkingv1beta1.CIDR{"10.1.0.0/16"},
				},
			}
			cfg1.Status.Conditions = []metav1.Condition{
				{
					Type:               networkingv1beta1.ConfigurationConditionNetworkCIDRsConfigured,
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.Now(),
					Reason:             "ConfigurationReady",
					Message:            "Network CIDRs configured",
				},
			}
			Expect(k8sClient.Status().Update(ctx, cfg1)).Should(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, cfg1) })

			cfg2 := ForgeFakeConfiguration("cfg-vn-2", tenantNamespace2.Name, remoteClusterID2,
				[]networkingv1beta1.CIDR{"10.10.0.0/16"},
				[]networkingv1beta1.CIDR{"192.170.0.0/16"},
				[]networkingv1beta1.CIDR{"172.20.0.0/16"},
				[]networkingv1beta1.CIDR{"10.11.0.0/16"})
			Expect(k8sClient.Create(ctx, cfg2)).Should(Succeed())
			cfg2.Status.Remote = &networkingv1beta1.ClusterConfig{
				CIDR: networkingv1beta1.ClusterConfigCIDR{
					Pod:      []networkingv1beta1.CIDR{"192.170.0.0/16"},
					External: []networkingv1beta1.CIDR{"10.11.0.0/16"},
				},
			}
			cfg2.Status.Conditions = []metav1.Condition{
				{
					Type:               networkingv1beta1.ConfigurationConditionNetworkCIDRsConfigured,
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.Now(),
					Reason:             "ConfigurationReady",
					Message:            "Network CIDRs configured",
				},
			}
			Expect(k8sClient.Status().Update(ctx, cfg2)).Should(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, cfg2) })

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
			cfg := ForgeFakeConfiguration("cfg-lifecycle", tenantNamespace1.Name, remoteClusterID1,
				[]networkingv1beta1.CIDR{"10.0.0.0/16"},
				[]networkingv1beta1.CIDR{"192.168.0.0/16"},
				[]networkingv1beta1.CIDR{"172.16.0.0/16"},
				[]networkingv1beta1.CIDR{"10.1.0.0/16"})
			Expect(k8sClient.Create(ctx, cfg)).Should(Succeed())
			cfg.Status.Remote = &networkingv1beta1.ClusterConfig{
				CIDR: networkingv1beta1.ClusterConfigCIDR{
					Pod:      []networkingv1beta1.CIDR{"192.168.0.0/16"},
					External: []networkingv1beta1.CIDR{"10.1.0.0/16"},
				},
			}
			cfg.Status.Conditions = []metav1.Condition{
				{
					Type:               networkingv1beta1.ConfigurationConditionNetworkCIDRsConfigured,
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.Now(),
					Reason:             "ConfigurationReady",
					Message:            "Network CIDRs configured",
				},
			}
			Expect(k8sClient.Status().Update(ctx, cfg)).Should(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, cfg) })

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

	Context("Check network configuration args are propagated to the virtual-kubelet deployment", func() {
		It("should inject Configuration CIDRs into the VK Deployment args", func() {
			vn := ForgeFakeVirtualNode("vn-with-config", tenantNamespace1.Name, remoteClusterID1)
			By("Creating the network configuration")
			cfg := ForgeFakeConfiguration("cfg-vn-with-config", tenantNamespace1.Name, remoteClusterID1,
				[]networkingv1beta1.CIDR{"10.0.0.0/16"},
				[]networkingv1beta1.CIDR{"192.168.0.0/16"},
				[]networkingv1beta1.CIDR{"172.16.0.0/16"},
				[]networkingv1beta1.CIDR{"10.1.0.0/16"})
			Expect(k8sClient.Create(ctx, cfg)).Should(Succeed())
			cfg.Status.Remote = &networkingv1beta1.ClusterConfig{
				CIDR: networkingv1beta1.ClusterConfigCIDR{
					Pod:      []networkingv1beta1.CIDR{"192.168.0.0/16"},
					External: []networkingv1beta1.CIDR{"10.1.0.0/16"},
				},
			}
			cfg.Status.Conditions = []metav1.Condition{
				{
					Type:               networkingv1beta1.ConfigurationConditionNetworkCIDRsConfigured,
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.Now(),
					Reason:             "ConfigurationReady",
					Message:            "Network CIDRs configured",
				},
			}
			Expect(k8sClient.Status().Update(ctx, cfg)).Should(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, cfg) })

			By("Creating the virtual-node")
			Expect(k8sClient.Create(ctx, vn)).Should(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, vn) })

			By("Waiting for the deployment to contain the network args")
			Eventually(func(g Gomega) []string {
				dep := &appsv1.Deployment{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: vn.Name, Namespace: vn.Namespace}, dep)).To(Succeed())
				g.Expect(dep.Spec.Template.Spec.Containers).ToNot(BeEmpty())
				return dep.Spec.Template.Spec.Containers[0].Args
			}, timeout, interval).Should(And(
				ContainElement(string(vkforge.RemotePodCIDR)+"=10.0.0.0/16"),
				ContainElement(string(vkforge.RemotePodCIDRRemap)+"=192.168.0.0/16"),
				ContainElement(string(vkforge.RemoteExternalCIDR)+"=172.16.0.0/16"),
				ContainElement(string(vkforge.RemoteExternalCIDRRemap)+"=10.1.0.0/16"),
			))
		})

		It("should update the VK Deployment args when the Configuration changes", func() {
			vn := ForgeFakeVirtualNode("vn-config-update", tenantNamespace2.Name, remoteClusterID2)
			By("Creating the network configuration")
			cfg := ForgeFakeConfiguration("cfg-vn-config-update", tenantNamespace2.Name, remoteClusterID2,
				[]networkingv1beta1.CIDR{"10.0.0.0/16"},
				[]networkingv1beta1.CIDR{"192.168.0.0/16"},
				[]networkingv1beta1.CIDR{"172.16.0.0/16"},
				[]networkingv1beta1.CIDR{"10.1.0.0/16"})
			Expect(k8sClient.Create(ctx, cfg)).Should(Succeed())
			cfg.Status.Remote = &networkingv1beta1.ClusterConfig{
				CIDR: networkingv1beta1.ClusterConfigCIDR{
					Pod:      []networkingv1beta1.CIDR{"192.168.0.0/16"},
					External: []networkingv1beta1.CIDR{"10.1.0.0/16"},
				},
			}
			cfg.Status.Conditions = []metav1.Condition{
				{
					Type:               networkingv1beta1.ConfigurationConditionNetworkCIDRsConfigured,
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.Now(),
					Reason:             "ConfigurationReady",
					Message:            "Network CIDRs configured",
				},
			}
			Expect(k8sClient.Status().Update(ctx, cfg)).Should(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, cfg) })

			By("Creating the virtual-node")
			Expect(k8sClient.Create(ctx, vn)).Should(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, vn) })

			By("Waiting for the initial network args")
			Eventually(func(g Gomega) []string {
				dep := &appsv1.Deployment{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: vn.Name, Namespace: vn.Namespace}, dep)).To(Succeed())
				g.Expect(dep.Spec.Template.Spec.Containers).ToNot(BeEmpty())
				return dep.Spec.Template.Spec.Containers[0].Args
			}, timeout, interval).Should(ContainElement(string(vkforge.RemotePodCIDR) + "=10.0.0.0/16"))

			By("Updating the configuration remapped CIDRs")
			cfg.Status.Remote.CIDR.Pod = []networkingv1beta1.CIDR{"193.168.0.0/16"}
			Expect(k8sClient.Status().Update(ctx, cfg)).Should(Succeed())

			By("Waiting for the deployment args to be updated")
			Eventually(func(g Gomega) []string {
				dep := &appsv1.Deployment{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: vn.Name, Namespace: vn.Namespace}, dep)).To(Succeed())
				g.Expect(dep.Spec.Template.Spec.Containers).ToNot(BeEmpty())
				return dep.Spec.Template.Spec.Containers[0].Args
			}, timeout, interval).Should(And(
				ContainElement(string(vkforge.RemotePodCIDRRemap)+"=193.168.0.0/16"),
				Not(ContainElement(string(vkforge.RemotePodCIDRRemap)+"=192.168.0.0/16")),
			))
		})
	})

})
