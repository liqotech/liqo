// Copyright 2019-2021 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cruise

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/k8s"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/liqotech/liqo/test/e2e/testconsts"
	"github.com/liqotech/liqo/test/e2e/testutils/microservices"
	"github.com/liqotech/liqo/test/e2e/testutils/net"
	"github.com/liqotech/liqo/test/e2e/testutils/tester"
	"github.com/liqotech/liqo/test/e2e/testutils/util"
)

const (
	// clustersRequired is the number of clusters required in this E2E test.
	clustersRequired = 2
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
		testContext = tester.GetTester(ctx)
		namespace   = "liqo"
		interval    = 3 * time.Second
		timeout     = 5 * time.Minute
	)

	Describe("Assert that Liqo is up, pod offloading and network connectivity are working", func() {
		Context("Check Join Status", func() {

			type connectivityTestcase struct {
				cluster1Context tester.ClusterContext
				cluster2Context tester.ClusterContext
				namespace       string
			}

			var ConnectivityCheckTableEntries []TableEntry
			for index1 := range testContext.Clusters {
				for index2 := range testContext.Clusters {
					if index2 != index1 {
						ConnectivityCheckTableEntries = append(ConnectivityCheckTableEntries,
							Entry(fmt.Sprintf("Check Pod to Pod connectivity from cluster %v to cluster %v", index1+1, index2+1),
								connectivityTestcase{
									cluster1Context: testContext.Clusters[index1],
									cluster2Context: testContext.Clusters[index2],
									namespace:       namespace,
								}))
					}
				}
			}

			DescribeTable("Liqo Pod to Pod Connectivity Check",
				func(c connectivityTestcase) {
					By("Deploy Tester Pod", func() {
						if testContext.OverlappingCIDRs && !c.cluster1Context.HomeCluster {
							Skip("Cannot use the local pod IP on a remote cluster when the pod CIDRs are overlapping")
							return
						}

						cluster1PodName, cluster2PodName := net.GetTesterName(c.cluster1Context.ClusterID, c.cluster2Context.ClusterID)

						cluster1Opt := &net.TesterOpts{
							ClusterID: c.cluster1Context.ClusterID,
							PodName:   cluster1PodName,
							Offloaded: !c.cluster1Context.HomeCluster,
						}
						cluster2Opt := &net.TesterOpts{
							ClusterID: c.cluster2Context.ClusterID,
							PodName:   cluster2PodName,
							Offloaded: !c.cluster2Context.HomeCluster,
						}

						err := net.EnsureNetTesterPods(ctx, testContext.Clusters[0].NativeClient, cluster1Opt, cluster2Opt)
						Expect(err).ToNot(HaveOccurred())

						Eventually(func() bool {
							check := net.CheckTesterPods(ctx, testContext.Clusters[0].NativeClient, c.cluster1Context.NativeClient,
								c.cluster2Context.NativeClient, testContext.Clusters[0].ClusterID, cluster1Opt, cluster2Opt)
							return check
						}, timeout, interval).Should(BeTrue())

						Eventually(func() error {
							return net.CheckPodConnectivity(ctx,
								testContext.Clusters[0].Config, testContext.Clusters[0].NativeClient, cluster1PodName, cluster2PodName)
						}, timeout, interval).Should(Succeed())

						Eventually(func() error {
							return net.ConnectivityCheckNodeToPod(ctx,
								testContext.Clusters[0].NativeClient, testContext.Clusters[0].ClusterID, cluster2PodName)
						}, timeout, interval).Should(Succeed())
					})
				},
				ConnectivityCheckTableEntries...,
			)

			DescribeTable("Liqo Pod to Service Connectivity Check",
				func(c connectivityTestcase) {
					By("Deploy Tester Services", func() {
						cluster1PodName, cluster2PodName := net.GetTesterName(c.cluster1Context.ClusterID, c.cluster2Context.ClusterID)

						cluster1Opt := &net.TesterOpts{
							ClusterID: c.cluster1Context.ClusterID,
							PodName:   cluster1PodName,
							Offloaded: !c.cluster1Context.HomeCluster,
						}
						cluster2Opt := &net.TesterOpts{
							ClusterID: c.cluster2Context.ClusterID,
							PodName:   cluster2PodName,
							Offloaded: !c.cluster2Context.HomeCluster,
						}

						err := net.EnsureNetTesterPods(ctx, testContext.Clusters[0].NativeClient, cluster1Opt, cluster2Opt)
						Expect(err).ToNot(HaveOccurred())

						Expect(net.EnsureClusterIP(ctx,
							testContext.Clusters[0].NativeClient,
							cluster2PodName, net.TestNamespaceName)).To(Succeed())

						Eventually(func() bool {
							check := net.CheckTesterPods(ctx, testContext.Clusters[0].NativeClient, c.cluster1Context.NativeClient,
								c.cluster2Context.NativeClient, testContext.Clusters[0].ClusterID, cluster1Opt, cluster2Opt)
							return check
						}, timeout, interval).Should(BeTrue())

						Eventually(func() error {
							return net.CheckServiceConnectivity(ctx,
								testContext.Clusters[0].Config, testContext.Clusters[0].NativeClient, cluster1PodName, cluster2PodName)
						}, timeout, interval).Should(Succeed())

					})
				},
				ConnectivityCheckTableEntries...,
			)
		})

		Context("E2E Testing with Online Boutique", func() {

			const (
				manifestBasePath = "https://raw.githubusercontent.com/liqotech/microservices-demo/master/release/"
			)

			type manifest struct {
				fileName    string
				minNCluster int
			}

			var (
				manifests = []manifest{
					{
						fileName:    "kubernetes-manifests.yaml",
						minNCluster: 2,
					},
					{
						fileName:    "fixed-2clusters.yaml",
						minNCluster: 2,
					},
					{
						fileName:    "fixed-3clusters.yaml",
						minNCluster: 3,
					},
				}

				generateTableEntries = func() []TableEntry {
					var entries []TableEntry
					for i := range manifests {
						if testContext.ClustersNumber < manifests[i].minNCluster {
							continue
						}
						entries = append(entries, Entry(
							fmt.Sprintf("Deploying the Online Boutique app with manifest %v", manifests[i].fileName),
							fmt.Sprintf("%v%v", manifestBasePath, manifests[i].fileName),
							fmt.Sprintf("%v-%v", microservices.TestNamespaceName, i),
						))
					}
					return entries
				}
			)

			DescribeTable("Testing online boutique", func(manifest, namespace string) {
				By("Deploying the Online Boutique app")
				options := k8s.NewKubectlOptions("", testContext.Clusters[0].KubeconfigPath, namespace)
				defer GinkgoRecover()
				err := microservices.DeployApp(GinkgoT(), testContext.Clusters[0].KubeconfigPath, manifest, namespace)
				Expect(err).ShouldNot(HaveOccurred())

				By("Waiting until each service of the application has ready endpoints")
				microservices.WaitDemoApp(GinkgoT(), options)

				By("Checking if all pods deployed in the test namespace have the right NodeAffinity")
				Eventually(func() bool {
					return microservices.CheckPodsNodeAffinity(ctx, testContext.Clusters[0].NativeClient, namespace)
				}, timeout, interval).Should(BeTrue())

				By("Verify Online Boutique Connectivity")
				err = microservices.CheckApplicationIsWorking(GinkgoT(), options)
				Expect(err).ShouldNot(HaveOccurred())

				// cleanup the namespace
				Expect(testContext.Clusters[0].NativeClient.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{})).To(Succeed())
			}, generateTableEntries()...)
		})

		AfterSuite(func() {
			for i := range testContext.Clusters {
				Eventually(func() error {
					return util.EnsureNamespaceDeletion(ctx, testContext.Clusters[i].NativeClient, testconsts.LiqoTestNamespaceLabels)
				}, timeout, interval).Should(Succeed())
			}
		})
	})

})
