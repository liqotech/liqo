package clusterlabels

import (
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	k8shelper "k8s.io/component-helpers/scheduling/corev1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
	offloadingv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	liqoutils "github.com/liqotech/liqo/pkg/utils/foreignCluster"
	"github.com/liqotech/liqo/test/e2e/testutils/tester"
	"github.com/liqotech/liqo/test/e2e/testutils/util"
)

const (
	// clustersRequired is the number of clusters required in this E2E test.
	clustersRequired = 4
	// clusterConfigName is the name of the ClusterConfig in every cluster with Liqo.
	clusterConfigName = "liqo-configuration"
	// testNamespaceName is the name of the test namespace for this test.
	testNamespaceName = "test-namespace-labels"
	// controllerClientPresence indicates if the test use the controller runtime clients.
	controllerClientPresence = true
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
		testContext = tester.GetTester(ctx, clustersRequired, controllerClientPresence)
		interval    = 1 * time.Second
		// shortTimeout is used for Consistently statement
		shortTimeout = 5 * time.Second
		timeout      = 10 * time.Second
		// longTimeout is used in situations that may take longer to be performed
		longTimeout = 2 * time.Minute
		localIndex  = 0
	)

	Context("Assert that labels inserted at installation time are in the right resources: clusterConfig,"+
		" resourceOffer and virtualNodes", func() {

		DescribeTable(" 1 - Check labels presence in the ClusterConfig resources for every cluster",
			// Every cluster must have in its ClusterConfig Resource, the labels inserted at installation time.
			func(cluster tester.ClusterContext, clusterLabels map[string]string) {
				clusterConfig := &configv1alpha1.ClusterConfig{}
				Eventually(func() error {
					return cluster.ControllerClient.Get(ctx, types.NamespacedName{Name: clusterConfigName}, clusterConfig)
				}, timeout, interval).Should(BeNil())
				Expect(clusterConfig.Spec.DiscoveryConfig.ClusterLabels).To(Equal(clusterLabels))
			},
			Entry("Check the ClusterConfig resource of the cluster 1", testContext.Clusters[0], util.GetClusterLabels(0)),
			Entry("Check the ClusterConfig resource of the cluster 2", testContext.Clusters[1], util.GetClusterLabels(1)),
			Entry("Check the ClusterConfig resource of the cluster 3", testContext.Clusters[2], util.GetClusterLabels(2)),
			Entry("Check the ClusterConfig resource of the cluster 4", testContext.Clusters[3], util.GetClusterLabels(3)),
		)

		DescribeTable(" 2 - Check labels presence in the ResourceOffer resources for every cluster",
			// In every Local Tenant Namespace there must be the ResourceOffer sent by the cluster under examination
			// with the expected labels in the field ResourceOffer.Spec.Labels.
			func(cluster tester.ClusterContext, index int, clusterLabels map[string]string) {
				resourceOffer := &sharingv1alpha1.ResourceOffer{}
				// The name prefix is useful in order to get every ResourceOffer by name.
				resourceOfferNamePrefix := "resourceoffer"
				// For every peering get the resourceOffer sent by the cluster under examination.
				for i := range testContext.Clusters {
					if i == index {
						continue
					}
					By("Getting the local tenant namespace corresponding to the right cluster and getting the " +
						"ResourceOffer sent by the cluster under examination")
					Eventually(func() error {
						tenantNamespaceName, err := liqoutils.GetLocalTenantNamespaceName(ctx,
							cluster.ControllerClient, testContext.Clusters[i].ClusterID)
						if err != nil {
							return err
						}
						return cluster.ControllerClient.Get(ctx, types.NamespacedName{
							Namespace: tenantNamespaceName,
							Name:      fmt.Sprintf("%s-%s", resourceOfferNamePrefix, cluster.ClusterID),
						}, resourceOffer)
					}, timeout, interval).Should(BeNil())
					Expect(resourceOffer.Spec.Labels).To(Equal(clusterLabels))
				}
			},
			Entry("Check the ResourceOffer resources of the cluster 1", testContext.Clusters[0], 0, util.GetClusterLabels(0)),
			Entry("Check the ResourceOffer resources of the cluster 2", testContext.Clusters[1], 1, util.GetClusterLabels(1)),
			Entry("Check the ResourceOffer resources of the cluster 3", testContext.Clusters[2], 2, util.GetClusterLabels(2)),
			Entry("Check the ResourceOffer resources of the cluster 4", testContext.Clusters[3], 3, util.GetClusterLabels(3)),
		)

		DescribeTable(" 3 - Check labels presence on the virtual nodes for every cluster",
			// Each virtual node representing the cluster under examination in the remote clusters must have the
			// expected labels.
			func(cluster tester.ClusterContext, index int, clusterLabels map[string]string) {
				virtualNode := &corev1.Node{}
				liqoPrefix := "liqo"
				virtualNodeName := fmt.Sprintf("%s-%s", liqoPrefix, cluster.ClusterID)
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
			Entry("Check the virtual node of the cluster 1", testContext.Clusters[0], 0, util.GetClusterLabels(0)),
			Entry("Check the virtual node of the cluster 2", testContext.Clusters[1], 1, util.GetClusterLabels(1)),
			Entry("Check the virtual node of the cluster 3", testContext.Clusters[2], 2, util.GetClusterLabels(2)),
			Entry("Check the virtual node of the cluster 4", testContext.Clusters[3], 3, util.GetClusterLabels(3)),
		)

	})

	// In these test cases it is created a namespace only inside one cluster
	Context(fmt.Sprintf("Create a namespace in the cluster '%d' with its NamespaceOffloading and check if the remote namespaces"+
		"are created on the right remote cluster according to the ClusterSelector specified in the NamespaceOffloading Spec ", localIndex), func() {

		It("Creating the namespace and checking the presence of the remote namespaces", func() {
			By(" 1 - Creating the local namespace without the NamespaceOffloading resource")
			Eventually(func() error {
				_, err := util.EnforceNamespace(ctx, testContext.Clusters[localIndex].NativeClient,
					testContext.Clusters[localIndex].ClusterID,
					testNamespaceName, util.GetNamespaceLabel(false))
				return err
			}, timeout, interval).Should(BeNil())

			By(" 2 - Create the NamespaceOffloading resource associated with the previously created namespace")
			Eventually(func() error {
				return util.CreateNamespaceOffloading(ctx, testContext.Clusters[localIndex].ControllerClient, testNamespaceName,
					offloadingv1alpha1.EnforceSameNameMappingStrategyType,
					offloadingv1alpha1.LocalAndRemotePodOffloadingStrategyType,
					util.GetClusterSelector())
			}, timeout, interval).Should(BeNil())

			By(fmt.Sprintf(" 3 - Getting the virtual nodes in the cluster '%d'", localIndex))
			virtualNodesList := &corev1.NodeList{}
			Eventually(func() error {
				return testContext.Clusters[localIndex].ControllerClient.List(ctx, virtualNodesList,
					client.MatchingLabels{liqoconst.TypeLabel: liqoconst.TypeNode})
			}, timeout, interval).Should(BeNil())
			Expect(len(virtualNodesList.Items)).To(Equal(clustersRequired - 1))

			By(" 4 - Checking the remote clusters on which the remote namespaces are created")
			for i := range virtualNodesList.Items {
				match, err := k8shelper.MatchNodeSelectorTerms(&virtualNodesList.Items[i], util.GetClusterSelector())
				Expect(err).To(BeNil())
				remoteClusterID := virtualNodesList.Items[i].Annotations[liqoconst.RemoteClusterID]
				if match {
					// Check if the remote namespace is correctly created.
					By(fmt.Sprintf(" 5 - Checking if a remote namespace is correctly created inside cluster '%s'", remoteClusterID))
					namespace := &corev1.Namespace{}
					Eventually(func() error {
						return testContext.ClustersClients[remoteClusterID].Get(ctx,
							types.NamespacedName{Name: testNamespaceName}, namespace)
					}, timeout, interval).Should(BeNil())
					value, ok := namespace.Annotations[liqoconst.RemoteNamespaceAnnotationKey]
					Expect(ok).To(BeTrue())
					Expect(value).To(Equal(testContext.Clusters[localIndex].ClusterID))
				} else {
					// Check if the remote namespace does not exists.
					By(fmt.Sprintf(" 5 - Checking that no remote namespace is created inside cluster '%s'", remoteClusterID))
					Consistently(func() metav1.StatusReason {
						namespace := &corev1.Namespace{}
						return apierrors.ReasonForError(testContext.ClustersClients[remoteClusterID].Get(ctx,
							types.NamespacedName{Name: testNamespaceName}, namespace))
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

			// Cleaning the environment after the test.
			By(" 3 - Getting the local namespace and delete it")
			Eventually(func() error {
				return util.EnsureNamespaceDeletion(ctx, testContext.Clusters[localIndex].NativeClient, util.GetNamespaceLabel(false))
			}, timeout, interval).Should(BeNil())
		})
	})
})
