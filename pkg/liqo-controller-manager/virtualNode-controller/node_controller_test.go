package virtualNode_controller

import (
	"context"
	namespaceresourcesv1 "github.com/liqotech/liqo/apis/virtualKubelet/v1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"strings"
	"time"
)

var _ = Describe("VirtualNode controller", func() {

	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	var nm *namespaceresourcesv1.NamespaceMap

	Context("Check if resources VirtualNodes and NamespaceMaps are correctly initialized", func() {

		ctx = context.TODO()
		nm = &namespaceresourcesv1.NamespaceMap{}

		It("Check presence of NamespaceMaps and virtual nodes", func() {

			By("Try to get first namespaceMap with remoteClusterId: " + remoteClusterId1)
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: remoteClusterId1, Namespace: mapNamespaceName}, nm); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("Try to get first virtual-node: " + nameVirtualNode1)
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: nameVirtualNode1}, virtualNode1); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("Try to get second namespaceMap with remoteClusterId: " + remoteClusterId2)
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: remoteClusterId2, Namespace: mapNamespaceName}, nm); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("Try to get second virtual-node: " + nameVirtualNode2)
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: nameVirtualNode2}, virtualNode2); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

		})

		It("Check if finalizers and ownerReference are correctly created for "+nameVirtualNode1, func() {

			By("Try to get first virtual-node: " + nameVirtualNode1)
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: nameVirtualNode1}, virtualNode1); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("Try to get first namespaceMap with remoteClusterId: " + remoteClusterId1)
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: remoteClusterId1, Namespace: mapNamespaceName}, nm); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			expectedOwnerReference := v1.OwnerReference{
				APIVersion:         "v1",
				BlockOwnerDeletion: pointer.BoolPtr(true),
				Kind:               "Node",
				Name:               virtualNode1.GetName(),
				UID:                virtualNode1.GetUID(),
				Controller:         pointer.BoolPtr(true),
			}

			By("Try to check ownership of NamespaceMap: " + nm.GetName())
			Expect(nm.GetOwnerReferences()).To(ContainElement(expectedOwnerReference))

			By("Try to check presence of finalizer in NamespaceMap: " + nm.GetName())
			Expect(nm.GetFinalizers()).To(ContainElement(namespaceMapFinalizer))

			By("Try to check presence of finalizer in VirtualNode: " + virtualNode1.GetName())
			// i have to update my node instance, because finalizer could be updated after my first get
			// eventually because I may get namespaceMap more than one time in order to catch the update
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: nameVirtualNode1}, virtualNode1); err != nil {
					return false
				}
				return containsString(virtualNode1.GetFinalizers(), virtualNodeFinalizer)
			}, timeout, interval).Should(BeTrue())

		})

		It("Check if finalizers and ownerReference are correctly created for "+nameVirtualNode2, func() {

			By("Try to get second virtual-node: " + nameVirtualNode2)
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: nameVirtualNode2}, virtualNode2); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("Try to get second namespaceMap with remoteClusterId: " + remoteClusterId2)
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: remoteClusterId2, Namespace: mapNamespaceName}, nm); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			expectedOwnerReference := v1.OwnerReference{
				APIVersion:         "v1",
				BlockOwnerDeletion: pointer.BoolPtr(true),
				Kind:               "Node",
				Name:               virtualNode2.GetName(),
				UID:                virtualNode2.GetUID(),
				Controller:         pointer.BoolPtr(true),
			}

			By("Try to check ownership of NamespaceMap: " + nm.GetName())
			Expect(nm.GetOwnerReferences()).To(ContainElement(expectedOwnerReference))

			By("Try to check presence of finalizer in NamespaceMap: " + nm.GetName())
			Expect(nm.GetFinalizers()).To(ContainElement(namespaceMapFinalizer))

			By("Try to check presence of finalizer in VirtualNode: " + virtualNode2.GetName())
			// i have to update my node instance, because finalizer could be updated after my first get
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: nameVirtualNode2}, virtualNode2); err != nil {
					return false
				}
				return containsString(virtualNode2.GetFinalizers(), virtualNodeFinalizer)
			}, timeout, interval).Should(BeTrue())

		})

	})

	Context("Check if finalizers on nodes working ", func() {

		ctx = context.TODO()

		BeforeEach(func() {
			buffer.Reset()
		})

		It("Check deletion of first virtualNode: "+nameVirtualNode1, func() {

			By("Try to get first virtual-node: " + nameVirtualNode1)
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: nameVirtualNode1}, virtualNode1); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("Try to delete virtual-node: " + virtualNode1.GetName())
			Expect(k8sClient.Delete(ctx, virtualNode1)).Should(Succeed())

			By("Try to catch right log: ")
			Eventually(func() bool {
				return strings.Contains(buffer.String(), "Someone try to delete virtual node, ok delete!!")
			}, timeout, interval).Should(BeTrue())

			By("Try to get if virtual-node " + nameVirtualNode1 + " is removed")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: nameVirtualNode1}, virtualNode1); err != nil {
					if errors.IsNotFound(err) {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())
		})

		It("Check deletion of second virtualNode: "+nameVirtualNode2, func() {

			By("Try to get second virtual-node: " + nameVirtualNode2)
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: nameVirtualNode2}, virtualNode2); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("Try to delete virtual-node: " + virtualNode2.GetName())
			Expect(k8sClient.Delete(ctx, virtualNode2)).Should(Succeed())

			By("Try to catch right log: ")
			Eventually(func() bool {
				return strings.Contains(buffer.String(), "Someone try to delete virtual node, ok delete!!")
			}, timeout, interval).Should(BeTrue())

			By("Try to get if virtual-node " + nameVirtualNode2 + " is removed")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: nameVirtualNode2}, virtualNode2); err != nil {
					if errors.IsNotFound(err) {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())
		})

	})

	Context("Check if a non-virtual node is monitored", func() {

		ctx = context.TODO()
		nm = &namespaceresourcesv1.NamespaceMap{}

		It("Check absence of NamespaceMaps and of finalizer", func() {

			By("Try to get not virtual-node: " + nameSimpleNode)
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: nameSimpleNode}, simpleNode); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("Check absence of NamespaceMap: " + remoteClusterIdSimpleNode)
			Consistently(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: remoteClusterIdSimpleNode, Namespace: mapNamespaceName}, nm); err != nil {
					if errors.IsNotFound(err) {
						return true
					}
				}
				return false
			}, timeout/5, interval).Should(BeTrue())

			By("Check absence of finalizer: " + virtualNodeFinalizer)
			Consistently(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: nameSimpleNode}, simpleNode); err != nil {
					return true
				}
				return containsString(simpleNode.GetFinalizers(), virtualNodeFinalizer)
			}, timeout/5, interval).ShouldNot(BeTrue())

		})

	})

})
