package conflictremotenamespace

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
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/test/e2e/testutils/tester"
	testutils "github.com/liqotech/liqo/test/e2e/testutils/util"
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Liqo E2E Suite")
}

const (
	// local name of the test namespace.
	testNamespaceName = "test-namespace-conflict"
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
		// index of the cluster on which a remote namespace with the same name already exists.
		remoteIndex             = 2
		localClusterID          = testContext.Clusters[localIndex].ClusterID
		remoteTestNamespaceName = fmt.Sprintf("%s-%s", testNamespaceName, localClusterID)
	)

	Context(fmt.Sprintf("Create a namespace inside the cluster '%d' and check what happen if a remote namespaace in the cluster "+
		"'%d' already exists. Remove the label and check the deletion of the remote namespaces.", localIndex, remoteIndex), func() {

		It(fmt.Sprintf("Create a namespace inside the cluster '%d' and check what happen "+
			"if a remote namespaace in the cluster '%d' already exists.", localIndex, remoteIndex), func() {
			namespace := &corev1.Namespace{}
			namespaceOffloading := &offloadingv1alpha1.NamespaceOffloading{}

			By(fmt.Sprintf(" 1 - Creating the remote namespace inside the cluster '%d'", remoteIndex))
			Eventually(func() error {
				return testutils.CreateNamespaceWithoutNamespaceOffloading(ctx, testContext.Clusters[remoteIndex].ControllerClient, remoteTestNamespaceName)
			}, timeout, interval).Should(Succeed())

			By(fmt.Sprintf(" 2 - Creating the local namespace inside the cluster '%d'", localIndex))
			Eventually(func() error {
				return testutils.CreateNamespaceWithNamespaceOffloading(ctx, testContext.Clusters[localIndex].ControllerClient, testNamespaceName)
			}, timeout, interval).Should(Succeed())

			By(" 3 - Getting the NamespaceOffloading resource")
			Eventually(func() error {
				if err := testContext.Clusters[localIndex].ControllerClient.Get(ctx,
					types.NamespacedName{Namespace: testNamespaceName, Name: liqoconst.DefaultNamespaceOffloadingName}, namespaceOffloading); err != nil {
					return err
				}
				if namespaceOffloading.Status.OffloadingPhase != offloadingv1alpha1.SomeFailedOffloadingPhaseType {
					return fmt.Errorf("the NamespaceOffloading resource has the wrong OffloadingPhase: %s",
						namespaceOffloading.Status.OffloadingPhase)
				}
				return nil
			}, longTimeout, interval).Should(Succeed())

			By(fmt.Sprintf(" 4 - Deleting the remote namespace inside the cluster '%d'", remoteIndex))
			Eventually(func() metav1.StatusReason {
				err := testContext.Clusters[remoteIndex].ControllerClient.Get(ctx,
					types.NamespacedName{Name: remoteTestNamespaceName}, namespace)
				_ = testContext.Clusters[remoteIndex].ControllerClient.Delete(ctx, namespace)
				return apierrors.ReasonForError(err)
			}, timeout, interval).Should(Equal(metav1.StatusReasonNotFound))

			By(fmt.Sprintf(" 5 - Checking that the remote namespace inside the cluster '%d' has been recreated", remoteIndex))
			Eventually(func() error {
				if err := testContext.Clusters[remoteIndex].ControllerClient.Get(ctx,
					types.NamespacedName{Name: remoteTestNamespaceName}, namespace); err != nil {
					return err
				}
				if value, ok := namespace.Annotations[liqoconst.RemoteNamespaceAnnotationKey]; !ok || value != localClusterID {
					return fmt.Errorf("the remote namespace has not the right Liqo annotation")
				}
				return nil
			}, longTimeout, interval).Should(Succeed())

		})

		It("Delete the local namespace and check if the remote namespaces are deleted", func() {
			By(" 1 - Getting the local namespace and delete it")
			namespace := &corev1.Namespace{}
			Eventually(func() metav1.StatusReason {
				err := testContext.Clusters[localIndex].ControllerClient.Get(ctx, types.NamespacedName{Name: testNamespaceName}, namespace)
				_ = testContext.Clusters[localIndex].ControllerClient.Delete(ctx, namespace)
				return apierrors.ReasonForError(err)
			}, longTimeout, interval).Should(Equal(metav1.StatusReasonNotFound))

			// When the local namespace is really deleted the remote namespace must be already deleted.
			By(" 2 - Checking that all remote namespaces are deleted")
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
