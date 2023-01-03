// Copyright 2019-2023 The Liqo Authors
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

// Package remotenamespacecreation tests the creation of remote namespaces.
package remotenamespacecreation

import (
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	offloadingv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	virtualkubeletv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	"github.com/liqotech/liqo/internal/crdReplicator/reflection"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreignCluster"
	"github.com/liqotech/liqo/test/e2e/testutils/tester"
	"github.com/liqotech/liqo/test/e2e/testutils/util"
)

const (
	// clustersRequired is the number of clusters required in this E2E test.
	clustersRequired = 4
	// testNamespaceName is the name of the test namespace for this test.
	testNamespaceName = "test-namespace-creation"
	// testName is the name of this E2E test.
	testName = "E2E_REMOTE_NAMESPACE_CREATION"
)

func TestE2E(t *testing.T) {
	util.CheckIfTestIsSkipped(t, clustersRequired, testName)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Liqo E2E Suite")
}

var _ = Describe("Liqo E2E", func() {
	var (
		ctx         = context.Background()
		testContext = tester.GetTester(ctx)
		interval    = 1 * time.Second
		timeout     = 10 * time.Second
		localIndex  = 0
		// index of the cluster on which the remote namespace is deleted to test the recreation process.
		remoteIndex = 2

		remoteTestNamespaceName string
		localSelector           client.MatchingLabelsSelector
	)

	BeforeEach(func() {
		remoteTestNamespaceName = fmt.Sprintf("%s-%s", testNamespaceName,
			foreignclusterutils.UniqueName(&testContext.Clusters[localIndex].Cluster))

		// Build the selector to consider only local NamespaceMaps.
		metals := reflection.LocalResourcesLabelSelector()
		selector, err := metav1.LabelSelectorAsSelector(&metals)
		Expect(err).ToNot(HaveOccurred())
		localSelector = client.MatchingLabelsSelector{Selector: selector}
	})

	Context(fmt.Sprintf("Create a namespace inside the cluster '%d', offload it and check if the remote namespaces "+
		"are created inside all remote clusters. Unoffload the namespace and check the deletion of the remote namespaces.", localIndex), func() {

		It(fmt.Sprintf("Create a namespace inside the cluster '%d', offload it and check if the remote namespaces"+
			"are created inside all remote clusters", localIndex), func() {
			namespace := &corev1.Namespace{}
			namespaceMapsList := &virtualkubeletv1alpha1.NamespaceMapList{}

			By(fmt.Sprintf(" 1 - Creating the local namespace inside the cluster '%d'", localIndex))
			Eventually(func() error {
				if _, err := util.EnforceNamespace(ctx, testContext.Clusters[localIndex].NativeClient,
					testContext.Clusters[localIndex].Cluster, testNamespaceName); err != nil {
					return err
				}

				return util.OffloadNamespace(testContext.Clusters[localIndex].KubeconfigPath, testNamespaceName)
			}, timeout, interval).Should(BeNil())

			By(" 2 - Getting the NamespaceMaps and checking the presence of the entries for that namespace, both in the spec and status")
			Eventually(func() error {
				if err := testContext.Clusters[localIndex].ControllerClient.List(ctx, namespaceMapsList, localSelector); err != nil {
					return err
				}
				Expect(len(namespaceMapsList.Items)).To(Equal(clustersRequired - 1))
				for i := range namespaceMapsList.Items {
					desiredMapping, desiredMappingPresence := namespaceMapsList.Items[i].Spec.DesiredMapping[testNamespaceName]
					if !desiredMappingPresence {
						return fmt.Errorf(" In the NamespaceMap corresponding to the cluster %q, "+
							"there is no DesiredMapping for the namespace %q",
							namespaceMapsList.Items[i].Labels[liqoconst.RemoteClusterID], testNamespaceName)
					}
					if desiredMapping != remoteTestNamespaceName {
						return fmt.Errorf(" In the NamespaceMap corresponding to the cluster %q, "+
							"the DesiredMapping for the namespace %q has the wrong value: %q (expected %q)",
							namespaceMapsList.Items[i].Labels[liqoconst.RemoteClusterID], testNamespaceName,
							desiredMapping, remoteTestNamespaceName)
					}
					currentMapping, currentMappingPresence := namespaceMapsList.Items[i].Status.CurrentMapping[testNamespaceName]
					if !currentMappingPresence {
						return fmt.Errorf(" In the NamespaceMap corresponding to the cluster %q, "+
							"there is no CurrentMapping for the namespace %q",
							namespaceMapsList.Items[i].Labels[liqoconst.RemoteClusterID], testNamespaceName)
					}
					if currentMapping.RemoteNamespace != remoteTestNamespaceName {
						return fmt.Errorf(" In the NamespaceMap corresponding to the cluster %q, "+
							"the CurrentMapping for the namespace %q has the wrong value: %q (expected %q)",
							namespaceMapsList.Items[i].Labels[liqoconst.RemoteClusterID], testNamespaceName,
							currentMapping.RemoteNamespace, remoteTestNamespaceName)
					}
					if currentMapping.Phase != virtualkubeletv1alpha1.MappingAccepted {
						return fmt.Errorf(" In the NamespaceMap corresponding to the cluster %q, "+
							"the CurrentMapping for the namespace %q has the wrong phase: %q",
							namespaceMapsList.Items[i].Labels[liqoconst.RemoteClusterID], testNamespaceName,
							currentMapping.Phase)
					}
				}
				return nil
			}, timeout, interval).Should(BeNil())

			By(" 3 - Checking the presence of the remote namespaces")
			for i := range testContext.Clusters {
				if i == localIndex {
					continue
				}
				Eventually(func() error {
					return testContext.Clusters[i].ControllerClient.Get(ctx,
						types.NamespacedName{Name: remoteTestNamespaceName}, namespace)
				}, timeout, interval).Should(BeNil())
				value, ok := namespace.Annotations[liqoconst.RemoteNamespaceManagedByAnnotationKey]
				Expect(ok).To(BeTrue())
				Expect(value).To(HaveSuffix(foreignclusterutils.UniqueName(&testContext.Clusters[i].Cluster)))
			}

			var oldUIDRemoteNamespace types.UID
			By(fmt.Sprintf(" 4 - Deleting the remote namespace inside the cluster '%d'", remoteIndex))
			Eventually(func() error {
				if err := testContext.Clusters[remoteIndex].ControllerClient.Get(ctx,
					types.NamespacedName{Name: remoteTestNamespaceName}, namespace); err != nil {
					return err
				}
				oldUIDRemoteNamespace = namespace.UID
				return testContext.Clusters[remoteIndex].ControllerClient.Delete(ctx, namespace)
			}, timeout, interval).Should(BeNil())

			By(fmt.Sprintf(" 5 - Checking that the remote namespace inside the cluster '%d' has been recreated", remoteIndex))
			Eventually(func() error {
				if err := testContext.Clusters[remoteIndex].ControllerClient.Get(ctx,
					types.NamespacedName{Name: remoteTestNamespaceName}, namespace); err != nil {
					return err
				}
				if oldUIDRemoteNamespace == namespace.UID {
					return fmt.Errorf("the old remote namespace still exists")
				}
				return nil
			}, timeout, interval).Should(BeNil())
		})

		It("Unoffload the namespace and check the deletion of the remote namespaces.", func() {
			namespace := &corev1.Namespace{}
			namespaceOffloading := &offloadingv1alpha1.NamespaceOffloading{}
			namespaceMapsList := &virtualkubeletv1alpha1.NamespaceMapList{}

			By(fmt.Sprintf(" 1 - Unoffloading the namespace inside the cluster %d", localIndex))
			Expect(util.UnoffloadNamespace(testContext.Clusters[localIndex].KubeconfigPath, testNamespaceName)).To(Succeed())

			By(" 2 - Checking if the NamespaceOffloading resource associated with the test namespace is correctly removed.")
			Eventually(func() metav1.StatusReason {
				return apierrors.ReasonForError(testContext.Clusters[localIndex].ControllerClient.Get(ctx, types.NamespacedName{
					Name: liqoconst.DefaultNamespaceOffloadingName, Namespace: testNamespaceName}, namespaceOffloading))
			}, timeout, interval).Should(Equal(metav1.StatusReasonNotFound))

			By(" 3 - Checking if the NamespaceMaps do not have the entries for that test namespace")
			Eventually(func() error {
				return testContext.Clusters[localIndex].ControllerClient.List(ctx, namespaceMapsList, localSelector)
			}, timeout, interval).Should(BeNil())
			Expect(len(namespaceMapsList.Items)).To(Equal(clustersRequired - 1))
			for i := range namespaceMapsList.Items {
				_, ok := namespaceMapsList.Items[i].Spec.DesiredMapping[testNamespaceName]
				Expect(ok).To(BeFalse())
				_, ok = namespaceMapsList.Items[i].Status.CurrentMapping[testNamespaceName]
				Expect(ok).To(BeFalse())
			}

			By(" 4 - Checking the absence of the remote namespaces")
			for i := range testContext.Clusters {
				if i == localIndex {
					continue
				}
				Eventually(func() metav1.StatusReason {
					return apierrors.ReasonForError(testContext.Clusters[i].ControllerClient.Get(ctx,
						types.NamespacedName{Name: remoteTestNamespaceName}, namespace))
				}, timeout, interval).Should(Equal(metav1.StatusReasonNotFound))
			}
		})
	})
})
