package unjoin_e2e

import (
	"context"
	"fmt"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	liqoconst "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/test/e2e/testutils/tester"
)

func Test_Unjoin(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Liqo E2E Suite")
}

var _ = Describe("Liqo E2E", func() {
	var (
		ctx         = context.Background()
		testContext = tester.GetTester(ctx, true)
	)

	Describe("Assert that Liqo is correctly uninstalled", func() {
		Context("Test Unjoin", func() {
			var PodsUpAndRunningTableEntries []TableEntry
			for index := range testContext.Clusters {
				PodsUpAndRunningTableEntries = append(PodsUpAndRunningTableEntries,
							Entry(strings.Join([]string{"Check Liqo is correctly uninstalled on cluster", fmt.Sprintf("%d", index)}, " "),
								testContext.Clusters[index], testContext.Namespace, ))
			}

			DescribeTable("Liqo Pod to Pod Connectivity Check",
				func(homeCluster tester.ClusterContext, namespace string) {
					err := NoPods(homeCluster.NativeClient, testContext.Namespace)
					Expect(err).ShouldNot(HaveOccurred())
					err = NoJoined(homeCluster.NativeClient)
					Expect(err).ShouldNot(HaveOccurred())
				},
			PodsUpAndRunningTableEntries...)

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
