/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package namespacemapctrl

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlutils "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mapsv1alpha1 "github.com/liqotech/liqo/apis/virtualKubelet/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

var _ = Describe("NamespaceMap controller", func() {

	const (
		timeout        = time.Second * 20
		interval       = time.Millisecond * 500
		namespace1Name = "namespace-test1"
	)

	BeforeEach(func() {
		By(" 0 - BEFORE_EACH -> Clean NamespaceMap CurrentMappings")
		Eventually(func() bool {
			if err := homeClient.List(context.TODO(), nms, client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterId1}); err != nil {
				return false
			}
			Expect(len(nms.Items) == 1).To(BeTrue())
			nms.Items[0].Spec.DesiredMapping = nil
			nms.Items[0].Status.CurrentMapping = nil
			err := homeClient.Update(context.TODO(), nms.Items[0].DeepCopy())
			return err == nil
		}, timeout, interval).Should(BeTrue())
		Eventually(func() bool {
			if err := homeClient.List(context.TODO(), nms, client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterId1}); err != nil {
				return false
			}
			Expect(len(nms.Items) == 1).To(BeTrue())
			return !ctrlutils.ContainsFinalizer(nms.Items[0].DeepCopy(), namespaceMapControllerFinalizer)
		}, timeout*2, interval).Should(BeTrue())
	})

	Context("Check creation and deletion of remote Namespace", func() {

		It(fmt.Sprintf("Check correct creation of the remote namespace %s on remote cluster '%s'",
			namespace1Name, remoteClusterId1), func() {

			By(" 1 - Adding desired mapping entry to the NamespaceMap")
			Eventually(func() bool {
				Expect(homeClient.List(context.TODO(), nms, client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterId1})).To(Succeed())
				Expect(len(nms.Items) == 1).To(BeTrue())
				nms.Items[0].Spec.DesiredMapping = map[string]string{}
				nms.Items[0].Spec.DesiredMapping[namespace1Name] = namespace1Name
				err := homeClient.Update(context.TODO(), nms.Items[0].DeepCopy())
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By(" 2 - Checking remote namespace existence")
			Eventually(func() bool {
				remoteNamespace := &corev1.Namespace{}
				if err := remoteClient2.Get(context.TODO(), types.NamespacedName{Name: namespace1Name}, remoteNamespace); err != nil {
					return false
				}
				return remoteNamespace.Annotations[liqoAnnotationKey] == remoteNamespaceAnnotationValue
			}, timeout, interval).Should(BeTrue())

			By(" 3 - Checking status of CurrentMapping entry: must be 'Accepted'")
			Eventually(func() bool {
				if err := homeClient.List(context.TODO(), nms, client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterId1}); err != nil {
					return false
				}
				if len(nms.Items) == 0 {
					return false
				}
				return nms.Items[0].Status.CurrentMapping[namespace1Name].RemoteNamespace == namespace1Name &&
					nms.Items[0].Status.CurrentMapping[namespace1Name].Phase == mapsv1alpha1.MappingAccepted
			}, timeout, interval).Should(BeTrue())

			By(" 4 - Finalizer of namespaceMap controller should be here")
			Eventually(func() bool {
				if err := homeClient.List(context.TODO(), nms, client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterId1}); err != nil {
					return false
				}
				Expect(len(nms.Items) == 1).To(BeTrue())
				return ctrlutils.ContainsFinalizer(nms.Items[0].DeepCopy(), namespaceMapControllerFinalizer)
			}, timeout, interval).Should(BeTrue())

			By("------- STARTING DELETION PHASE -------")

			By(fmt.Sprintf(" 5 - Delete desired mapping entry for namespace '%s'", namespace1Name))
			Eventually(func() bool {
				Expect(homeClient.List(context.TODO(), nms, client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterId1})).To(Succeed())
				Expect(len(nms.Items) == 1).To(BeTrue())
				delete(nms.Items[0].Spec.DesiredMapping, namespace1Name)
				err := homeClient.Update(context.TODO(), nms.Items[0].DeepCopy())
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By(fmt.Sprintf(" 6 - Check deletion timestamp of remote namespace '%s'", namespace1Name))
			Eventually(func() bool {
				remoteNamespace := &corev1.Namespace{}
				Expect(remoteClient2.Get(context.TODO(), types.NamespacedName{Name: namespace1Name}, remoteNamespace)).To(Succeed())
				return !remoteNamespace.DeletionTimestamp.IsZero()
			}, timeout, interval).Should(BeTrue())

			By(" 7 - Checking status of CurrentMapping entry: must be 'Terminating'")
			Eventually(func() bool {
				if err := homeClient.List(context.TODO(), nms, client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterId1}); err != nil {
					return false
				}
				Expect(len(nms.Items) == 1).To(BeTrue())
				return nms.Items[0].Status.CurrentMapping[namespace1Name].RemoteNamespace == namespace1Name &&
					nms.Items[0].Status.CurrentMapping[namespace1Name].Phase == mapsv1alpha1.MappingTerminating
			}, timeout, interval).Should(BeTrue())

			By(" 8 - Finalizer of namespaceMap controller should be here")
			Consistently(func() bool {
				if err := homeClient.List(context.TODO(), nms, client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterId1}); err != nil {
					return false
				}
				Expect(len(nms.Items) == 1).To(BeTrue())
				return ctrlutils.ContainsFinalizer(nms.Items[0].DeepCopy(), namespaceMapControllerFinalizer)
			}, timeout/5, interval).Should(BeTrue())

			By(" 9 - Remove CurrentMapping entry")
			Eventually(func() bool {
				if err := homeClient.List(context.TODO(), nms, client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterId1}); err != nil {
					return false
				}
				Expect(len(nms.Items) == 1).To(BeTrue())
				delete(nms.Items[0].Status.CurrentMapping, namespace1Name)
				err := homeClient.Update(context.TODO(), nms.Items[0].DeepCopy())
				return err == nil
			}, timeout, interval).Should(BeTrue())

			// controller restarts with exponential backoff
			By(" 10 - Check if NamespaceMap Controller finalizer is removed")
			Eventually(func() bool {
				if err := homeClient.List(context.TODO(), nms, client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterId1}); err != nil {
					return false
				}
				Expect(len(nms.Items) == 1).To(BeTrue())
				return !ctrlutils.ContainsFinalizer(nms.Items[0].DeepCopy(), namespaceMapControllerFinalizer)
			}, timeout*2, interval).Should(BeTrue())

		})

		It(fmt.Sprintf("Check NamespaceMap status when a remote namespace with the same name '%s' "+
			"already exists on remote cluster '%s'", namespace1Name, remoteClusterId1), func() {
			remoteName := "pippo"
			By(fmt.Sprintf(" 1 - Create remote namespace '%s' if it isn't already there", remoteName))
			Eventually(func() bool {
				remoteNamespace := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: remoteName,
					},
				}
				err := remoteClient2.Create(context.TODO(), remoteNamespace)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By(" 2 - Adding desired mapping entry to the NamespaceMap")
			Eventually(func() bool {
				Expect(homeClient.List(context.TODO(), nms, client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterId1})).To(Succeed())
				Expect(len(nms.Items) == 1).To(BeTrue())
				nms.Items[0].Spec.DesiredMapping = map[string]string{}
				nms.Items[0].Spec.DesiredMapping[remoteName] = remoteName
				err := homeClient.Update(context.TODO(), nms.Items[0].DeepCopy())
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By(" 3 - Checking status of CurrentMapping entry: must be 'CreationLoopBackOff'")
			Eventually(func() bool {
				if err := homeClient.List(context.TODO(), nms, client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterId1}); err != nil {
					return false
				}
				if len(nms.Items) == 0 {
					return false
				}
				return nms.Items[0].Status.CurrentMapping[remoteName].RemoteNamespace == remoteName &&
					nms.Items[0].Status.CurrentMapping[remoteName].Phase == mapsv1alpha1.MappingCreationLoopBackOff
			}, timeout, interval).Should(BeTrue())

			By(" 4 - Finalizer of namespaceMap controller should be here")
			Eventually(func() bool {
				if err := homeClient.List(context.TODO(), nms, client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterId1}); err != nil {
					return false
				}
				Expect(len(nms.Items) == 1).To(BeTrue())
				return ctrlutils.ContainsFinalizer(nms.Items[0].DeepCopy(), namespaceMapControllerFinalizer)
			}, timeout, interval).Should(BeTrue())

			By("------- STARTING DELETION PHASE -------")

			By(fmt.Sprintf(" 5 - Delete desired mapping entry for namespace '%s'", namespace1Name))
			Eventually(func() bool {
				Expect(homeClient.List(context.TODO(), nms, client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterId1})).To(Succeed())
				Expect(len(nms.Items) == 1).To(BeTrue())
				delete(nms.Items[0].Spec.DesiredMapping, remoteName)
				err := homeClient.Update(context.TODO(), nms.Items[0].DeepCopy())
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By(" 6 - Check if NamespaceMap Controller finalizer is removed")
			Eventually(func() bool {
				if err := homeClient.List(context.TODO(), nms, client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterId1}); err != nil {
					return false
				}
				Expect(len(nms.Items) == 1).To(BeTrue())
				return !ctrlutils.ContainsFinalizer(nms.Items[0].DeepCopy(), namespaceMapControllerFinalizer)
			}, timeout*2, interval).Should(BeTrue())

		})

	})

	// manca il test che testa la cancellazione della risorsa
	// aggiungere un po' di namespace remoti prima con le desired entry
	// guardare se parte la logica di cancellazione
	Context("Check the deletion phase of the NamespaceMap", func() {

		It("Create some remote Namespace and then verify the deletion logic when the NamespaceMap is "+
			"requested to be deleted", func() {

			namespace2Name := "namespace-test2"
			namespace3Name := "namespace-test3"
			namespace4Name := "namespace-test4"
			namespace5Name := "namespace-test5"

			By(" 1 - Get NamespaceMap and set 2 desiredMappings")
			Eventually(func() bool {
				Expect(homeClient.List(context.TODO(), nms, client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterId1})).To(Succeed())
				Expect(len(nms.Items) == 1).To(BeTrue())
				if nms.Items[0].Spec.DesiredMapping == nil {
					nms.Items[0].Spec.DesiredMapping = map[string]string{}
				}
				nms.Items[0].Spec.DesiredMapping[namespace2Name] = namespace2Name
				nms.Items[0].Spec.DesiredMapping[namespace3Name] = namespace3Name
				err := homeClient.Update(context.TODO(), nms.Items[0].DeepCopy())
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By(" 2 - Get NamespaceMap and set another 1 desiredMappings")
			Eventually(func() bool {
				Expect(homeClient.List(context.TODO(), nms, client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterId1})).To(Succeed())
				Expect(len(nms.Items) == 1).To(BeTrue())
				if nms.Items[0].Spec.DesiredMapping == nil {
					nms.Items[0].Spec.DesiredMapping = map[string]string{}
				}
				nms.Items[0].Spec.DesiredMapping[namespace4Name] = namespace4Name
				err := homeClient.Update(context.TODO(), nms.Items[0].DeepCopy())
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By(" 3 - Get NamespaceMap and set another 1 desiredMappings")
			Eventually(func() bool {
				Expect(homeClient.List(context.TODO(), nms, client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterId1})).To(Succeed())
				Expect(len(nms.Items) == 1).To(BeTrue())
				if nms.Items[0].Spec.DesiredMapping == nil {
					nms.Items[0].Spec.DesiredMapping = map[string]string{}
				}
				nms.Items[0].Spec.DesiredMapping[namespace5Name] = namespace5Name
				err := homeClient.Update(context.TODO(), nms.Items[0].DeepCopy())
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By(" 4 - Check len of DesiredMapping (len==4) and MappingPhase must be 'Accepted' ")
			Eventually(func() bool {
				Expect(homeClient.List(context.TODO(), nms, client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterId1})).To(Succeed())
				Expect(len(nms.Items) == 1).To(BeTrue())
				if len(nms.Items[0].Spec.DesiredMapping) != 4 {
					return false
				}
				return nms.Items[0].Status.CurrentMapping[namespace2Name].Phase == mapsv1alpha1.MappingAccepted &&
					nms.Items[0].Status.CurrentMapping[namespace3Name].Phase == mapsv1alpha1.MappingAccepted &&
					nms.Items[0].Status.CurrentMapping[namespace4Name].Phase == mapsv1alpha1.MappingAccepted &&
					nms.Items[0].Status.CurrentMapping[namespace5Name].Phase == mapsv1alpha1.MappingAccepted
			}, timeout, interval).Should(BeTrue())

			By(" 5 - Delete NamespaceMap, so the deletion timestamp is set")
			Eventually(func() bool {
				Expect(homeClient.List(context.TODO(), nms, client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterId1})).To(Succeed())
				Expect(len(nms.Items) == 1).To(BeTrue())
				err := homeClient.Delete(context.TODO(), nms.Items[0].DeepCopy())
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By(" 6 - Check if all remote Namespaces are in terminating phase ")
			Eventually(func() bool {
				Expect(homeClient.List(context.TODO(), nms, client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterId1})).To(Succeed())
				Expect(len(nms.Items) == 1).To(BeTrue())
				return nms.Items[0].Status.CurrentMapping[namespace2Name].Phase == mapsv1alpha1.MappingTerminating &&
					nms.Items[0].Status.CurrentMapping[namespace3Name].Phase == mapsv1alpha1.MappingTerminating &&
					nms.Items[0].Status.CurrentMapping[namespace4Name].Phase == mapsv1alpha1.MappingTerminating &&
					nms.Items[0].Status.CurrentMapping[namespace5Name].Phase == mapsv1alpha1.MappingTerminating
			}, timeout, interval).Should(BeTrue())

			By(" 7 - Check if NamespaceMap Controller finalizer is still there")
			Eventually(func() bool {
				if err := homeClient.List(context.TODO(), nms, client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterId1}); err != nil {
					return false
				}
				Expect(len(nms.Items) == 1).To(BeTrue())
				return ctrlutils.ContainsFinalizer(nms.Items[0].DeepCopy(), namespaceMapControllerFinalizer)
			}, timeout*2, interval).Should(BeTrue())

			By(" 8 - Clean NamespaceMap status")
			Eventually(func() bool {
				Expect(homeClient.List(context.TODO(), nms, client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterId1})).To(Succeed())
				Expect(len(nms.Items) == 1).To(BeTrue())
				nms.Items[0].Status.CurrentMapping = nil
				err := homeClient.Update(context.TODO(), nms.Items[0].DeepCopy())
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By(" 9 - Check if NamespaceMap is removed")
			Eventually(func() bool {
				if err := homeClient.List(context.TODO(), nms, client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterId1}); err != nil {
					return false
				}
				return len(nms.Items) == 0
			}, timeout*2, interval).Should(BeTrue())
		})
	})

})
