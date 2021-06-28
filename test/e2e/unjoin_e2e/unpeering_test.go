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

func Test_Unjoin(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Liqo E2E Suite")
}

var _ = Describe("Liqo E2E", func() {
	var (
		ctx         = context.Background()
		testContext = tester.GetTester(ctx)
	)

	Describe("Assert that Liqo is correctly uninstalled", func() {
		Context("Test Unjoin", func() {
			err := NoPods(testContext.Clusters[0].Client, testContext.Namespace)
			Expect(err).ShouldNot(HaveOccurred())
			err = NoJoined(testContext.Clusters[0].Client)
			Expect(err).ShouldNot(HaveOccurred())
			readyPods, notReadyPods, err := util.ArePodsUp(ctx, testContext.Clusters[1].Client, testContext.Namespace)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(notReadyPods).Should(BeZero())
			Expect(len(readyPods)).Should(BeNumerically(">", 0))
		},
		)
	})
})

func NoPods(clientset *kubernetes.Clientset, namespace string) error {
	pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		klog.Error(err)
		return err
	}
	if len(pods.Items) > 0 {
		return fmt.Errorf("There are still running pods in Liqo namespace")
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
		return fmt.Errorf("There are still virtual nodes in the cluster")
	}
	return nil

}
