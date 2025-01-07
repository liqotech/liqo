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

// Package clusterlabels tests the cluster labels.
package clusterlabels

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	liqoctlutil "github.com/liqotech/liqo/pkg/liqoctl/utils"
	argsutils "github.com/liqotech/liqo/pkg/utils/args"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreigncluster"
	"github.com/liqotech/liqo/test/e2e/testconsts"
	"github.com/liqotech/liqo/test/e2e/testutils/config"
	"github.com/liqotech/liqo/test/e2e/testutils/tester"
	"github.com/liqotech/liqo/test/e2e/testutils/util"
)

const (
	// clustersRequired is the number of clusters required in this E2E test.
	clustersRequired = 3
	// testName is the name of this E2E test.
	testName = "CLUSTER_LABELS"
)

var (
	ctx         = context.Background()
	testContext = tester.GetTester(ctx)
	interval    = config.Interval
	// shortTimeout is used for Consistently statement.
	shortTimeout = config.TimeoutConsistently
	timeout      = config.Timeout
	// namespaceName is the name of the test namespace for this test.
	namespaceName = util.GetNameNamespaceTest(testName)
	localIndex    = 0

	getTableEntries = func(role *liqov1beta1.RoleType) []TableEntry {
		res := []TableEntry{}
		for i := 0; i < 3; i++ {
			// If the role is specified, check only the clusters that match the role.
			if role != nil && testContext.Clusters[i].Role != *role {
				continue
			}
			res = append(res, Entry(
				fmt.Sprintf("Check the labels of the cluster %v", i+1),
				testContext.Clusters[i],
				i,
				util.GetClusterLabels(i),
			))
		}
		return res
	}
)

func TestE2E(t *testing.T) {
	util.CheckIfTestIsSkipped(t, clustersRequired, testName)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Liqo E2E Suite")
}

var _ = Describe("Liqo E2E", func() {
	Context("Assert that labels inserted at installation time are in the right resources: ControllerManager args", func() {

		DescribeTable(" 1 - Check labels presence in the ControllerManager arguments for every cluster", util.DescribeTableArgs(
			func(cluster tester.ClusterContext, index int, clusterLabels map[string]string) {
				args, err := liqoctlutil.RetrieveLiqoControllerManagerDeploymentArgs(ctx, cluster.ControllerClient, "liqo")
				Expect(err).ToNot(HaveOccurred())

				val, err := liqoctlutil.ExtractValuesFromArgumentList("--cluster-labels", args)
				Expect(err).To(Succeed())

				labels := argsutils.StringMap{}
				Expect(labels.Set(val)).To(Succeed())

				for key, value := range clusterLabels {
					Expect(labels.StringMap).To(HaveKeyWithValue(key, value))
				}
			},
			getTableEntries(nil)...,
		)...)

		DescribeTable(" 2 - Check labels presence on the virtual nodes for every cluster", util.DescribeTableArgs(
			// Each virtual node representing the cluster under examination in the remote clusters must have the
			// expected labels.
			func(provider tester.ClusterContext, index int, clusterLabels map[string]string) {
				virtualNode := &corev1.Node{}
				virtualNodeName := string(provider.Cluster)
				for i := range testContext.Clusters {
					// Skip cluster under examination.
					if i == index {
						continue
					}
					// Skip clusters that are not consumers since there is no virtual node.
					if testContext.Clusters[i].Role != liqov1beta1.ConsumerRole {
						continue
					}
					Eventually(func() error {
						return testContext.Clusters[i].ControllerClient.Get(ctx,
							types.NamespacedName{Name: virtualNodeName}, virtualNode)
					}, timeout, interval).Should(BeNil())
					for key, value := range clusterLabels {
						Expect(virtualNode.Labels).To(HaveKeyWithValue(key, value))
					}
				}
			},
			getTableEntries(ptr.To(liqov1beta1.ProviderRole))...,
		)...)

	})

	// In these test cases it is created a namespace only inside one cluster
	Context(fmt.Sprintf("Create a namespace in the cluster '%d' with its NamespaceOffloading and check if the remote namespaces "+
		"are created on the right remote cluster according to the ClusterSelector specified in the NamespaceOffloading Spec ", localIndex), func() {

		selector := metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{Key: testconsts.RegionKey, Operator: metav1.LabelSelectorOpIn, Values: []string{testconsts.RegionB}},
				{Key: testconsts.ProviderKey, Operator: metav1.LabelSelectorOpIn, Values: []string{testconsts.ProviderAWS}},
			},
		}

		It("Creating the namespace and checking the presence of the remote namespaces", func() {
			By(" 1 - Creating the local namespace without the NamespaceOffloading resource")
			Eventually(func() error {
				_, err := util.EnforceNamespace(ctx, testContext.Clusters[localIndex].NativeClient,
					testContext.Clusters[localIndex].Cluster, namespaceName)
				return err
			}, timeout, interval).Should(BeNil())

			By(" 2 - Create the NamespaceOffloading resource associated with the previously created namespace")
			Expect(util.OffloadNamespace(testContext.Clusters[localIndex].KubeconfigPath, namespaceName,
				"--namespace-mapping-strategy", "EnforceSameName", "--pod-offloading-strategy", "LocalAndRemote",
				"--selector", metav1.FormatLabelSelector(&selector))).To(Succeed())

			By(fmt.Sprintf(" 3 - Getting the virtual nodes in the cluster '%d'", localIndex))
			virtualNodesList := &corev1.NodeList{}
			Eventually(func() error {
				return testContext.Clusters[localIndex].ControllerClient.List(ctx, virtualNodesList,
					client.MatchingLabels{liqoconst.TypeLabel: liqoconst.TypeNode})
			}, timeout, interval).Should(BeNil())
			Expect(len(virtualNodesList.Items)).To(Equal(testContext.Clusters[localIndex].NumPeeredProviders))

			By(" 4 - Checking the remote clusters on which the remote namespaces are created")
			for i := range virtualNodesList.Items {
				ls, err := metav1.LabelSelectorAsSelector(&selector)
				Expect(err).ToNot(HaveOccurred())

				match := ls.Matches(labels.Set(virtualNodesList.Items[i].Labels))
				remoteClusterID := virtualNodesList.Items[i].Labels[liqoconst.RemoteClusterID]

				var cl kubernetes.Interface
				var id liqov1beta1.ClusterID
				for j := range testContext.Clusters {
					cluster := &testContext.Clusters[j]
					if string(cluster.Cluster) == remoteClusterID {
						cl = cluster.NativeClient
						id = cluster.Cluster
						break
					}
				}
				Expect(cl).ToNot(BeNil())

				if match {
					// Check if the remote namespace is correctly created.
					By(fmt.Sprintf(" 5 - Checking if a remote namespace is correctly created inside cluster '%s'", remoteClusterID))
					namespace := &corev1.Namespace{}

					Eventually(func() error {
						namespace, err = cl.CoreV1().Namespaces().Get(ctx, namespaceName, metav1.GetOptions{})
						return err
					}, timeout, interval).Should(BeNil())

					value, ok := namespace.Annotations[liqoconst.RemoteNamespaceManagedByAnnotationKey]
					Expect(ok).To(BeTrue())
					Expect(value).To(HaveSuffix(foreignclusterutils.UniqueName(id)))
				} else {
					// Check if the remote namespace does not exists.
					By(fmt.Sprintf(" 5 - Checking that no remote namespace is created inside cluster '%s'", remoteClusterID))
					Consistently(func() metav1.StatusReason {
						_, err = cl.CoreV1().Namespaces().Get(ctx, namespaceName, metav1.GetOptions{})
						return apierrors.ReasonForError(err)
					}, shortTimeout, interval).Should(Equal(metav1.StatusReasonNotFound))
				}

			}

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
			}, timeout, interval).Should(Equal(metav1.StatusReasonNotFound))

			// When the NamespaceOffloading resource is really deleted the remote namespaces must be already deleted.
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
