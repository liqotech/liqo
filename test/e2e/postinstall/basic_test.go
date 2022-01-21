// Copyright 2019-2022 The Liqo Authors
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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/discovery"
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
				VirtualNodesTableEntries = append(VirtualNodesTableEntries, Entry("VirtualNodes are Ready on cluster "+fmt.Sprintf("%d", index+1),
					testContext.Clusters[index]))
			}

			DescribeTable("Liqo pods are up and running",
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
						Expect(pod.Status.ContainerStatuses[0].RestartCount).To(BeNumerically("==", 0))
					}

					var tenantNsList *corev1.NamespaceList
					Eventually(func() []corev1.Namespace {
						namespaceLabel := map[string]string{}
						namespaceLabel[discovery.TenantNamespaceLabel] = "true"
						var err error
						tenantNsList, err = cluster.NativeClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{
							LabelSelector: labels.SelectorFromSet(namespaceLabel).String(),
						})
						Expect(err).ToNot(HaveOccurred())
						return tenantNsList.Items
					}, timeout, interval).Should(HaveLen(testContext.ClustersNumber - 1))

					for _, tenantNs := range tenantNsList.Items {
						Eventually(func() bool {
							readyPods, notReadyPods, err := util.ArePodsUp(ctx, cluster.NativeClient, tenantNs.Name)
							klog.Infof("Tenant pods status: %d ready, %d not ready", len(readyPods), len(notReadyPods))
							return err == nil && len(notReadyPods) == 0 && len(readyPods) == 1
						}, timeout, interval).Should(BeTrue())
					}
				},
				PodsUpAndRunningTableEntries...,
			)

			DescribeTable("Liqo Virtual nodes are ready",
				func(homeCluster tester.ClusterContext) {
					Eventually(func() bool {
						return util.CheckVirtualNodes(ctx, homeCluster.NativeClient, testContext.ClustersNumber)
					}, timeout, interval).Should(BeTrue())
				},
				VirtualNodesTableEntries...,
			)
		})
	})
})
