package namespacectrl

import (
	"context"
	"fmt"

	mapsv1alpha1 "github.com/liqotech/liqo/apis/virtualKubelet/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"

	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/pkg/util/slice"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Namespace controller", func() {

	const (
		timeout         = time.Second * 10
		interval        = time.Millisecond * 250
		mappingLabel    = "mapping.liqo.io"
		offloadingLabel = "offloading.liqo.io"
	)

	ctx = context.TODO()
	nms = &mapsv1alpha1.NamespaceMapList{}

	Context("Adding some labels and checking state of NamespaceMaps", func() {

		BeforeEach(func() {

			By(fmt.Sprintf("Try to cleaning %s labels", nameNamespaceTest))
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: nameNamespaceTest}, namespace); err != nil {
					return false
				}
				delete(namespace.GetLabels(), mappingLabel)
				delete(namespace.GetLabels(), offloadingLabel)
				delete(namespace.GetLabels(), offloadingCluster1Label1)
				delete(namespace.GetLabels(), offloadingCluster1Label2)
				delete(namespace.GetLabels(), offloadingCluster2Label1)
				delete(namespace.GetLabels(), offloadingCluster2Label2)
				namespace.GetLabels()[randomLabel] = ""

				if err := k8sClient.Update(ctx, namespace); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf("Check if NamespaceMap associated to %s is cleaned by the controller", remoteClusterId1))
			Eventually(func() bool {
				if err := k8sClient.List(context.TODO(), nms, client.InNamespace(liqoconst.MapNamespaceName),
					client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterId1}); err != nil {
					return false
				}
				if len(nms.Items) != 1 {
					return false
				}
				By("Check Desired Mapping")
				if _, ok := nms.Items[0].Spec.DesiredMapping[nameNamespaceTest]; !ok {
					return true
				}
				return false
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf("Check if NamespaceMap associated to %s is cleaned by the controller ", remoteClusterId2))
			Eventually(func() bool {
				if err := k8sClient.List(context.TODO(), nms, client.InNamespace(liqoconst.MapNamespaceName),
					client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterId2}); err != nil {
					return false
				}
				if len(nms.Items) != 1 {
					return false
				}
				By("Check Desired Mapping")
				if _, ok := nms.Items[0].Spec.DesiredMapping[nameNamespaceTest]; !ok {
					return true
				}
				return false
			}, timeout, interval).Should(BeTrue())

		})

		It(fmt.Sprintf("Adding %s and %s , check presence of finalizer: %s",
			mappingLabel, offloadingLabel, namespaceControllerFinalizer), func() {

			By(fmt.Sprintf("Try to get Namespace: %s", nameNamespaceTest))
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: nameNamespaceTest}, namespace); err != nil {
					return false
				}
				namespace.GetLabels()[mappingLabel] = nameRemoteNamespace
				namespace.GetLabels()[offloadingLabel] = ""

				By(fmt.Sprintf("Adding labels to: %s", nameNamespaceTest))
				if err := k8sClient.Patch(ctx, namespace, client.Merge); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf("Try to get again: %s", nameNamespaceTest))
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: nameNamespaceTest}, namespace); err != nil {
					return false
				}
				return slice.ContainsString(namespace.GetFinalizers(), namespaceControllerFinalizer, nil)
			}, timeout, interval).Should(BeTrue())

		})

		It(fmt.Sprintf("Adding %s and %s , check NamespaceMaps desired mappings", mappingLabel, offloadingLabel), func() {

			By(fmt.Sprintf("Try to get Namespace: %s", nameNamespaceTest))
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: nameNamespaceTest}, namespace); err != nil {
					return false
				}
				namespace.GetLabels()[mappingLabel] = nameRemoteNamespace
				namespace.GetLabels()[offloadingLabel] = ""

				By(fmt.Sprintf("Adding labels to: %s", nameNamespaceTest))
				if err := k8sClient.Patch(ctx, namespace, client.Merge); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf("Try to get NamespaceMap associated to: %s", remoteClusterId1))
			Eventually(func() bool {
				if err := k8sClient.List(context.TODO(), nms, client.InNamespace(liqoconst.MapNamespaceName),
					client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterId1}); err != nil {
					return false
				}
				if len(nms.Items) != 1 {
					return false
				}
				By("Check Desired Mappings")
				if value, ok := nms.Items[0].Spec.DesiredMapping[nameNamespaceTest]; ok && value == nameRemoteNamespace {
					return true
				}
				return false
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf("Try to get NamespaceMap: associated to %s", remoteClusterId2))
			Eventually(func() bool {
				if err := k8sClient.List(context.TODO(), nms, client.InNamespace(liqoconst.MapNamespaceName),
					client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterId2}); err != nil {
					return false
				}
				if len(nms.Items) != 1 {
					return false
				}
				By("Check Desired Mappings")
				if value, ok := nms.Items[0].Spec.DesiredMapping[nameNamespaceTest]; ok && value == nameRemoteNamespace {
					return true
				}
				return false
			}, timeout, interval).Should(BeTrue())

		})

		It(fmt.Sprintf("Adding %s, %s and all labels for %s, check NamespaceMaps status",
			mappingLabel, offloadingLabel, nameVirtualNode1), func() {

			By(fmt.Sprintf("Try to get Namespace: %s", nameNamespaceTest))
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: nameNamespaceTest}, namespace); err != nil {
					return false
				}
				namespace.GetLabels()[mappingLabel] = nameRemoteNamespace
				namespace.GetLabels()[offloadingLabel] = ""
				namespace.GetLabels()[offloadingCluster1Label1] = ""
				namespace.GetLabels()[offloadingCluster1Label2] = ""

				By(fmt.Sprintf("Adding labels to: %s", nameNamespaceTest))
				if err := k8sClient.Patch(ctx, namespace, client.Merge); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf("Check if entry is also on NamespaceMap associated to: %s", remoteClusterId2))
			Eventually(func() bool {
				if err := k8sClient.List(context.TODO(), nms, client.InNamespace(liqoconst.MapNamespaceName), client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterId2}); err != nil {
					return false
				}
				if len(nms.Items) != 1 {
					return false
				}
				By("Check Desired Mappings")
				if value, ok := nms.Items[0].Spec.DesiredMapping[nameNamespaceTest]; ok && value == nameRemoteNamespace {
					return true
				}
				return false

			}, timeout, interval).Should(BeTrue())

		})

		It(fmt.Sprintf("Adding %s and all labels for %s, check NamespaceMaps status",
			mappingLabel, nameVirtualNode1), func() {

			By(fmt.Sprintf("Try to get Namespace: %s", nameNamespaceTest))
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: nameNamespaceTest}, namespace); err != nil {
					return false
				}
				namespace.GetLabels()[mappingLabel] = nameRemoteNamespace
				namespace.GetLabels()[offloadingCluster1Label1] = ""
				namespace.GetLabels()[offloadingCluster1Label2] = ""

				By(fmt.Sprintf("Adding labels to: %s", nameNamespaceTest))
				if err := k8sClient.Patch(ctx, namespace, client.Merge); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf("Try to get NamespaceMap associated to %s", remoteClusterId1))
			Eventually(func() bool {
				if err := k8sClient.List(context.TODO(), nms, client.InNamespace(liqoconst.MapNamespaceName), client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterId1}); err != nil {
					return false
				}
				if len(nms.Items) != 1 {
					return false
				}

				By("Check Desired Mappings")
				if value, ok := nms.Items[0].Spec.DesiredMapping[nameNamespaceTest]; ok && value == nameRemoteNamespace {
					return true
				}
				return false

			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf("Try to get NamespaceMap associated to %s", remoteClusterId2))
			Consistently(func() bool {
				if err := k8sClient.List(context.TODO(), nms, client.InNamespace(liqoconst.MapNamespaceName), client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterId2}); err != nil {
					return true
				}
				if len(nms.Items) != 1 {
					return true
				}

				By("Check Desired Mappings")
				if value, ok := nms.Items[0].Spec.DesiredMapping[nameNamespaceTest]; ok && value == nameRemoteNamespace {
					return true
				}
				return false
			}, timeout/5, interval).ShouldNot(BeTrue())

		})

		It(fmt.Sprintf("Update %s  value and check if label is automatically patched to old value",
			mappingLabel), func() {

			By(fmt.Sprintf("Try to get Namespace: %s", nameNamespaceTest))
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: nameNamespaceTest}, namespace); err != nil {
					return false
				}
				namespace.GetLabels()[mappingLabel] = nameRemoteNamespace
				namespace.GetLabels()[offloadingLabel] = ""

				By(fmt.Sprintf("Adding labels to: %s", nameNamespaceTest))
				if err := k8sClient.Patch(ctx, namespace, client.Merge); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf("Try to get NamespaceMap associated to: %s", remoteClusterId1))
			Eventually(func() bool {
				if err := k8sClient.List(context.TODO(), nms, client.InNamespace(liqoconst.MapNamespaceName),
					client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterId1}); err != nil {
					return false
				}
				if len(nms.Items) != 1 {
					return false
				}

				By("Check Desired Mappings")
				if value, ok := nms.Items[0].Spec.DesiredMapping[nameNamespaceTest]; ok && value == nameRemoteNamespace {
					return true
				}
				return false
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf("Try to get NamespaceMap associated to: %s", remoteClusterId2))
			Eventually(func() bool {
				if err := k8sClient.List(context.TODO(), nms, client.InNamespace(liqoconst.MapNamespaceName),
					client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterId2}); err != nil {
					return false
				}
				if len(nms.Items) != 1 {
					return false
				}

				By("Check status of NattingTable")
				if value, ok := nms.Items[0].Spec.DesiredMapping[nameNamespaceTest]; ok && value == nameRemoteNamespace {
					return true
				}
				return false
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf("Try to get again %s", nameNamespaceTest))
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: nameNamespaceTest}, namespace); err != nil {
					return false
				}
				namespace.GetLabels()[mappingLabel] = "new-value-remote-namespace-test"

				By(fmt.Sprintf("Update %s", mappingLabel))
				if err := k8sClient.Update(ctx, namespace); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf("Try to get %s and check if label %s value is the old one", nameNamespaceTest, mappingLabel))
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: nameNamespaceTest}, namespace); err != nil {
					return false
				}
				value, ok := namespace.GetLabels()[mappingLabel]
				return ok && value == nameRemoteNamespace
			}, timeout, interval).Should(BeTrue())

		})

	})

	Context("Check deletion of entries in NamespaceMaps", func() {

		BeforeEach(func() {

			By(fmt.Sprintf("Add labels to %s", nameNamespaceTest))
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: nameNamespaceTest}, namespace); err != nil {
					return false
				}
				namespace.GetLabels()[mappingLabel] = nameRemoteNamespace
				namespace.GetLabels()[offloadingLabel] = ""
				delete(namespace.GetLabels(), offloadingCluster1Label1)
				delete(namespace.GetLabels(), offloadingCluster1Label2)
				delete(namespace.GetLabels(), offloadingCluster2Label1)
				delete(namespace.GetLabels(), offloadingCluster2Label2)
				namespace.GetLabels()[randomLabel] = ""

				if err := k8sClient.Update(ctx, namespace); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf("Try to get NamespaceMap associated to: %s", remoteClusterId1))
			Eventually(func() bool {
				if err := k8sClient.List(context.TODO(), nms, client.InNamespace(liqoconst.MapNamespaceName),
					client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterId1}); err != nil {
					return false
				}
				if len(nms.Items) != 1 {
					return false
				}

				By("Check Desired Mappings")
				if value, ok := nms.Items[0].Spec.DesiredMapping[nameNamespaceTest]; ok && value == nameRemoteNamespace {
					return true
				}
				return false
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf("Try to get NamespaceMap associated to: %s", remoteClusterId2))
			Eventually(func() bool {
				if err := k8sClient.List(context.TODO(), nms, client.InNamespace(liqoconst.MapNamespaceName),
					client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterId2}); err != nil {
					return false
				}
				if len(nms.Items) != 1 {
					return false
				}

				By("Check Desired Mappings")
				if value, ok := nms.Items[0].Spec.DesiredMapping[nameNamespaceTest]; ok && value == nameRemoteNamespace {
					return true
				}
				return false
			}, timeout, interval).Should(BeTrue())

		})

		It(fmt.Sprintf("Remove %s label and check if NamespaceMaps are cleared", mappingLabel), func() {

			By(fmt.Sprintf("Try to remove %s label", mappingLabel))
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: nameNamespaceTest}, namespace); err != nil {
					return false
				}

				delete(namespace.GetLabels(), mappingLabel)
				if err := k8sClient.Update(ctx, namespace); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf("Check if NamespaceMap associated to %s is cleaned by the controller", remoteClusterId1))
			Eventually(func() bool {
				if err := k8sClient.List(context.TODO(), nms, client.InNamespace(liqoconst.MapNamespaceName),
					client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterId1}); err != nil {
					return false
				}
				if len(nms.Items) != 1 {
					return false
				}

				By("Check Desired Mappings")
				if _, ok := nms.Items[0].Spec.DesiredMapping[nameNamespaceTest]; !ok {
					return true
				}
				return false

			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf("Check if NamespaceMap associated to %s is cleaned by the controller", remoteClusterId2))
			Eventually(func() bool {
				if err := k8sClient.List(context.TODO(), nms, client.InNamespace(liqoconst.MapNamespaceName),
					client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterId2}); err != nil {
					return false
				}
				if len(nms.Items) != 1 {
					return false
				}

				By("Check Desired Mappings")
				if _, ok := nms.Items[0].Spec.DesiredMapping[nameNamespaceTest]; !ok {
					return true
				}
				return false

			}, timeout, interval).Should(BeTrue())

		})

		It(fmt.Sprintf("Remove %s and check if NamespaceMaps are cleared", nameNamespaceTest), func() {

			By("Check if namespace is really deleted")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: nameNamespaceTest}, namespace); err != nil {
					return false
				}

				if err := k8sClient.Delete(ctx, namespace); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf("Check if NamespaceMap associated to %s is cleaned by the controller", remoteClusterId1))
			Eventually(func() bool {
				if err := k8sClient.List(context.TODO(), nms, client.InNamespace(liqoconst.MapNamespaceName),
					client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterId1}); err != nil {
					return false
				}
				if len(nms.Items) != 1 {
					return false
				}

				By("Check Desired Mappings")
				if _, ok := nms.Items[0].Spec.DesiredMapping[nameNamespaceTest]; !ok {
					return true
				}
				return false

			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf("Check if NamespaceMap associated to %s is cleaned by the controller", remoteClusterId2))
			Eventually(func() bool {
				if err := k8sClient.List(context.TODO(), nms, client.InNamespace(liqoconst.MapNamespaceName),
					client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterId2}); err != nil {
					return false
				}
				if len(nms.Items) != 1 {
					return false
				}

				By("Check Desired Mappings")
				if _, ok := nms.Items[0].Spec.DesiredMapping[nameNamespaceTest]; !ok {
					return true
				}
				return false

			}, timeout, interval).Should(BeTrue())

		})

	})

})
