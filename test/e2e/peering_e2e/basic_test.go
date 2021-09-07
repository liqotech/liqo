package cruise

import (
	"context"
	"fmt"
	"strings"
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

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Liqo E2E Suite")
}

var _ = Describe("Liqo E2E", func() {
	var (
		ctx         = context.Background()
		testContext = tester.GetTester(ctx, true)
		namespace   = "liqo"
		interval    = 3 * time.Second
		timeout     = 5 * time.Minute
	)

	Describe("Assert that Liqo is up, pod offloading and network connectivity are working", func() {
		Context("Check Join Status", func() {
			var PodsUpAndRunningTableEntries []TableEntry
			for index := range testContext.Clusters {
				for index2 := range testContext.Clusters {
					if index != index2 {
						PodsUpAndRunningTableEntries = append(PodsUpAndRunningTableEntries,
							Entry(strings.Join([]string{"Check Pod to Pod connectivity from cluster", fmt.Sprintf("%d",index),
								"to cluster", fmt.Sprintf("%d",index2)}, " "),
								testContext.Clusters[index], testContext.Clusters[index2], namespace))
					}
				}
			}

			DescribeTable("Liqo Pod to Pod Connectivity Check",
				func(homeCluster, foreignCluster tester.ClusterContext, namespace string) {
					By("Deploy Tester Pod", func() {
						err := net.EnsureNetTesterPods(ctx, homeCluster.NativeClient, homeCluster.ClusterID)
						Expect(err).ToNot(HaveOccurred())
						Eventually(func() bool {
							check := net.CheckTesterPods(ctx, homeCluster.NativeClient, foreignCluster.NativeClient, homeCluster.ClusterID)
							return check
						}, timeout, interval).Should(BeTrue())
						Eventually(func() error {
							return net.CheckPodConnectivity(ctx, homeCluster.Config, homeCluster.NativeClient)
						}, timeout, interval).ShouldNot(HaveOccurred())
						Eventually(func() error {
							return net.ConnectivityCheckNodeToPod(ctx, homeCluster.NativeClient, homeCluster.ClusterID)
						}, timeout, interval).ShouldNot(HaveOccurred())
					})
				},
				PodsUpAndRunningTableEntries...,
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

			for i := range testContext.Clusters {
				err := util.DeleteNamespace(ctx, testContext.Clusters[i].NativeClient, testutils.LiqoTestNamespaceLabels)
				Expect(err).ShouldNot(HaveOccurred())
			}
			Eventually(func() bool {
				for i := range testContext.Clusters {
					list, err := testContext.Clusters[i].NativeClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{
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
