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

package postinstall

import (
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"k8s.io/klog/v2"

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
				PodsUpAndRunningTableEntries = append(PodsUpAndRunningTableEntries, Entry("Pods UP on cluster "+fmt.Sprintf("%d", index),
					testContext.Clusters[index], namespace))
				VirtualNodesTableEntries = append(VirtualNodesTableEntries, Entry("VirtualNodes are Ready on cluster "+fmt.Sprintf("%d", index),
					testContext.Clusters[index], namespace))
			}

			DescribeTable("Liqo pods are up and running",
				func(cluster tester.ClusterContext, namespace string) {
					Eventually(func() bool {
						readyPods, notReadyPods, err := util.ArePodsUp(ctx, cluster.NativeClient, testContext.Namespace)
						klog.Infof("Liqo pods status: %d ready, %d not ready", len(readyPods), len(notReadyPods))
						return err == nil && len(notReadyPods) == 0 && len(readyPods) > 0
					}, timeout, interval).Should(BeTrue())
				},
				PodsUpAndRunningTableEntries...,
			)

			DescribeTable("Liqo Virtual nodes are ready",
				func(homeCluster tester.ClusterContext, namespace string) {
					Eventually(func() bool {
						return util.CheckVirtualNodes(ctx, homeCluster.NativeClient)
					}, timeout, interval).Should(BeTrue())
				},
				VirtualNodesTableEntries...,
			)
		})
	})
})
