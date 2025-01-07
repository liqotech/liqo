// Copyright 2019-2025 The Liqo Authors
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

package postinstall

import (
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"

	"github.com/liqotech/liqo/pkg/consts"
	gwconsts "github.com/liqotech/liqo/pkg/gateway"
	fcutils "github.com/liqotech/liqo/pkg/utils/foreigncluster"
	"github.com/liqotech/liqo/pkg/vkMachinery"
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
		testContext = tester.GetTester(ctx)
		namespace   = "liqo"
		interval    = 3 * time.Second
		timeout     = 5 * time.Minute
	)

	Describe("Assert that Liqo is up, pod offloading and network connectivity are working", func() {
		Context("Check Join Status", func() {
			var PodsUpAndRunningTableEntries, VirtualNodesTableEntries []TableEntry
			for index := range testContext.Clusters {
				PodsUpAndRunningTableEntries = append(PodsUpAndRunningTableEntries, Entry("Pods UP on cluster "+fmt.Sprintf("%d", index+1),
					testContext.Clusters[index], namespace))
				// Check if the virtual nodes are ready only on consumer clusters
				if fcutils.IsConsumer(testContext.Clusters[index].Role) {
					VirtualNodesTableEntries = append(VirtualNodesTableEntries, Entry("VirtualNodes are Ready on cluster "+fmt.Sprintf("%d", index+1),
						testContext.Clusters[index]))
				}
			}

			DescribeTable("Liqo pods are up and running", util.DescribeTableArgs(
				func(cluster tester.ClusterContext, namespace string) {
					Eventually(func() bool {
						readyPods, notReadyPods, err := util.ArePodsUp(ctx, cluster.NativeClient, testContext.Namespace)
						klog.Infof("Liqo pods status: %d ready, %d not ready", len(readyPods), len(notReadyPods))
						return err == nil && len(notReadyPods) == 0 && len(readyPods) > 0
					}, timeout, interval).Should(BeTrue())

					// Check that the pods were not restarted
					pods, err := cluster.NativeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
					Expect(err).ToNot(HaveOccurred())
					for _, pod := range pods.Items {
						Expect(pod.Status.ContainerStatuses).ToNot(BeEmpty())
						Expect(pod.Status.ContainerStatuses[0].RestartCount).To(BeNumerically("<=", 2))
					}

					var tenantNsList *corev1.NamespaceList
					Eventually(func() []corev1.Namespace {
						namespaceLabel := map[string]string{}
						namespaceLabel[consts.TenantNamespaceLabel] = "true"
						var err error
						tenantNsList, err = cluster.NativeClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{
							LabelSelector: labels.SelectorFromSet(namespaceLabel).String(),
						})
						Expect(err).ToNot(HaveOccurred())
						return tenantNsList.Items
					}, timeout, interval).Should(HaveLen(util.NumTenantNamespaces(
						cluster.NumPeeredConsumers, cluster.NumPeeredProviders, cluster.Role)))

					for _, tenantNs := range tenantNsList.Items {
						Eventually(func() bool {
							readyPods, notReadyPods, err := util.ArePodsUp(ctx, cluster.NativeClient, tenantNs.Name)
							Expect(err).ToNot(HaveOccurred())
							klog.Infof("Tenant pods status: %d ready, %d not ready", len(readyPods), len(notReadyPods))
							// Get deployment gateway
							gwDeployments, err := cluster.NativeClient.AppsV1().Deployments(tenantNs.Name).List(ctx, metav1.ListOptions{
								LabelSelector: fmt.Sprintf("%s=%s", gwconsts.GatewayComponentKey, gwconsts.GatewayComponentGateway),
							})
							Expect(err).ToNot(HaveOccurred())
							Expect(gwDeployments.Items).To(HaveLen(1))
							gwReplicas := int(ptr.Deref(gwDeployments.Items[0].Spec.Replicas, 1))

							// Get deployment virtual-kubelet if role is consumer
							vkReplicas := 0
							if fcutils.IsConsumer(cluster.Role) {
								vkDeployments, err := cluster.NativeClient.AppsV1().Deployments(tenantNs.Name).List(ctx, metav1.ListOptions{
									LabelSelector: labels.SelectorFromSet(vkMachinery.KubeletBaseLabels).String(),
								})
								Expect(err).ToNot(HaveOccurred())
								Expect(vkDeployments.Items).To(HaveLen(1))
								vkReplicas = int(ptr.Deref(vkDeployments.Items[0].Spec.Replicas, 1))
							}
							return len(notReadyPods) == 0 &&
								len(readyPods) == util.NumPodsInTenantNs(true, cluster.Role, gwReplicas, vkReplicas)
						}, timeout, interval).Should(BeTrue())
					}
				},
				PodsUpAndRunningTableEntries...,
			)...)

			DescribeTable("Liqo Virtual nodes are ready", util.DescribeTableArgs(
				func(homeCluster tester.ClusterContext) {
					Eventually(func() bool {
						return util.CheckVirtualNodes(ctx, homeCluster.NativeClient, testContext.ClustersNumber)
					}, timeout, interval).Should(BeTrue())
				},
				VirtualNodesTableEntries...,
			)...)
		})
	})
})
