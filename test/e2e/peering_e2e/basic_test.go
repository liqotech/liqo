package peeringe2e

import (
	"context"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/k8s"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"github.com/liqotech/liqo/test/e2e/testutils/microservices"
	"github.com/liqotech/liqo/test/e2e/testutils/net"
	"github.com/liqotech/liqo/test/e2e/testutils/tester"
	"github.com/liqotech/liqo/test/e2e/testutils/util"
)

const (
	// clustersRequired is the number of clusters required in this E2E test.
	clustersRequired = 2
	// controllerClientPresence indicates if the test use the controller runtime clients.
	controllerClientPresence = true
	// testName is the name of this E2E test.
	testName = "E2E_PEERING"
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
		namespace   = "liqo"
		interval    = 3 * time.Second
		timeout     = 5 * time.Minute
	)

	Describe("Assert that Liqo is up, pod offloading and network connectivity are working", func() {
		Context("Check Join Status", func() {
			DescribeTable("Liqo pods are up and running",
				func(cluster tester.ClusterContext, namespace string) {
					readyPods, notReadyPods, err := util.ArePodsUp(ctx, cluster.NativeClient, testContext.Namespace)
					Eventually(func() bool {
						return err == nil
					}, timeout, interval).Should(BeTrue())
					Expect(len(notReadyPods)).To(Equal(0))
					Expect(len(readyPods)).Should(BeNumerically(">", 0))
				},
				Entry("Pods UP on cluster 1", testContext.Clusters[0], namespace),
				Entry("Pods UP on cluster 2", testContext.Clusters[1], namespace),
			)

			DescribeTable("Liqo Virtual Nodes are ready",
				func(homeCluster tester.ClusterContext, namespace string) {
					nodeReady := util.CheckVirtualNodes(ctx, homeCluster.NativeClient)
					Expect(nodeReady).To(BeTrue())
				},
				Entry("VirtualNode is Ready on cluster 2", testContext.Clusters[0], namespace),
				Entry("VirtualNode is Ready on cluster 1", testContext.Clusters[1], namespace),
			)

			DescribeTable("Liqo Pod to Pod Connectivity Check",
				func(homeCluster, foreignCluster tester.ClusterContext, namespace string) {
					By("Deploy Tester Pod", func() {
						err := net.EnsureNetTesterPods(ctx, homeCluster.NativeClient, homeCluster.ClusterID)
						Expect(err).ToNot(HaveOccurred())
						Eventually(func() bool {
							check := net.CheckTesterPods(ctx, homeCluster.NativeClient, foreignCluster.NativeClient, homeCluster.ClusterID)
							return check
						}, timeout, interval).Should(BeTrue())
					})

					By("Check Pod to Pod Connectivity", func() {
						Eventually(func() error {
							return net.CheckPodConnectivity(ctx, homeCluster.Config, homeCluster.NativeClient)
						}, timeout, interval).ShouldNot(HaveOccurred())
					})

					By("Check Service NodePort Connectivity", func() {
						err := net.ConnectivityCheckNodeToPod(ctx, homeCluster.NativeClient, homeCluster.ClusterID)
						Expect(err).ToNot(HaveOccurred())
					})
				},
				Entry("Check Pod to Pod connectivity from cluster 1", testContext.Clusters[0], testContext.Clusters[1], namespace),
				Entry("Check Pod to Pod connectivity from cluster 2", testContext.Clusters[1], testContext.Clusters[0], namespace),
			)
		})

		Context("E2E Testing with Online Boutique", func() {
			It("Testing online boutique", func() {
				By("Deploying the Online Boutique app")
				options := k8s.NewKubectlOptions("", testContext.Clusters[0].KubeconfigPath, microservices.TestNamespaceName)
				defer GinkgoRecover()
				err := microservices.DeployApp(GinkgoT(), testContext.Clusters[0].KubeconfigPath)
				Expect(err).ShouldNot(HaveOccurred())

				By("Waiting until each service of the application has ready endpoints")
				microservices.WaitDemoApp(GinkgoT(), options)

				By("Checking if all pods deployed in the test namespace have the right NodeAffinity")
				Eventually(func() bool {
					return microservices.CheckPodsNodeAffinity(ctx, testContext.Clusters[0].NativeClient)
				}, timeout, interval).Should(BeTrue())

				By("Verify Online Boutique Connectivity")
				err = microservices.CheckApplicationIsWorking(GinkgoT(), options)
				Expect(err).ShouldNot(HaveOccurred())
			})
		})

		AfterSuite(func() {
			Eventually(func() error {
				globalErr := error(nil)
				for i := range testContext.Clusters {
					if err := util.EnsureNamespaceDeletion(ctx, testContext.Clusters[i].NativeClient, util.GetNamespaceLabel(true)); err != nil {
						globalErr = err
					}
				}
				return globalErr
			}, timeout, interval).Should(BeNil())
		})
	})

})
