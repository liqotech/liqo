package virtualNode_controller

import (
	mapsv1alpha1 "github.com/liqotech/liqo/apis/virtualKubelet/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"

	"context"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/pkg/util/slice"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	testLabel = "namespace1"
	testValue = "namespace1-remote"
)

var _ = Describe("VirtualNode controller", func() {

	nms = &mapsv1alpha1.NamespaceMapList{}

	Context("Check if resources VirtualNodes and NamespaceMaps are correctly initialized", func() {

		It("Check presence of NamespaceMaps and virtual nodes", func() {

			By(fmt.Sprintf("Try to get NamespaceMap associated to: %s", remoteClusterId1))
			Eventually(func() bool {
				if err := k8sClient.List(context.TODO(), nms, client.InNamespace(liqoconst.MapNamespaceName), client.MatchingLabels{liqoconst.VirtualNodeClusterId: remoteClusterId1}); err != nil {
					return false
				}
				if len(nms.Items) != 1 {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf("Try to get virtual-node: %s", nameVirtualNode1))
			Eventually(func() bool {
				if err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: nameVirtualNode1}, virtualNode1); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf("Try to get NamespaceMap associated to: %s", remoteClusterId2))
			Eventually(func() bool {
				if err := k8sClient.List(context.TODO(), nms, client.InNamespace(liqoconst.MapNamespaceName), client.MatchingLabels{liqoconst.VirtualNodeClusterId: remoteClusterId2}); err != nil {
					return false
				}
				if len(nms.Items) != 1 {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf("Try to get virtual-node: %s", nameVirtualNode2))
			Eventually(func() bool {
				if err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: nameVirtualNode2}, virtualNode2); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

		})

		It(fmt.Sprintf("Check if finalizers and ownerReference are correctly created for %s", nameVirtualNode1), func() {

			By(fmt.Sprintf("Try to get virtual-node: %s", nameVirtualNode1))
			Eventually(func() bool {
				if err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: nameVirtualNode1}, virtualNode1); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf("Try to get NamespaceMap associated to: %s", remoteClusterId1))
			Eventually(func() bool {
				if err := k8sClient.List(context.TODO(), nms, client.InNamespace(liqoconst.MapNamespaceName), client.MatchingLabels{liqoconst.VirtualNodeClusterId: remoteClusterId1}); err != nil {
					return false
				}
				if len(nms.Items) != 1 {
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

			By(fmt.Sprintf("Try to check ownership of NamespaceMap: %s", nms.Items[0].GetName()))
			Expect(nms.Items[0].GetOwnerReferences()).To(ContainElement(expectedOwnerReference))

			By(fmt.Sprintf("Try to check presence of finalizer in NamespaceMap: %s", nms.Items[0].GetName()))
			Expect(nms.Items[0].GetFinalizers()).To(ContainElement(virtualNodeControllerFinalizer))

			By(fmt.Sprintf("Try to check presence of finalizer in VirtualNode: %s", virtualNode1.GetName()))
			// i have to update my node instance, because finalizer could be updated after my first get
			Eventually(func() bool {
				if err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: nameVirtualNode1}, virtualNode1); err != nil {
					return false
				}
				return slice.ContainsString(virtualNode1.GetFinalizers(), virtualNodeControllerFinalizer, nil)
			}, timeout, interval).Should(BeTrue())

		})

		It(fmt.Sprintf("Check if finalizers and ownerReference are correctly created for %s", nameVirtualNode2), func() {

			By(fmt.Sprintf("Try to get virtual-node: %s", nameVirtualNode2))
			Eventually(func() bool {
				if err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: nameVirtualNode2}, virtualNode2); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf("Try to get NamespaceMap associated to: %s", remoteClusterId2))
			Eventually(func() bool {
				if err := k8sClient.List(context.TODO(), nms, client.InNamespace(liqoconst.MapNamespaceName), client.MatchingLabels{liqoconst.VirtualNodeClusterId: remoteClusterId2}); err != nil {
					return false
				}
				if len(nms.Items) != 1 {
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

			By(fmt.Sprintf("Try to check ownership of NamespaceMap: %s", nms.Items[0].GetName()))
			Expect(nms.Items[0].GetOwnerReferences()).To(ContainElement(expectedOwnerReference))

			By(fmt.Sprintf("Try to check presence of finalizer in NamespaceMap: %s", nms.Items[0].GetName()))
			Expect(nms.Items[0].GetFinalizers()).To(ContainElement(virtualNodeControllerFinalizer))

			By(fmt.Sprintf("Try to check presence of finalizer in VirtualNode: %s", virtualNode2.GetName()))
			// i have to update my node instance, because finalizer could be updated after my first get
			Eventually(func() bool {
				if err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: nameVirtualNode2}, virtualNode2); err != nil {
					return false
				}
				return slice.ContainsString(virtualNode2.GetFinalizers(), virtualNodeControllerFinalizer, nil)
			}, timeout, interval).Should(BeTrue())

		})

	})

	Context("Check if a non-virtual node is monitored", func() {

		It("Check absence of NamespaceMap and of finalizer", func() {

			By(fmt.Sprintf("Try to get not virtual-node: %s", nameSimpleNode))
			Eventually(func() bool {
				if err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: nameSimpleNode}, simpleNode); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf("Check absence of NamespaceMap associated to %s", remoteClusterIdSimpleNode))
			Consistently(func() bool {
				if err := k8sClient.List(context.TODO(), nms, client.InNamespace(liqoconst.MapNamespaceName), client.MatchingLabels{liqoconst.VirtualNodeClusterId: remoteClusterIdSimpleNode}); err != nil {
					return false
				}
				if len(nms.Items) == 0 {
					return true
				}
				return false
			}, timeout/5, interval).Should(BeTrue())

			By(fmt.Sprintf("Check absence of finalizer %s: ", virtualNodeControllerFinalizer))
			Consistently(func() bool {
				if err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: nameSimpleNode}, simpleNode); err != nil {
					return true
				}
				return slice.ContainsString(simpleNode.GetFinalizers(), virtualNodeControllerFinalizer, nil)
			}, timeout/5, interval).ShouldNot(BeTrue())

		})

	})

	Context("Check deletion lifecycle of Namespacemaps and virtual-nodes ", func() {

		BeforeEach(func() {
			buffer.Reset()
		})

		It(fmt.Sprintf("Check regeneration of NamespaceMap associated to %s", remoteClusterId1), func() {

			oldName := ""
			By(fmt.Sprintf("Try to delete NamespaceMap associated to: %s", remoteClusterId1))
			Eventually(func() bool {
				if err := k8sClient.List(context.TODO(), nms, client.MatchingLabels{liqoconst.VirtualNodeClusterId: remoteClusterId1}); err != nil {
					return false
				}
				if len(nms.Items) != 1 {
					return false
				}

				if nms.Items[0].Spec.DesiredMapping == nil {
					nms.Items[0].Spec.DesiredMapping = map[string]string{}
				}

				// random state
				if nms.Items[0].Status.CurrentMapping == nil {
					nms.Items[0].Status.CurrentMapping = map[string]mapsv1alpha1.RemoteNamespaceStatus{}
				}
				nms.Items[0].Spec.DesiredMapping[testLabel] = testValue
				nms.Items[0].Status.CurrentMapping[testLabel] = mapsv1alpha1.RemoteNamespaceStatus{
					RemoteNamespace: testValue,
					Phase:           mapsv1alpha1.MappingAccepted,
				}

				By(fmt.Sprintf("Try to update NamespaceMap: %s", nms.Items[0].GetName()))
				if err := k8sClient.Update(context.TODO(), &nms.Items[0]); err != nil {
					return false
				}

				oldName = nms.Items[0].GetName()
				By(fmt.Sprintf("Try to delete NamespaceMap: %s", nms.Items[0].GetName()))
				if err := k8sClient.Delete(context.TODO(), &nms.Items[0]); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf("Try to get new NamespaceMap associated to: %s", remoteClusterId1))
			Eventually(func() bool {
				if err := k8sClient.List(context.TODO(), nms, client.InNamespace(liqoconst.MapNamespaceName), client.MatchingLabels{liqoconst.VirtualNodeClusterId: remoteClusterId1}); err != nil {
					return false
				}
				if len(nms.Items) != 1 {
					return false
				}
				if oldName != nms.Items[0].GetName() && nms.Items[0].Spec.DesiredMapping[testLabel] == testValue &&
					nms.Items[0].Status.CurrentMapping[testLabel].RemoteNamespace == testValue && nms.Items[0].Status.CurrentMapping[testLabel].Phase == mapsv1alpha1.MappingAccepted {
					return true
				}
				return false
			}, timeout, interval).Should(BeTrue())

		})

		It(fmt.Sprintf("Check regeneration of NamespaceMap associated to %s", remoteClusterId2), func() {

			oldName := ""
			By(fmt.Sprintf("Try to delete NamespaceMap associated to: %s", remoteClusterId2))
			Eventually(func() bool {
				if err := k8sClient.List(context.TODO(), nms, client.InNamespace(liqoconst.MapNamespaceName), client.MatchingLabels{liqoconst.VirtualNodeClusterId: remoteClusterId2}); err != nil {
					return false
				}
				if len(nms.Items) != 1 {
					return false
				}

				if nms.Items[0].Spec.DesiredMapping == nil {
					nms.Items[0].Spec.DesiredMapping = map[string]string{}
				}

				if nms.Items[0].Status.CurrentMapping == nil {
					nms.Items[0].Status.CurrentMapping = map[string]mapsv1alpha1.RemoteNamespaceStatus{}
				}
				nms.Items[0].Spec.DesiredMapping[testLabel] = testValue
				nms.Items[0].Status.CurrentMapping[testLabel] = mapsv1alpha1.RemoteNamespaceStatus{
					RemoteNamespace: testValue,
					Phase:           mapsv1alpha1.MappingAccepted,
				}

				By(fmt.Sprintf("Try to update NamespaceMap: %s", nms.Items[0].GetName()))
				if err := k8sClient.Update(context.TODO(), &nms.Items[0]); err != nil {
					return false
				}

				oldName = nms.Items[0].GetName()
				By(fmt.Sprintf("Try to delete NamespaceMap: %s", nms.Items[0].GetName()))
				if err := k8sClient.Delete(context.TODO(), &nms.Items[0]); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf("Try to get new NamespaceMap associated to: %s", remoteClusterId2))
			Eventually(func() bool {
				if err := k8sClient.List(context.TODO(), nms, client.InNamespace(liqoconst.MapNamespaceName), client.MatchingLabels{liqoconst.VirtualNodeClusterId: remoteClusterId2}); err != nil {
					return false
				}
				if len(nms.Items) != 1 {
					return false
				}
				if oldName != nms.Items[0].GetName() && nms.Items[0].Spec.DesiredMapping[testLabel] == testValue &&
					nms.Items[0].Status.CurrentMapping[testLabel].RemoteNamespace == testValue && nms.Items[0].Status.CurrentMapping[testLabel].Phase == mapsv1alpha1.MappingAccepted {
					return true
				}
				return false
			}, timeout, interval).Should(BeTrue())

		})

		It(fmt.Sprintf("Check deletion of virtualNode: %s", nameVirtualNode1), func() {

			By(fmt.Sprintf("Try to delete virtual-node: %s", nameVirtualNode1))
			Eventually(func() bool {
				if err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: nameVirtualNode1}, virtualNode1); err != nil {
					return false
				}
				if err := k8sClient.Delete(context.TODO(), virtualNode1); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("Try to catch right virtual-node log: ")
			Eventually(func() bool {
				return strings.Contains(buffer.String(), fmt.Sprintf("The virtual node '%s' is requested to be deleted", nameVirtualNode1))
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf("Try to get if virtual-node %s is removed", nameVirtualNode1))
			Eventually(func() bool {
				if err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: nameVirtualNode1}, virtualNode1); err != nil {
					if errors.IsNotFound(err) {
						return true
					}
				}
				return false
			}, timeout*2, interval).Should(BeTrue())
		})

		It(fmt.Sprintf("Check deletion of virtualNode: %s", nameVirtualNode2), func() {

			By(fmt.Sprintf("Try to delete virtual-node: %s", nameVirtualNode2))
			Eventually(func() bool {
				if err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: nameVirtualNode2}, virtualNode2); err != nil {
					return false
				}
				if err := k8sClient.Delete(context.TODO(), virtualNode2); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("Try to catch right virtual-node log: ")
			Eventually(func() bool {
				return strings.Contains(buffer.String(), fmt.Sprintf("The virtual node '%s' is requested to be deleted", nameVirtualNode2))
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf("Try to get if virtual-node %s is removed", nameVirtualNode2))
			Eventually(func() bool {
				if err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: nameVirtualNode2}, virtualNode2); err != nil {
					if errors.IsNotFound(err) {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())
		})

	})

})
