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

// Package conflictremotenamespace tests the conflicting creation of remote namespaces.
package conflictremotenamespace

import (
	"context"
	"fmt"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreigncluster"
	"github.com/liqotech/liqo/test/e2e/testutils/config"
	"github.com/liqotech/liqo/test/e2e/testutils/tester"
	"github.com/liqotech/liqo/test/e2e/testutils/util"
)

const (
	// clustersRequired is the number of clusters required in this E2E test.
	clustersRequired = 3
	// testName is the name of this E2E test.
	testName = "CONFLICT_REMOTE_NAMESPACE"
)

var (
	ctx           = context.Background()
	testContext   = tester.GetTester(ctx)
	interval      = config.Interval
	timeout       = config.Timeout
	localIndex    = 0
	namespaceName = util.GetNameNamespaceTest(testName)

	// index of the cluster on which a remote namespace with the same name already exists.
	remoteIndex = 2
)

func TestE2E(t *testing.T) {
	util.CheckIfTestIsSkipped(t, clustersRequired, testName)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Liqo E2E Suite")
}

var _ = Describe("Liqo E2E", func() {

	Context(fmt.Sprintf("Create a namespace inside the cluster '%d' and check what happen if a remote namespaace in the cluster "+
		"'%d' already exists. Remove the label and check the deletion of the remote namespaces.", localIndex, remoteIndex), func() {

		It(fmt.Sprintf("Create a namespace inside the cluster '%d' and check what happen "+
			"if a remote namespaace in the cluster '%d' already exists.", localIndex, remoteIndex), func() {
			namespace := &corev1.Namespace{}
			namespaceOffloading := &offloadingv1beta1.NamespaceOffloading{}

			By(fmt.Sprintf(" 1 - Creating the remote namespace inside the cluster '%d'", remoteIndex))
			Eventually(func() error {
				_, err := util.EnforceNamespace(ctx, testContext.Clusters[remoteIndex].NativeClient,
					testContext.Clusters[remoteIndex].Cluster, namespaceName)
				return err
			}, timeout, interval).Should(BeNil())

			By(fmt.Sprintf(" 2 - Creating the local namespace inside the cluster '%d'", localIndex))
			Eventually(func() error {
				if _, err := util.EnforceNamespace(ctx, testContext.Clusters[localIndex].NativeClient,
					testContext.Clusters[localIndex].Cluster, namespaceName); err != nil {
					return err
				}

				// Do not use liqoctl to create the resource, since it will fail waiting for offloading to complete.
				return util.CreateNamespaceOffloading(ctx, testContext.Clusters[localIndex].ControllerClient, namespaceName,
					offloadingv1beta1.EnforceSameNameMappingStrategyType, offloadingv1beta1.LocalAndRemotePodOffloadingStrategyType)

			}, timeout, interval).Should(BeNil())

			By(" 3 - Getting the NamespaceOffloading resource")
			Eventually(func() error {
				if err := testContext.Clusters[localIndex].ControllerClient.Get(ctx,
					types.NamespacedName{Namespace: namespaceName, Name: liqoconst.DefaultNamespaceOffloadingName}, namespaceOffloading); err != nil {
					return err
				}
				if namespaceOffloading.Status.OffloadingPhase != offloadingv1beta1.SomeFailedOffloadingPhaseType {
					return fmt.Errorf("the NamespaceOffloading resource has the wrong OffloadingPhase: %s",
						namespaceOffloading.Status.OffloadingPhase)
				}
				return nil
			}, timeout, interval).Should(BeNil())

			// This remote namespace has not the annotation inserted by the NamespaceMap controller,
			// it has been created in the STEP 1 without that annotation.
			By(fmt.Sprintf(" 4 - Deleting the remote namespace inside the cluster '%d'", remoteIndex))
			Eventually(func() error {
				if err := testContext.Clusters[remoteIndex].ControllerClient.Get(ctx,
					types.NamespacedName{Name: namespaceName}, namespace); err != nil {
					return err
				}
				return testContext.Clusters[remoteIndex].ControllerClient.Delete(ctx, namespace)
			}, timeout, interval).Should(BeNil())

			By(fmt.Sprintf(" 5 - Checking that the remote namespace with the right "+
				"annotation has been created inside the cluster '%d' by the NamespaceMap controller", remoteIndex))
			Eventually(func() error {
				if err := testContext.Clusters[remoteIndex].ControllerClient.Get(ctx,
					types.NamespacedName{Name: namespaceName}, namespace); err != nil {
					return err
				}
				value, ok := namespace.Annotations[liqoconst.RemoteNamespaceManagedByAnnotationKey]
				suffix := foreignclusterutils.UniqueName(testContext.Clusters[remoteIndex].Cluster)
				if !ok || !strings.HasSuffix(value, suffix) {
					return fmt.Errorf("the remote namespace has not the right Liqo annotation, found: %q, expected suffix: %q", value, suffix)
				}
				return nil
			}, 3*timeout /* namespace deletion is slow */, interval).Should(BeNil())
		})

		It("Delete the NamespaceOffloading resource in the local namespace "+
			"and check if the remote namespaces are deleted", func() {
			By(" 1 - Getting the NamespaceOffloading in the local namespace and delete it")
			namespaceOffloading := &offloadingv1beta1.NamespaceOffloading{}
			Eventually(func() metav1.StatusReason {
				err := testContext.Clusters[localIndex].ControllerClient.Get(ctx,
					types.NamespacedName{Name: liqoconst.DefaultNamespaceOffloadingName, Namespace: namespaceName},
					namespaceOffloading)
				_ = testContext.Clusters[localIndex].ControllerClient.Delete(ctx, namespaceOffloading)
				return apierrors.ReasonForError(err)
			}, 3*timeout /* namespace deletion is slow */, interval).Should(Equal(metav1.StatusReasonNotFound))

			// When the NamespaceOffloading resource is really deleted the remote namespace must be already deleted.
			By(" 2 - Checking that all remote namespaces are deleted")
			namespace := &corev1.Namespace{}
			for i := range testContext.Clusters {
				if i == localIndex {
					continue
				}
				Eventually(func() metav1.StatusReason {
					return apierrors.ReasonForError(testContext.Clusters[i].ControllerClient.Get(ctx,
						types.NamespacedName{Name: namespaceName}, namespace))
				}, timeout, interval).Should(Equal(metav1.StatusReasonNotFound))
			}
		})
	})
})

var _ = AfterSuite(func() {
	for i := range testContext.Clusters {
		Eventually(func() error {
			return util.EnsureNamespaceDeletion(ctx, testContext.Clusters[i].NativeClient, namespaceName)
		}, timeout, interval).Should(Succeed())
	}
})
