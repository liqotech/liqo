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

package offloadingstatuscontroller

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutils "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	offv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	mapsv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

var _ = Describe("Namespace controller", func() {

	const (
		timeout       = time.Second * 20
		interval      = time.Millisecond * 500
		testFinalizer = "test-finalizer"
	)

	BeforeEach(func() {

		By(" 0 - BEFORE_EACH -> Clean NamespaceMap CurrentMapping")

		// 0.1 - Clean namespaceMaps CurrentMapping
		Eventually(func() bool {
			if err := homeClient.List(context.TODO(), nms); err != nil {
				return false
			}
			Expect(len(nms.Items) == mapNumber).To(BeTrue())
			for i := range nms.Items {
				nms.Items[i].Status.CurrentMapping = map[string]mapsv1alpha1.RemoteNamespaceStatus{}
				if err := homeClient.Status().Update(context.TODO(), nms.Items[i].DeepCopy()); err != nil {
					return false
				}
			}
			return true
		}, timeout, interval).Should(BeTrue())

		// 0.2 - Check that they are cleaned
		Eventually(func() bool {
			if err := homeClient.List(context.TODO(), nms); err != nil {
				return false
			}
			Expect(len(nms.Items) == mapNumber).To(BeTrue())
			for i := range nms.Items {
				if len(nms.Items[i].Status.CurrentMapping) != 0 {
					return false
				}
			}
			return true
		}, timeout, interval).Should(BeTrue())

	})

	// Todo: this implementation is without InProgress Status
	Context("Check RemoteNamespaceConditions and Status of NamespaceOffloading1", func() {

		It(" TEST 1: check NoClusterSelected status, when NamespaceMap Status is empty", func() {

			By(" 1 - Checking status of NamespaceOffloading ")
			Eventually(func() bool {
				if err := homeClient.Get(context.TODO(), types.NamespacedName{
					Name:      liqoconst.DefaultNamespaceOffloadingName,
					Namespace: namespace1Name}, namespaceOffloading1); err != nil {
					return false
				}
				if namespaceOffloading1.Status.OffloadingPhase != offv1alpha1.NoClusterSelectedOffloadingPhaseType {
					return false
				}
				if len(namespaceOffloading1.Status.RemoteNamespacesConditions[remoteClusterID1]) != 1 ||
					namespaceOffloading1.Status.RemoteNamespacesConditions[remoteClusterID1][0].Type != offv1alpha1.NamespaceOffloadingRequired ||
					namespaceOffloading1.Status.RemoteNamespacesConditions[remoteClusterID1][0].Status != corev1.ConditionFalse {
					return false
				}
				if len(namespaceOffloading1.Status.RemoteNamespacesConditions[remoteClusterID2]) != 1 ||
					namespaceOffloading1.Status.RemoteNamespacesConditions[remoteClusterID2][0].Type != offv1alpha1.NamespaceOffloadingRequired ||
					namespaceOffloading1.Status.RemoteNamespacesConditions[remoteClusterID2][0].Status != corev1.ConditionFalse {
					return false
				}
				if len(namespaceOffloading1.Status.RemoteNamespacesConditions[remoteClusterID3]) != 1 ||
					namespaceOffloading1.Status.RemoteNamespacesConditions[remoteClusterID3][0].Type != offv1alpha1.NamespaceOffloadingRequired ||
					namespaceOffloading1.Status.RemoteNamespacesConditions[remoteClusterID3][0].Status != corev1.ConditionFalse {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
		})

		It(" TEST 2: check Ready status", func() {

			By(" 1 - Get NamespaceMap associated to remote cluster 1 and change Status")
			Eventually(func() bool {
				if err := homeClient.List(context.TODO(), nms, client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterID1}); err != nil {
					return false
				}
				Expect(len(nms.Items) == 1).To(BeTrue())
				nms.Items[0].Status.CurrentMapping = map[string]mapsv1alpha1.RemoteNamespaceStatus{}
				nms.Items[0].Status.CurrentMapping[namespace1Name] = mapsv1alpha1.RemoteNamespaceStatus{
					RemoteNamespace: namespace1Name,
					Phase:           mapsv1alpha1.MappingAccepted,
				}
				err := homeClient.Status().Update(context.TODO(), nms.Items[0].DeepCopy())
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By(" 2 - Checking Ready status of the NamespaceOffloading ")
			Eventually(func() bool {
				if err := homeClient.Get(context.TODO(), types.NamespacedName{
					Name:      liqoconst.DefaultNamespaceOffloadingName,
					Namespace: namespace1Name}, namespaceOffloading1); err != nil {
					return false
				}
				if namespaceOffloading1.Status.OffloadingPhase != offv1alpha1.ReadyOffloadingPhaseType {
					return false
				}
				if len(namespaceOffloading1.Status.RemoteNamespacesConditions[remoteClusterID1]) != 1 ||
					namespaceOffloading1.Status.RemoteNamespacesConditions[remoteClusterID1][0].Type != offv1alpha1.NamespaceReady ||
					namespaceOffloading1.Status.RemoteNamespacesConditions[remoteClusterID1][0].Status != corev1.ConditionTrue {
					return false
				}
				if len(namespaceOffloading1.Status.RemoteNamespacesConditions[remoteClusterID2]) != 1 ||
					namespaceOffloading1.Status.RemoteNamespacesConditions[remoteClusterID2][0].Type != offv1alpha1.NamespaceOffloadingRequired ||
					namespaceOffloading1.Status.RemoteNamespacesConditions[remoteClusterID2][0].Status != corev1.ConditionFalse {
					return false
				}
				if len(namespaceOffloading1.Status.RemoteNamespacesConditions[remoteClusterID3]) != 1 ||
					namespaceOffloading1.Status.RemoteNamespacesConditions[remoteClusterID3][0].Type != offv1alpha1.NamespaceOffloadingRequired ||
					namespaceOffloading1.Status.RemoteNamespacesConditions[remoteClusterID3][0].Status != corev1.ConditionFalse {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
		})

		It(" TEST 3: check AllFailed status", func() {

			By(" 1 - Get NamespaceMap associated to remote cluster 2 and change Status")
			Eventually(func() bool {
				if err := homeClient.List(context.TODO(), nms, client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterID2}); err != nil {
					return false
				}
				Expect(len(nms.Items) == 1).To(BeTrue())
				nms.Items[0].Status.CurrentMapping = map[string]mapsv1alpha1.RemoteNamespaceStatus{}
				nms.Items[0].Status.CurrentMapping[namespace1Name] = mapsv1alpha1.RemoteNamespaceStatus{
					RemoteNamespace: namespace1Name,
					Phase:           mapsv1alpha1.MappingCreationLoopBackOff,
				}
				err := homeClient.Status().Update(context.TODO(), nms.Items[0].DeepCopy())
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By(" 2 - Checking AllFailed status of the NamespaceOffloading ")
			Eventually(func() bool {
				if err := homeClient.Get(context.TODO(), types.NamespacedName{
					Name:      liqoconst.DefaultNamespaceOffloadingName,
					Namespace: namespace1Name}, namespaceOffloading1); err != nil {
					return false
				}
				if namespaceOffloading1.Status.OffloadingPhase != offv1alpha1.AllFailedOffloadingPhaseType {
					return false
				}
				if len(namespaceOffloading1.Status.RemoteNamespacesConditions[remoteClusterID1]) != 1 ||
					namespaceOffloading1.Status.RemoteNamespacesConditions[remoteClusterID1][0].Type != offv1alpha1.NamespaceOffloadingRequired ||
					namespaceOffloading1.Status.RemoteNamespacesConditions[remoteClusterID1][0].Status != corev1.ConditionFalse {
					return false
				}
				if len(namespaceOffloading1.Status.RemoteNamespacesConditions[remoteClusterID2]) != 1 ||
					namespaceOffloading1.Status.RemoteNamespacesConditions[remoteClusterID2][0].Type != offv1alpha1.NamespaceReady ||
					namespaceOffloading1.Status.RemoteNamespacesConditions[remoteClusterID2][0].Status != corev1.ConditionFalse {
					return false
				}
				if len(namespaceOffloading1.Status.RemoteNamespacesConditions[remoteClusterID3]) != 1 ||
					namespaceOffloading1.Status.RemoteNamespacesConditions[remoteClusterID3][0].Type != offv1alpha1.NamespaceOffloadingRequired ||
					namespaceOffloading1.Status.RemoteNamespacesConditions[remoteClusterID3][0].Status != corev1.ConditionFalse {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
		})

		It(" TEST 4: check SomeFailed status", func() {

			By(" 1 - Get NamespaceMap associated to remote cluster 2 and change Status to MappingCreationBackoff")
			Eventually(func() bool {
				if err := homeClient.List(context.TODO(), nms, client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterID2}); err != nil {
					return false
				}
				Expect(len(nms.Items) == 1).To(BeTrue())
				nms.Items[0].Status.CurrentMapping = map[string]mapsv1alpha1.RemoteNamespaceStatus{}
				nms.Items[0].Status.CurrentMapping[namespace1Name] = mapsv1alpha1.RemoteNamespaceStatus{
					RemoteNamespace: namespace1Name,
					Phase:           mapsv1alpha1.MappingCreationLoopBackOff,
				}
				err := homeClient.Status().Update(context.TODO(), nms.Items[0].DeepCopy())
				return err == nil
			}, timeout, interval).Should(BeTrue())

			time.Sleep(time.Second * 3)

			By(" 2 - Get NamespaceMap associated to remote cluster 1 and change Status to MappingAccepted")
			Eventually(func() bool {
				if err := homeClient.List(context.TODO(), nms, client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterID1}); err != nil {
					return false
				}
				Expect(len(nms.Items) == 1).To(BeTrue())
				nms.Items[0].Status.CurrentMapping = map[string]mapsv1alpha1.RemoteNamespaceStatus{}
				nms.Items[0].Status.CurrentMapping[namespace1Name] = mapsv1alpha1.RemoteNamespaceStatus{
					RemoteNamespace: namespace1Name,
					Phase:           mapsv1alpha1.MappingAccepted,
				}
				err := homeClient.Status().Update(context.TODO(), nms.Items[0].DeepCopy())
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By(" 3 - Checking SomeFailed status of the NamespaceOffloading ")

			Eventually(func() bool {
				if err := homeClient.Get(context.TODO(), types.NamespacedName{
					Name:      liqoconst.DefaultNamespaceOffloadingName,
					Namespace: namespace1Name}, namespaceOffloading1); err != nil {
					return false
				}
				if namespaceOffloading1.Status.OffloadingPhase != offv1alpha1.SomeFailedOffloadingPhaseType {
					return false
				}
				if len(namespaceOffloading1.Status.RemoteNamespacesConditions[remoteClusterID1]) != 1 ||
					namespaceOffloading1.Status.RemoteNamespacesConditions[remoteClusterID1][0].Type != offv1alpha1.NamespaceReady ||
					namespaceOffloading1.Status.RemoteNamespacesConditions[remoteClusterID1][0].Status != corev1.ConditionTrue {
					return false
				}
				if len(namespaceOffloading1.Status.RemoteNamespacesConditions[remoteClusterID2]) != 1 ||
					namespaceOffloading1.Status.RemoteNamespacesConditions[remoteClusterID2][0].Type != offv1alpha1.NamespaceReady ||
					namespaceOffloading1.Status.RemoteNamespacesConditions[remoteClusterID2][0].Status != corev1.ConditionFalse {
					return false
				}
				if len(namespaceOffloading1.Status.RemoteNamespacesConditions[remoteClusterID3]) != 1 ||
					namespaceOffloading1.Status.RemoteNamespacesConditions[remoteClusterID3][0].Type != offv1alpha1.NamespaceOffloadingRequired ||
					namespaceOffloading1.Status.RemoteNamespacesConditions[remoteClusterID3][0].Status != corev1.ConditionFalse {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			Expect(homeClient.Delete(context.TODO(), namespaceOffloading1)).To(Succeed())
		})

	})

	Context("Check RemoteNamespaceConditions and Status of NamespaceOffloading when the deletion timestamp is set", func() {

		It(" TEST 5: set the Deletion timestamp on NamespaceOffloading and check the evolution of its status", func() {

			// The namespace name is associated with the test number
			namespace5Name := "namespace5"
			By(fmt.Sprintf(" 1 - Create the namespace '%s'", namespace5Name))
			namespace5 := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace5Name,
				},
			}
			Expect(homeClient.Create(context.TODO(), namespace5)).To(Succeed())

			By(" 2 - Create the associated NamespaceOffloading with finalizer")
			namespaceOffloading5 := &offv1alpha1.NamespaceOffloading{
				ObjectMeta: metav1.ObjectMeta{
					Name:      liqoconst.DefaultNamespaceOffloadingName,
					Namespace: namespace5Name,
				},
				Spec: offv1alpha1.NamespaceOffloadingSpec{
					NamespaceMappingStrategy: offv1alpha1.EnforceSameNameMappingStrategyType,
					PodOffloadingStrategy:    offv1alpha1.LocalAndRemotePodOffloadingStrategyType,
					ClusterSelector:          corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{}},
				},
			}
			ctrlutils.AddFinalizer(namespaceOffloading5, testFinalizer)
			Expect(homeClient.Create(context.TODO(), namespaceOffloading5)).To(Succeed())

			By(" 3 - Get NamespaceMap associated to remote cluster 2 and change Status to MappingCreationBackoff")
			Eventually(func() bool {
				if err := homeClient.List(context.TODO(), nms, client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterID2}); err != nil {
					return false
				}
				Expect(len(nms.Items) == 1).To(BeTrue())
				nms.Items[0].Status.CurrentMapping = map[string]mapsv1alpha1.RemoteNamespaceStatus{}
				nms.Items[0].Status.CurrentMapping[namespace5Name] = mapsv1alpha1.RemoteNamespaceStatus{
					RemoteNamespace: namespace5Name,
					Phase:           mapsv1alpha1.MappingCreationLoopBackOff,
				}
				err := homeClient.Status().Update(context.TODO(), nms.Items[0].DeepCopy())
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By(" 4 - Get NamespaceMap associated to remote cluster 1 and change Status to MappingAccepted")
			Eventually(func() bool {
				if err := homeClient.List(context.TODO(), nms, client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterID1}); err != nil {
					return false
				}
				Expect(len(nms.Items) == 1).To(BeTrue())
				nms.Items[0].Status.CurrentMapping = map[string]mapsv1alpha1.RemoteNamespaceStatus{}
				nms.Items[0].Status.CurrentMapping[namespace5Name] = mapsv1alpha1.RemoteNamespaceStatus{
					RemoteNamespace: namespace5Name,
					Phase:           mapsv1alpha1.MappingAccepted,
				}
				err := homeClient.Status().Update(context.TODO(), nms.Items[0].DeepCopy())
				return err == nil
			}, timeout, interval).Should(BeTrue())

			time.Sleep(time.Second * 3)

			By(" 5 - Set the deletion timestamp")
			Eventually(func() bool {
				if err := homeClient.Get(context.TODO(), types.NamespacedName{
					Name:      liqoconst.DefaultNamespaceOffloadingName,
					Namespace: namespace5Name}, namespaceOffloading5); err != nil {
					return false
				}
				err := homeClient.Delete(context.TODO(), namespaceOffloading5)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By(" 6 - Checking Terminating status of the NamespaceOffloading and the remote conditions")
			Eventually(func() bool {
				if err := homeClient.Get(context.TODO(), types.NamespacedName{
					Name:      liqoconst.DefaultNamespaceOffloadingName,
					Namespace: namespace5Name}, namespaceOffloading5); err != nil {
					return false
				}
				if namespaceOffloading5.Status.OffloadingPhase != offv1alpha1.TerminatingOffloadingPhaseType {
					return false
				}
				if len(namespaceOffloading5.Status.RemoteNamespacesConditions[remoteClusterID1]) != 1 ||
					namespaceOffloading5.Status.RemoteNamespacesConditions[remoteClusterID1][0].Type != offv1alpha1.NamespaceReady ||
					namespaceOffloading5.Status.RemoteNamespacesConditions[remoteClusterID1][0].Status != corev1.ConditionTrue {
					return false
				}
				if len(namespaceOffloading5.Status.RemoteNamespacesConditions[remoteClusterID2]) != 1 ||
					namespaceOffloading5.Status.RemoteNamespacesConditions[remoteClusterID2][0].Type != offv1alpha1.NamespaceReady ||
					namespaceOffloading5.Status.RemoteNamespacesConditions[remoteClusterID2][0].Status != corev1.ConditionFalse {
					return false
				}
				if len(namespaceOffloading5.Status.RemoteNamespacesConditions[remoteClusterID3]) != 1 ||
					namespaceOffloading5.Status.RemoteNamespacesConditions[remoteClusterID3][0].Type != offv1alpha1.NamespaceOffloadingRequired ||
					namespaceOffloading5.Status.RemoteNamespacesConditions[remoteClusterID3][0].Status != corev1.ConditionFalse {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By(" 7 - Clean NamespaceMap status")
			Eventually(func() bool {
				if err := homeClient.List(context.TODO(), nms); err != nil {
					return false
				}
				Expect(len(nms.Items) == mapNumber).To(BeTrue())
				for i := range nms.Items {
					nms.Items[i].Status.CurrentMapping = map[string]mapsv1alpha1.RemoteNamespaceStatus{}
					if err := homeClient.Status().Update(context.TODO(), nms.Items[i].DeepCopy()); err != nil {
						return false
					}
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By(" 8 - Checking Terminating status of the NamespaceOffloading and the remote conditions must be empty")
			Eventually(func() bool {
				if err := homeClient.Get(context.TODO(), types.NamespacedName{
					Name:      liqoconst.DefaultNamespaceOffloadingName,
					Namespace: namespace5Name}, namespaceOffloading5); err != nil {
					return false
				}
				if namespaceOffloading5.Status.OffloadingPhase != offv1alpha1.TerminatingOffloadingPhaseType {
					return false
				}
				return len(namespaceOffloading5.Status.RemoteNamespacesConditions[remoteClusterID1]) == 0 &&
					len(namespaceOffloading5.Status.RemoteNamespacesConditions[remoteClusterID2]) == 0 &&
					len(namespaceOffloading5.Status.RemoteNamespacesConditions[remoteClusterID3]) == 0
			}, timeout, interval).Should(BeTrue())

		})

		It(" TEST 6: Delete a NamespaceMap and check if the corresponding remote condition is deleted", func() {

			// The namespace name is associated with the test number
			namespace6Name := "namespace6"
			By(fmt.Sprintf(" 1 - Creating the namespace '%s'", namespace6Name))
			namespace6 := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace6Name,
				},
			}
			Expect(homeClient.Create(context.TODO(), namespace6)).To(BeNil())

			By(" 2 - Creating the associated NamespaceOffloading")
			namespaceOffloading6 := &offv1alpha1.NamespaceOffloading{
				ObjectMeta: metav1.ObjectMeta{
					Name:      liqoconst.DefaultNamespaceOffloadingName,
					Namespace: namespace6Name,
				},
				Spec: offv1alpha1.NamespaceOffloadingSpec{
					NamespaceMappingStrategy: offv1alpha1.EnforceSameNameMappingStrategyType,
					PodOffloadingStrategy:    offv1alpha1.LocalAndRemotePodOffloadingStrategyType,
					ClusterSelector:          corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{}},
				},
			}
			Expect(homeClient.Create(context.TODO(), namespaceOffloading6)).To(BeNil())

			By(" 3 - Get NamespaceMap associated to remote cluster 2 and delete it")
			Eventually(func() error {
				if err := homeClient.List(context.TODO(), nms, client.MatchingLabels{liqoconst.RemoteClusterID: remoteClusterID2}); err != nil {
					return err
				}
				if len(nms.Items) == 0 {
					return nil
				}
				_ = homeClient.Delete(context.TODO(), nms.Items[0].DeepCopy())
				return fmt.Errorf("the namespaceMap deletion is still in progress")
			}, timeout, interval).Should(BeNil())

			By(" 4 - Checking Terminating status of the NamespaceOffloading and the remote conditions")
			namespaceOffloading6 = &offv1alpha1.NamespaceOffloading{}
			Eventually(func() error {
				if err := homeClient.Get(context.TODO(), types.NamespacedName{
					Name:      liqoconst.DefaultNamespaceOffloadingName,
					Namespace: namespace6Name}, namespaceOffloading6); err != nil {
					return err
				}
				if len(namespaceOffloading6.Status.RemoteNamespacesConditions) != mapNumber-1 {
					return fmt.Errorf("there are still '%d' remoteNamespaceCondition",
						len(namespaceOffloading6.Status.RemoteNamespacesConditions))
				}
				if len(namespaceOffloading6.Status.RemoteNamespacesConditions[remoteClusterID1]) != 1 {
					return fmt.Errorf("the remote condition associated with the namespaceMap 1 is not present")
				}
				if len(namespaceOffloading6.Status.RemoteNamespacesConditions[remoteClusterID3]) != 1 {
					return fmt.Errorf("the remote condition associated with the namespaceMap 3 is not present")
				}
				return nil
			}, timeout, interval).Should(BeNil())

		})

	})
})
