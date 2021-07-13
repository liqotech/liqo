package unjoine2e

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	liqoconst "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/test/e2e/testutils/tester"
	"github.com/liqotech/liqo/test/e2e/testutils/util"
)

const (
	// clustersRequired is the number of clusters required in this E2E test.
	clustersRequired = 2
	// controllerClientPresence indicates if the test use the controller runtime clients.
	controllerClientPresence = false
	// testName is the name of this E2E test.
	testName = "E2E_UNJOIN"
)

func Test_Unjoin(t *testing.T) {
	util.CheckIfTestIsSkipped(t, clustersRequired, testName)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Liqo E2E Suite")
}

var _ = Describe("Liqo E2E", func() {
	var (
		ctx         = context.Background()
		testContext = tester.GetTester(ctx, clustersRequired, controllerClientPresence)
	)

	Describe("Assert that Liqo is correctly uninstalled", func() {
		It("Test Unjoin", func() {
			err := NoPods(testContext.Clusters[0].NativeClient, testContext.Namespace)
			Expect(err).ShouldNot(HaveOccurred())
			err = NoJoined(testContext.Clusters[0].NativeClient)
			Expect(err).ShouldNot(HaveOccurred())
			readyPods, notReadyPods, err := util.ArePodsUp(ctx, testContext.Clusters[1].NativeClient, testContext.Namespace)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(notReadyPods).Should(BeZero())
			Expect(len(readyPods)).Should(BeNumerically(">", 0))
		})
	},
	)
})

func NoPods(clientset *kubernetes.Clientset, namespace string) error {
	pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		klog.Error(err)
		return err
	}
	if len(pods.Items) > 0 {
		return fmt.Errorf("there are still running pods in Liqo namespace")
	}
	return nil
}

func NoJoined(clientset *kubernetes.Clientset) error {
	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%v=%v", liqoconst.TypeLabel, liqoconst.TypeNode),
	})
	if err != nil {
		klog.Error(err)
		return err
	}

	if len(nodes.Items) > 0 {
		return fmt.Errorf("there are still virtual nodes in the cluster")
	}
	return nil

}
