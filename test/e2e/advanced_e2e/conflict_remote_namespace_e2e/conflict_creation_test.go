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
	"github.com/liqotech/liqo/test/e2e/testutils/util"
)

const (
	// clustersRequired is the number of clusters required in this E2E test.
	clustersRequired = 4
	// testNamespaceName is the name of the test namespace for this test.
	testNamespaceName = "test-namespace-conflict"
	// controllerClientPresence indicates if the test use the controller runtime clients.
	controllerClientPresence = true
	// testName is the name of this E2E test.
	testName = "E2E_CONFLICT_CREATION"
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
				_, err := util.EnforceNamespace(ctx, testContext.Clusters[remoteIndex].NativeClient,
					testContext.Clusters[remoteIndex].ClusterID, remoteTestNamespaceName,
					util.GetNamespaceLabel(false))
				return err
			}, timeout, interval).Should(BeNil())

			By(fmt.Sprintf(" 2 - Creating the local namespace inside the cluster '%d'", localIndex))
			Eventually(func() error {
				_, err := util.EnforceNamespace(ctx, testContext.Clusters[localIndex].NativeClient,
					testContext.Clusters[localIndex].ClusterID, testNamespaceName,
					util.GetNamespaceLabel(true))
				return err
			}, timeout, interval).Should(BeNil())

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
			}, longTimeout, interval).Should(BeNil())

			// This remote namespace has not the annotation inserted by the NamespaceMap controller,
			// it has been created in the STEP 1 without that annotation.
			By(fmt.Sprintf(" 4 - Deleting the remote namespace inside the cluster '%d'", remoteIndex))
			Eventually(func() error {
				if err := testContext.Clusters[remoteIndex].ControllerClient.Get(ctx,
					types.NamespacedName{Name: remoteTestNamespaceName}, namespace); err != nil {
					return err
				}
				return testContext.Clusters[remoteIndex].ControllerClient.Delete(ctx, namespace)
			}, timeout, interval).Should(BeNil())

			By(fmt.Sprintf(" 5 - Checking that the remote namespace with the right "+
				"annotation has been created inside the cluster '%d' by the NamespaceMap controller", remoteIndex))
			Eventually(func() error {
				if err := testContext.Clusters[remoteIndex].ControllerClient.Get(ctx,
					types.NamespacedName{Name: remoteTestNamespaceName}, namespace); err != nil {
					return err
				}
				if value, ok := namespace.Annotations[liqoconst.RemoteNamespaceAnnotationKey]; !ok || value != localClusterID {
					return fmt.Errorf("the remote namespace has not the right Liqo annotation")
				}
				return nil
			}, longTimeout, interval).Should(BeNil())
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

			// When the NamespaceOffloading resource is really deleted the remote namespace must be already deleted.
			By(" 2 - Checking that all remote namespaces are deleted")
			namespace := &corev1.Namespace{}
			for i := range testContext.Clusters {
				if i == localIndex {
					continue
				}
				Eventually(func() metav1.StatusReason {
					return apierrors.ReasonForError(testContext.Clusters[i].ControllerClient.Get(ctx,
						types.NamespacedName{Name: remoteTestNamespaceName}, namespace))
				}, timeout, interval).Should(Equal(metav1.StatusReasonNotFound))
			}

			// Cleaning the environment after the test.
			By(" 3 - Getting the local namespace and delete it")
			Eventually(func() error {
				return util.EnsureNamespaceDeletion(ctx, testContext.Clusters[localIndex].NativeClient, util.GetNamespaceLabel(true))
			}, longTimeout, interval).Should(BeNil())
		})
	})
})
