package remotenamespacecreation

import (
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	offloadingv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	virtualkubeletv1alpha1 "github.com/liqotech/liqo/apis/virtualKubelet/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/test/e2e/testconsts"
	"github.com/liqotech/liqo/test/e2e/testutils/tester"
	testutils "github.com/liqotech/liqo/test/e2e/testutils/util"
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Liqo E2E Suite")
}

const (
	// local name of the test namespace.
	testNamespaceName = "test-namespace-creation"
)

var _ = Describe("Liqo E2E", func() {
	var (
		ctx         = context.Background()
		testContext = tester.GetTester(ctx)
		interval    = 1 * time.Second
		timeout     = 10 * time.Second
		// longTimeout is used in situations that may take longer to be performed
		longTimeout = 2 * time.Minute
		localIndex  = 0
		// index of the cluster on which the remote namespace is deleted to test the recreation process.
		remoteIndex             = 2
		localClusterID          = testContext.Clusters[localIndex].ClusterID
		remoteTestNamespaceName = fmt.Sprintf("%s-%s", testNamespaceName, localClusterID)
	)

	Context(fmt.Sprintf("Create a namespace inside the cluster '%d' with the liqo enabling label and check if the remote namespaces"+
		"are created inside all remote clusters. Remove the label and check the deletion of the remote namespaces.", localIndex), func() {

		It(fmt.Sprintf("Create a namespace inside the cluster '%d' with the liqo enabling label and check if the remote namespaces"+
			"are created inside all remote clusters", localIndex), func() {
			namespace := &corev1.Namespace{}
			namespaceOffloading := &offloadingv1alpha1.NamespaceOffloading{}
			namespaceMapsList := &virtualkubeletv1alpha1.NamespaceMapList{}

			By(fmt.Sprintf(" 1 - Creating the local namespace inside the cluster %d", localIndex))
			Eventually(func() error {
				return testutils.CreateNamespaceWithNamespaceOffloading(ctx, testContext.Clusters[localIndex].ControllerClient, testNamespaceName)
			}, timeout, interval).Should(Succeed())

			By(" 2 - Getting the just created NamespaceOffloading resource")
			Eventually(func() error {
				return testContext.Clusters[localIndex].ControllerClient.Get(ctx,
					types.NamespacedName{Namespace: testNamespaceName, Name: liqoconst.DefaultNamespaceOffloadingName}, namespaceOffloading)
			}, timeout, interval).Should(Succeed())

			By(" 3 - Getting the NamespaceMaps and check the presence of the entries for that namespace, both in the spec and status")
			Eventually(func() error {
				if err := testContext.Clusters[localIndex].ControllerClient.List(ctx, namespaceMapsList); err != nil {
					return err
				}
				Expect(len(namespaceMapsList.Items)).To(Equal(testconsts.NumberOfTestClusters - 1))
				for i := range namespaceMapsList.Items {
					desiredMapping, desiredMappingPresence := namespaceMapsList.Items[i].Spec.DesiredMapping[testNamespaceName]
					if !desiredMappingPresence {
						return fmt.Errorf(" In the NamespaceMap corresponding to the cluster '%s', "+
							"there is no DesiredMapping for the namespace '%s'",
							namespaceMapsList.Items[i].Labels[liqoconst.RemoteClusterID], testNamespaceName)
					}
					if desiredMapping != remoteTestNamespaceName {
						return fmt.Errorf(" In the NamespaceMap corresponding to the cluster '%s', "+
							"the DesiredMapping for the namespace '%s' has the wrong value: '%s'",
							namespaceMapsList.Items[i].Labels[liqoconst.RemoteClusterID], testNamespaceName,
							desiredMapping)
					}
					currentMapping, currentMappingPresence := namespaceMapsList.Items[i].Status.CurrentMapping[testNamespaceName]
					if !currentMappingPresence {
						return fmt.Errorf(" In the NamespaceMap corresponding to the cluster '%s', "+
							"there is no CurrentMapping for the namespace '%s'",
							namespaceMapsList.Items[i].Labels[liqoconst.RemoteClusterID], testNamespaceName)
					}
					if currentMapping.RemoteNamespace != remoteTestNamespaceName {
						return fmt.Errorf(" In the NamespaceMap corresponding to the cluster '%s', "+
							"the CurrentMapping for the namespace '%s' has the wrong value: '%s'",
							namespaceMapsList.Items[i].Labels[liqoconst.RemoteClusterID], testNamespaceName,
							currentMapping.RemoteNamespace)
					}
					if currentMapping.Phase != virtualkubeletv1alpha1.MappingAccepted {
						return fmt.Errorf(" In the NamespaceMap corresponding to the cluster '%s', "+
							"the CurrentMapping for the namespace '%s' has the wrong phase: '%s'",
							namespaceMapsList.Items[i].Labels[liqoconst.RemoteClusterID], testNamespaceName,
							currentMapping.Phase)
					}
				}
				return nil
			}, longTimeout, interval).Should(Succeed())

			By(" 4 - Checking the presence of the remote namespaces")
			for i := range testContext.Clusters {
				if i == localIndex {
					continue
				}
				Eventually(func() error {
					return testContext.Clusters[i].ControllerClient.Get(ctx,
						types.NamespacedName{Name: remoteTestNamespaceName}, namespace)
				}, longTimeout, interval).Should(Succeed())
				value, ok := namespace.Annotations[liqoconst.RemoteNamespaceAnnotationKey]
				Expect(ok).To(Equal(true))
				Expect(value).To(Equal(localClusterID))
			}

			By(fmt.Sprintf(" 5 - Deleting the remote namespace inside the cluster '%d'", remoteIndex))
			Eventually(func() metav1.StatusReason {
				err := testContext.Clusters[remoteIndex].ControllerClient.Get(ctx,
					types.NamespacedName{Name: remoteTestNamespaceName}, namespace)
				_ = testContext.Clusters[remoteIndex].ControllerClient.Delete(ctx, namespace)
				return apierrors.ReasonForError(err)
			}, timeout, interval).Should(Equal(metav1.StatusReasonNotFound))

			By(fmt.Sprintf(" 6 - Checking that the remote namespace inside the cluster '%d' has been recreated", remoteIndex))
			Eventually(func() error {
				return testContext.Clusters[remoteIndex].ControllerClient.Get(ctx,
					types.NamespacedName{Name: remoteTestNamespaceName}, namespace)
			}, longTimeout, interval).Should(Succeed())
		})

		It("Remove the label and check the deletion of the remote namespaces.", func() {
			namespace := &corev1.Namespace{}
			namespaceOffloading := &offloadingv1alpha1.NamespaceOffloading{}
			namespaceMapsList := &virtualkubeletv1alpha1.NamespaceMapList{}

			By(" 1 - Getting the local namespace inside the cluster 1, and remove the liqo enabling label")
			Eventually(func() error {
				if err := testContext.Clusters[localIndex].ControllerClient.Get(ctx, types.NamespacedName{Name: testNamespaceName}, namespace); err != nil {
					return err
				}
				delete(namespace.Labels, liqoconst.EnablingLiqoLabel)
				return testContext.Clusters[localIndex].ControllerClient.Update(ctx, namespace)
			}, timeout, interval).Should(Succeed())

			By(" 2 - Checking if the NamespaceOffloading resource associated with the test namespace is correctly removed.")
			Eventually(func() metav1.StatusReason {
				return apierrors.ReasonForError(testContext.Clusters[localIndex].ControllerClient.Get(ctx, types.NamespacedName{
					Name: liqoconst.DefaultNamespaceOffloadingName, Namespace: testNamespaceName}, namespaceOffloading))
			}, longTimeout, interval).Should(Equal(metav1.StatusReasonNotFound))

			By(" 3 - Checking if the NamespaceMaps do not have the entries for that test namespace")
			Eventually(func() error {
				return testContext.Clusters[localIndex].ControllerClient.List(ctx, namespaceMapsList)
			}, timeout, interval).Should(Succeed())
			Expect(len(namespaceMapsList.Items)).To(Equal(testconsts.NumberOfTestClusters - 1))
			for i := range namespaceMapsList.Items {
				_, ok := namespaceMapsList.Items[i].Spec.DesiredMapping[testNamespaceName]
				Expect(ok).To(Equal(false))
				_, ok = namespaceMapsList.Items[i].Status.CurrentMapping[testNamespaceName]
				Expect(ok).To(Equal(false))
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

			By(" 5 - Deleting the local test namespace")
			Eventually(func() metav1.StatusReason {
				err := testContext.Clusters[localIndex].ControllerClient.Get(ctx, types.NamespacedName{Name: testNamespaceName}, namespace)
				_ = testContext.Clusters[localIndex].ControllerClient.Delete(ctx, namespace)
				return apierrors.ReasonForError(err)
			}, longTimeout, interval).Should(Equal(metav1.StatusReasonNotFound))
		})
	})
})
