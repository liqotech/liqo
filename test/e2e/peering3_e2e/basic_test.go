package peering3e2e

import (
	"context"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/k8s"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/liqotech/liqo/test/e2e/testutils"
	"github.com/liqotech/liqo/test/e2e/testutils/microservices"
	"github.com/liqotech/liqo/test/e2e/testutils/net"
	"github.com/liqotech/liqo/test/e2e/testutils/tester"
	"github.com/liqotech/liqo/test/e2e/testutils/util"
)

const kubeResourcePath = "https://raw.githubusercontent.com/liqotech/microservices-demo/master/release/fixed-3clusters.yaml"

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Liqo E2E Suite")
}

var _ = Describe("Liqo E2E", func() {
	var (
		ctx         = context.Background()
		testContext = tester.GetTester(ctx)
		namespace   = "liqo"
		interval    = 3 * time.Second
		timeout     = 5 * time.Minute
	)

	Describe("Assert that Liqo is up, pod offloading and network connectivity are working", func() {
		Context("Check Join Status", func() {
			DescribeTable("Liqo pods are up and running",
				func(cluster tester.ClusterContext, namespace string) {
					readyPods, notReadyPods, err := util.ArePodsUp(ctx, cluster.Client, testContext.Namespace)
					Eventually(func() bool {
						return err == nil
					}, timeout, interval).Should(BeTrue())
					Expect(len(notReadyPods)).To(Equal(0))
					Expect(len(readyPods)).Should(BeNumerically(">", 0))
				},
				Entry("Pods UP on cluster 1", testContext.Clusters[0], namespace),
				Entry("Pods UP on cluster 2", testContext.Clusters[1], namespace),
				Entry("Pods UP on cluster 3", testContext.Clusters[2], namespace),
			)

			DescribeTable("Liqo Virtual Nodes are ready",
				func(homeCluster tester.ClusterContext) {
					nodeReady := util.CheckVirtualNodes(ctx, homeCluster.Client)
					Expect(nodeReady).To(BeTrue())
				},
				Entry("VirtualNodes are Ready on cluster 1", testContext.Clusters[0]),
				Entry("VirtualNodes are Ready on cluster 2", testContext.Clusters[1]),
				Entry("VirtualNodes are Ready on cluster 3", testContext.Clusters[2]),
			)

			DescribeTable("Liqo Pod to Pod Connectivity Check",
				func(homeCluster, foreignCluster tester.ClusterContext, namespace string) {
					By("Deploy Tester Pod", func() {
						err := net.EnsureNetTesterPods(ctx, homeCluster.Client, homeCluster.ClusterID, namespace)
						Expect(err).ToNot(HaveOccurred())
						Eventually(func() bool {
							check := net.CheckTesterPods(ctx, homeCluster.Client, foreignCluster.Client, homeCluster.ClusterID, namespace)
							return check
						}, timeout, interval).Should(BeTrue())
					})

					By("Check Pod to Pod Connectivity", func() {
						Eventually(func() error {
							return net.CheckPodConnectivity(ctx, homeCluster.Config, homeCluster.Client, namespace)
						}, timeout, interval).ShouldNot(HaveOccurred())
					})

					By("Check Service NodePort Connectivity", func() {
						err := net.ConnectivityCheckNodeToPod(ctx, homeCluster.Client, homeCluster.ClusterID, namespace)
						Expect(err).ToNot(HaveOccurred())
					})
				},
				Entry("Check Pod to Pod connectivity from cluster 1 to cluster 2", testContext.Clusters[0], testContext.Clusters[1], "test1to2"),
				Entry("Check Pod to Pod connectivity from cluster 2 to cluster 1", testContext.Clusters[1], testContext.Clusters[0], "test2to1"),
				Entry("Check Pod to Pod connectivity from cluster 2 to cluster 3", testContext.Clusters[1], testContext.Clusters[2], "test2to3"),
				Entry("Check Pod to Pod connectivity from cluster 3 to cluster 2", testContext.Clusters[2], testContext.Clusters[1], "test3to2"),
			)
		})

		Context("E2E Testing with Online Boutique", func() {
			It("Testing online boutique", func() {
				By("Deploying the Online Boutique app")
				options := k8s.NewKubectlOptions("", testContext.Clusters[1].KubeconfigPath, microservices.TestNamespaceName)
				defer GinkgoRecover()
				err := microservices.DeployApp(GinkgoT(), testContext.Clusters[1].KubeconfigPath, kubeResourcePath)
				Expect(err).ShouldNot(HaveOccurred())

				By("Waiting until each service of the application has ready endpoints")
				microservices.WaitDemoApp(GinkgoT(), options)

				By("Checking if all pods deployed in the test namespace have the right NodeAffinity")
				// Eventually(func() bool {
				// 	return microservices.CheckPodsNodeAffinity(ctx, testContext.Clusters[1].Client)
				// }, timeout, interval).Should(BeTrue())

				By("Verify Online Boutique Connectivity")
				err = microservices.CheckApplicationIsWorking(GinkgoT(), options)
				Expect(err).ShouldNot(HaveOccurred())
			})
		})

		AfterSuite(func() {

			for i := range testContext.Clusters {
				err := util.DeleteNamespace(ctx, testContext.Clusters[i].Client, testutils.LiqoTestNamespaceLabels)
				Expect(err).ShouldNot(HaveOccurred())
			}
			Eventually(func() bool {
				for i := range testContext.Clusters {
					list, err := testContext.Clusters[i].Client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{
						LabelSelector: labels.SelectorFromSet(testutils.LiqoTestNamespaceLabels).String(),
					})
					if err != nil || len(list.Items) > 0 {
						return false
					}
				}
				return true
			}, timeout, interval).Should(BeTrue())
		})
	})

})
