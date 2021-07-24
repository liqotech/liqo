package peering2e2e

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

const (
	randomSchedulingResourcePath = "https://raw.githubusercontent.com/liqotech/microservices-demo/master/release/kubernetes-manifests.yaml"
	fixedSchedulingResourcePath  = "https://raw.githubusercontent.com/liqotech/microservices-demo/master/release/fixed-2clusters.yaml"
)

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
			)

			DescribeTable("Liqo Virtual Nodes are ready",
				func(homeCluster tester.ClusterContext, namespace string) {
					nodeReady := util.CheckVirtualNodes(ctx, homeCluster.Client)
					Expect(nodeReady).To(BeTrue())
				},
				Entry("VirtualNode is Ready on cluster 2", testContext.Clusters[0], namespace),
				Entry("VirtualNode is Ready on cluster 1", testContext.Clusters[1], namespace),
			)
		})

		Context("E2E network testing with pods and services", func() {
			DescribeTable("Liqo Pod to Pod Connectivity Check",
				func(homeCluster, foreignCluster tester.ClusterContext, namespace string) {
					By("Deploy Tester Pod", func() {
						err := net.EnsureNetTesterPods(ctx, homeCluster.Client, homeCluster.ClusterID)
						Expect(err).ToNot(HaveOccurred())
						Eventually(func() bool {
							check := net.CheckTesterPods(ctx, homeCluster.Client, foreignCluster.Client, homeCluster.ClusterID)
							return check
						}, timeout, interval).Should(BeTrue())
					})

					By("Check Pod to Pod Connectivity", func() {
						Eventually(func() error {
							return net.CheckPodConnectivity(ctx, homeCluster.Config, homeCluster.Client)
						}, timeout, interval).ShouldNot(HaveOccurred())
					})

					By("Check Service NodePort Connectivity", func() {
						Eventually(func() error {
							return net.ConnectivityCheckNodeToPod(ctx, homeCluster.Client, homeCluster.ClusterID)
						}, timeout, interval).ShouldNot(HaveOccurred())
					})
				},
				Entry("Check Pod to Pod connectivity from cluster 1", testContext.Clusters[0], testContext.Clusters[1], namespace),
				Entry("Check Pod to Pod connectivity from cluster 2", testContext.Clusters[1], testContext.Clusters[0], namespace),
			)
		})

		DescribeTable("Online Boutique application deployment check",
			func(kubeResourcePath string) {
				By("Deploying the Online Boutique app")
				options := k8s.NewKubectlOptions("", testContext.Clusters[0].KubeconfigPath, microservices.TestNamespaceName)
				defer GinkgoRecover()
				err := microservices.DeployApp(GinkgoT(), testContext.Clusters[0].KubeconfigPath, kubeResourcePath)
				Expect(err).ShouldNot(HaveOccurred())

				By("Waiting until each service of the application has ready endpoints")
				microservices.WaitDemoApp(GinkgoT(), options)

				By("Checking if all pods deployed in the test namespace have the right NodeAffinity")
				Eventually(func() bool {
					return microservices.CheckPodsNodeAffinity(ctx, testContext.Clusters[0].Client)
				}, timeout, interval).Should(BeTrue())

				By("Verify Online Boutique Connectivity")
				err = microservices.CheckApplicationIsWorking(GinkgoT(), options)
				Expect(err).ShouldNot(HaveOccurred())
			},
			Entry("Check application deployment with random scheduling", randomSchedulingResourcePath),
			Entry("Check application deployment with fixed scheduling", fixedSchedulingResourcePath),
		)

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
