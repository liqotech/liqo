package namespace_controller

import (
	"context"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/pkg/util/slice"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

var _ = Describe("Namespace controller", func() {

	const (
		timeout         = time.Second * 10
		interval        = time.Millisecond * 250
		mappingLabel    = "mapping.liqo.io"
		offloadingLabel = "offloading.liqo.io"
	)

	ctx = context.TODO()

	BeforeEach(func() {

		By("Try to cleaning " + nameNamespaceTest + " labels")
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

		By("Check if NamespaceMap " + remoteClusterId1 + " is cleaned by the controller")
		Eventually(func() bool {
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: remoteClusterId1, Namespace: mapNamespaceName}, nm1); err != nil {
				return false
			}
			if nm1.Status.NattingTable == nil && nm1.Status.DeNattingTable == nil {
				return true
			}

			if _, ok := nm1.Status.NattingTable[nameNamespaceTest]; !ok {
				if _, ok = nm1.Status.DeNattingTable[nameRemoteNamespace]; !ok {
					return true
				}
			}
			return false
		}, timeout, interval).Should(BeTrue())

		By("Check if NamespaceMap " + remoteClusterId2 + " is cleaned by the controller ")
		Eventually(func() bool {
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: remoteClusterId2, Namespace: mapNamespaceName}, nm2); err != nil {
				return false
			}
			if nm2.Status.NattingTable == nil && nm2.Status.DeNattingTable == nil {
				return true
			}

			if _, ok := nm2.Status.NattingTable[nameNamespaceTest]; !ok {
				if _, ok = nm2.Status.DeNattingTable[nameRemoteNamespace]; !ok {
					return true
				}
			}
			return false
		}, timeout, interval).Should(BeTrue())

	})

	Context("Adding some labels and checking state of NamespaceMaps", func() {

		It("Adding "+mappingLabel+" and "+offloadingLabel+" , check presence of finalizer: "+namespaceFinalizer, func() {

			By("Try to get Namespace: " + nameNamespaceTest)
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: nameNamespaceTest}, namespace); err != nil {
					return false
				}
				namespace.GetLabels()[mappingLabel] = nameRemoteNamespace
				namespace.GetLabels()[offloadingLabel] = ""

				By("Adding labels to: " + nameNamespaceTest)
				if err := k8sClient.Patch(ctx, namespace, client.Merge); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("Try to get again: " + nameNamespaceTest)
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: nameNamespaceTest}, namespace); err != nil {
					return false
				}
				return slice.ContainsString(namespace.GetFinalizers(), namespaceFinalizer, nil)
			}, timeout, interval).Should(BeTrue())

		})

		It("Adding "+mappingLabel+" and "+offloadingLabel+" , check NamespaceMaps status", func() {

			By("Try to get Namespace: " + nameNamespaceTest)
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: nameNamespaceTest}, namespace); err != nil {
					return false
				}
				namespace.GetLabels()[mappingLabel] = nameRemoteNamespace
				namespace.GetLabels()[offloadingLabel] = ""

				By("Adding labels to: " + nameNamespaceTest)
				if err := k8sClient.Patch(ctx, namespace, client.Merge); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("Try to get NamespaceMap: " + remoteClusterId1)
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: remoteClusterId1, Namespace: mapNamespaceName}, nm1); err != nil {
					return false
				}
				value1, ok1 := nm1.Status.NattingTable[nameNamespaceTest]
				value2, ok2 := nm1.Status.DeNattingTable[nameRemoteNamespace]

				By("Check status of NattingTable and DeNattingTable ")
				if ok1 && ok2 && value1 == nameRemoteNamespace && value2 == nameNamespaceTest {
					return true
				}
				return false
			}, timeout, interval).Should(BeTrue())

			By("Try to get NamespaceMap: " + remoteClusterId2)
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: remoteClusterId2, Namespace: mapNamespaceName}, nm2); err != nil {
					return false
				}
				value1, ok1 := nm2.Status.NattingTable[nameNamespaceTest]
				value2, ok2 := nm2.Status.DeNattingTable[nameRemoteNamespace]

				By("Check status of NattingTable and DeNattingTable ")
				if ok1 && ok2 && value1 == nameRemoteNamespace && value2 == nameNamespaceTest {
					return true
				}
				return false
			}, timeout, interval).Should(BeTrue())

		})

		It("Adding "+mappingLabel+" and "+offloadingLabel+" and particular labels for "+nameVirtualNode1+" , check NamespaceMaps status", func() {

			By("Try to get Namespace: " + nameNamespaceTest)
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: nameNamespaceTest}, namespace); err != nil {
					return false
				}
				namespace.GetLabels()[mappingLabel] = nameRemoteNamespace
				namespace.GetLabels()[offloadingLabel] = ""
				namespace.GetLabels()[offloadingCluster1Label1] = ""
				namespace.GetLabels()[offloadingCluster1Label2] = ""

				By("Adding labels to: " + nameNamespaceTest)
				if err := k8sClient.Patch(ctx, namespace, client.Merge); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("Try to get NamespaceMap: " + remoteClusterId2)
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: remoteClusterId2, Namespace: mapNamespaceName}, nm2); err != nil {
					return false
				}
				value1, ok1 := nm2.Status.NattingTable[nameNamespaceTest]
				value2, ok2 := nm2.Status.DeNattingTable[nameRemoteNamespace]

				By("Check status of NattingTable and DeNattingTable ")
				if ok1 && ok2 && value1 == nameRemoteNamespace && value2 == nameNamespaceTest {
					return true
				}
				return false
			}, timeout, interval).Should(BeTrue())

		})

		It("Adding "+mappingLabel+" and particular labels for "+nameVirtualNode1+" , check NamespaceMaps status", func() {

			By("Try to get Namespace: " + nameNamespaceTest)
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: nameNamespaceTest}, namespace); err != nil {
					return false
				}
				namespace.GetLabels()[mappingLabel] = nameRemoteNamespace
				namespace.GetLabels()[offloadingCluster1Label1] = ""
				namespace.GetLabels()[offloadingCluster1Label2] = ""

				By("Adding labels to: " + nameNamespaceTest)
				if err := k8sClient.Patch(ctx, namespace, client.Merge); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("Try to get NamespaceMap: " + remoteClusterId1)
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: remoteClusterId1, Namespace: mapNamespaceName}, nm1); err != nil {
					return false
				}
				value1, ok1 := nm1.Status.NattingTable[nameNamespaceTest]
				value2, ok2 := nm1.Status.DeNattingTable[nameRemoteNamespace]

				By("Check status of NattingTable and DeNattingTable ")
				if ok1 && ok2 && value1 == nameRemoteNamespace && value2 == nameNamespaceTest {
					return true
				}
				return false
			}, timeout, interval).Should(BeTrue())

			By("Try to get NamespaceMap: " + remoteClusterId2)
			Consistently(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: remoteClusterId2, Namespace: mapNamespaceName}, nm2); err != nil {
					return true
				}
				value1, ok1 := nm2.Status.NattingTable[nameNamespaceTest]
				value2, ok2 := nm2.Status.DeNattingTable[nameRemoteNamespace]

				By("Check status of NattingTable and DeNattingTable ")
				if ok1 && ok2 && value1 == nameRemoteNamespace && value2 == nameNamespaceTest {
					return true
				}
				return false
			}, timeout/5, interval).ShouldNot(BeTrue())

		})

		It("Update "+mappingLabel+"  value and check if the namespace's label is automatically patched to old value", func() {

			By("Try to get Namespace: " + nameNamespaceTest)
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: nameNamespaceTest}, namespace); err != nil {
					return false
				}
				namespace.GetLabels()[mappingLabel] = nameRemoteNamespace
				namespace.GetLabels()[offloadingLabel] = ""

				By("Adding labels to: " + nameNamespaceTest)
				if err := k8sClient.Patch(ctx, namespace, client.Merge); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("Try to get NamespaceMap: " + remoteClusterId1)
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: remoteClusterId1, Namespace: mapNamespaceName}, nm1); err != nil {
					return false
				}
				value1, ok1 := nm1.Status.NattingTable[nameNamespaceTest]
				value2, ok2 := nm1.Status.DeNattingTable[nameRemoteNamespace]

				By("Check status of NattingTable and DeNattingTable ")
				if ok1 && ok2 && value1 == nameRemoteNamespace && value2 == nameNamespaceTest {
					return true
				}
				return false
			}, timeout, interval).Should(BeTrue())

			By("Try to get NamespaceMap: " + remoteClusterId2)
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: remoteClusterId2, Namespace: mapNamespaceName}, nm2); err != nil {
					return false
				}
				value1, ok1 := nm2.Status.NattingTable[nameNamespaceTest]
				value2, ok2 := nm2.Status.DeNattingTable[nameRemoteNamespace]

				By("Check status of NattingTable and DeNattingTable ")
				if ok1 && ok2 && value1 == nameRemoteNamespace && value2 == nameNamespaceTest {
					return true
				}
				return false
			}, timeout, interval).Should(BeTrue())

			By("Try to get again " + nameNamespaceTest)
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: nameNamespaceTest}, namespace); err != nil {
					return false
				}
				namespace.GetLabels()[mappingLabel] = "new-value-remote-namespace-test"

				By("Update " + mappingLabel)
				if err := k8sClient.Update(ctx, namespace); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("Try to get " + nameNamespaceTest + " and check if label " + mappingLabel + " value is the old one")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: nameNamespaceTest}, namespace); err != nil {
					return false
				}
				value, ok := namespace.GetLabels()[mappingLabel]
				return ok && value == nameRemoteNamespace
			}, timeout, interval).Should(BeTrue())

		})

	})

	Context("Check deletion lifecycle of "+nameNamespaceTest, func() {

		BeforeEach(func() {

			By("Add labels to " + nameNamespaceTest)
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: nameNamespaceTest}, namespace); err != nil {
					return false
				}
				namespace.GetLabels()[mappingLabel] = nameRemoteNamespace
				namespace.GetLabels()[offloadingLabel] = ""

				if err := k8sClient.Update(ctx, namespace); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("Try to get NamespaceMap: " + remoteClusterId1)
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: remoteClusterId1, Namespace: mapNamespaceName}, nm1); err != nil {
					return false
				}
				value1, ok1 := nm1.Status.NattingTable[nameNamespaceTest]
				value2, ok2 := nm1.Status.DeNattingTable[nameRemoteNamespace]

				By("Check status of NattingTable and DeNattingTable ")
				if ok1 && ok2 && value1 == nameRemoteNamespace && value2 == nameNamespaceTest {
					return true
				}
				return false
			}, timeout, interval).Should(BeTrue())

			By("Try to get NamespaceMap: " + remoteClusterId2)
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: remoteClusterId2, Namespace: mapNamespaceName}, nm2); err != nil {
					return false
				}
				value1, ok1 := nm2.Status.NattingTable[nameNamespaceTest]
				value2, ok2 := nm2.Status.DeNattingTable[nameRemoteNamespace]

				By("Check status of NattingTable and DeNattingTable ")
				if ok1 && ok2 && value1 == nameRemoteNamespace && value2 == nameNamespaceTest {
					return true
				}
				return false
			}, timeout, interval).Should(BeTrue())

		})

		It("Remove "+nameNamespaceTest+" and check if NamespaceMaps are cleared", func() {

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: nameNamespaceTest}, namespace); err != nil {
					return false
				}

				if err := k8sClient.Delete(ctx, namespace); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("Check if NamespaceMap " + remoteClusterId1 + " is cleaned by the controller")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: remoteClusterId1, Namespace: mapNamespaceName}, nm1); err != nil {
					return false
				}
				if nm1.Status.NattingTable == nil && nm1.Status.DeNattingTable == nil {
					return true
				}

				if _, ok := nm1.Status.NattingTable[nameNamespaceTest]; !ok {
					if _, ok = nm1.Status.DeNattingTable[nameRemoteNamespace]; !ok {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())

			By("Check if NamespaceMap " + remoteClusterId2 + " is cleaned by the controller ")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: remoteClusterId2, Namespace: mapNamespaceName}, nm2); err != nil {
					return false
				}
				if nm2.Status.NattingTable == nil && nm2.Status.DeNattingTable == nil {
					return true
				}

				if _, ok := nm2.Status.NattingTable[nameNamespaceTest]; !ok {
					if _, ok = nm2.Status.DeNattingTable[nameRemoteNamespace]; !ok {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())

		})

	})

})
