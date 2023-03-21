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

// Package clusterlabels tests the cluster labels.
package clusterlabels

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
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	offloadingv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	liqoctlutil "github.com/liqotech/liqo/pkg/liqoctl/util"
	argsutils "github.com/liqotech/liqo/pkg/utils/args"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreignCluster"
	liqogetters "github.com/liqotech/liqo/pkg/utils/getters"
	liqolabels "github.com/liqotech/liqo/pkg/utils/labels"
	"github.com/liqotech/liqo/test/e2e/testconsts"
	"github.com/liqotech/liqo/test/e2e/testutils/tester"
	"github.com/liqotech/liqo/test/e2e/testutils/util"
)

const (
	// clustersRequired is the number of clusters required in this E2E test.
	clustersRequired = 4
	// testNamespaceName is the name of the test namespace for this test.
	testNamespaceName = "test-namespace-labels"
	// testName is the name of this E2E test.
	testName = "E2E_CLUSTER_LABELS"
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
		// shortTimeout is used for Consistently statement
		shortTimeout = 5 * time.Second
		timeout      = 10 * time.Second
		// longTimeout is used in situations that may take longer to be performed
		longTimeout = 2 * time.Minute
		localIndex  = 0

		getTableEntries = func() []TableEntry {
			res := []TableEntry{}
			for i := 0; i < 4; i++ {
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

	Context("Assert that labels inserted at installation time are in the right resources: ControllerManager args,"+
		" resourceOffer and virtualNodes", func() {

		DescribeTable(" 1 - Check labels presence in the ControllerManager arguments for every cluster", util.DescribeTableArgs(
			func(cluster tester.ClusterContext, index int, clusterLabels map[string]string) {
				args, err := liqoctlutil.RetrieveLiqoControllerManagerDeploymentArgs(ctx, cluster.ControllerClient, "liqo")
				Expect(err).ToNot(HaveOccurred())

				val, ok := liqoctlutil.ExtractValuesFromArgumentList("--cluster-labels", args)
				Expect(ok).To(BeTrue())

				labels := argsutils.StringMap{}
				Expect(labels.Set(val)).To(Succeed())

				for key, value := range clusterLabels {
					Expect(labels.StringMap).To(HaveKeyWithValue(key, value))
				}
			},
			getTableEntries()...,
		)...)

		DescribeTable(" 2 - Check labels presence in the ResourceOffer resources for every cluster", util.DescribeTableArgs(
			// In every Local Tenant Namespace there must be the ResourceOffer sent by the cluster under examination
			// with the expected labels in the field ResourceOffer.Spec.Labels.
			func(cluster tester.ClusterContext, index int, clusterLabels map[string]string) {
				resourceOffer := &sharingv1alpha1.ResourceOffer{}
				// For every peering get the resourceOffer sent by the cluster under examination.
				for i := range testContext.Clusters {
					if i == index {
						continue
					}

					By("Retrieving the ResourceOffers created by the cluster under examination")
					Eventually(func() (err error) {
						resourceOffer, err = liqogetters.GetResourceOfferByLabel(ctx, cluster.ControllerClient, metav1.NamespaceAll,
							liqolabels.LocalLabelSelectorForCluster(testContext.Clusters[i].Cluster.ClusterID))
						return err
					}, timeout, interval).Should(Succeed())

					for key, value := range clusterLabels {
						Expect(resourceOffer.Spec.Labels).To(HaveKeyWithValue(key, value))
					}
				}
			},
			getTableEntries()...,
		)...)

		DescribeTable(" 3 - Check labels presence on the virtual nodes for every cluster", util.DescribeTableArgs(
			// Each virtual node representing the cluster under examination in the remote clusters must have the
			// expected labels.
			func(cluster tester.ClusterContext, index int, clusterLabels map[string]string) {
				virtualNode := &corev1.Node{}
				liqoPrefix := "liqo"
				virtualNodeName := fmt.Sprintf("%s-%s", liqoPrefix, cluster.Cluster.ClusterName)
				for i := range testContext.Clusters {
					if i == index {
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
			getTableEntries()...,
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
					testContext.Clusters[localIndex].Cluster, testNamespaceName)
				return err
			}, timeout, interval).Should(BeNil())

			By(" 2 - Create the NamespaceOffloading resource associated with the previously created namespace")
			Expect(util.OffloadNamespace(testContext.Clusters[localIndex].KubeconfigPath, testNamespaceName,
				"--namespace-mapping-strategy", "EnforceSameName", "--pod-offloading-strategy", "LocalAndRemote",
				"--selector", metav1.FormatLabelSelector(&selector))).To(Succeed())

			By(fmt.Sprintf(" 3 - Getting the virtual nodes in the cluster '%d'", localIndex))
			virtualNodesList := &corev1.NodeList{}
			Eventually(func() error {
				return testContext.Clusters[localIndex].ControllerClient.List(ctx, virtualNodesList,
					client.MatchingLabels{liqoconst.TypeLabel: liqoconst.TypeNode})
			}, timeout, interval).Should(BeNil())
			Expect(len(virtualNodesList.Items)).To(Equal(clustersRequired - 1))

			By(" 4 - Checking the remote clusters on which the remote namespaces are created")
			for i := range virtualNodesList.Items {
				ls, err := metav1.LabelSelectorAsSelector(&selector)
				Expect(err).ToNot(HaveOccurred())

				match := ls.Matches(labels.Set(virtualNodesList.Items[i].Labels))
				remoteClusterID := virtualNodesList.Items[i].Labels[liqoconst.RemoteClusterID]

				var cl kubernetes.Interface
				var identity discoveryv1alpha1.ClusterIdentity
				for j := range testContext.Clusters {
					cluster := &testContext.Clusters[j]
					if cluster.Cluster.ClusterID == remoteClusterID {
						cl = cluster.NativeClient
						identity = cluster.Cluster
						break
					}
				}
				Expect(cl).ToNot(BeNil())

				if match {
					// Check if the remote namespace is correctly created.
					By(fmt.Sprintf(" 5 - Checking if a remote namespace is correctly created inside cluster '%s'", remoteClusterID))
					namespace := &corev1.Namespace{}

					Eventually(func() error {
						namespace, err = cl.CoreV1().Namespaces().Get(ctx, testNamespaceName, metav1.GetOptions{})
						return err
					}, timeout, interval).Should(BeNil())

					value, ok := namespace.Annotations[liqoconst.RemoteNamespaceManagedByAnnotationKey]
					Expect(ok).To(BeTrue())
					Expect(value).To(HaveSuffix(foreignclusterutils.UniqueName(&identity)))
				} else {
					// Check if the remote namespace does not exists.
					By(fmt.Sprintf(" 5 - Checking that no remote namespace is created inside cluster '%s'", remoteClusterID))
					Consistently(func() metav1.StatusReason {
						_, err = cl.CoreV1().Namespaces().Get(ctx, testNamespaceName, metav1.GetOptions{})
						return apierrors.ReasonForError(err)
					}, shortTimeout, interval).Should(Equal(metav1.StatusReasonNotFound))
				}

			}

		})

		It("Delete the NamespaceOffloading resource in the local namespace "+
			"and check if the remote namespaces are deleted", func() {
			By(" 1 - Getting the NamespaceOffloading in the local namespace and delete it")
			namespaceOffloading := &offloadingv1alpha1.NamespaceOffloading{}
			Eventually(func() metav1.StatusReason {
				err := testContext.Clusters[localIndex].ControllerClient.Get(ctx,
					types.NamespacedName{Name: liqoconst.DefaultNamespaceOffloadingName, Namespace: testNamespaceName},
					namespaceOffloading)
				_ = testContext.Clusters[localIndex].ControllerClient.Delete(ctx, namespaceOffloading)
				return apierrors.ReasonForError(err)
			}, longTimeout, interval).Should(Equal(metav1.StatusReasonNotFound))

			// When the NamespaceOffloading resource is really deleted the remote namespaces must be already deleted.
			By(" 2 - Checking that all remote namespaces are deleted")
			namespace := &corev1.Namespace{}
			for i := range testContext.Clusters {
				if i == localIndex {
					continue
				}
				Eventually(func() metav1.StatusReason {
					return apierrors.ReasonForError(testContext.Clusters[i].ControllerClient.Get(ctx,
						types.NamespacedName{Name: testNamespaceName}, namespace))
				}, timeout, interval).Should(Equal(metav1.StatusReasonNotFound))
			}
		})
	})
})
