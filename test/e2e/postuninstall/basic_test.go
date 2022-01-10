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

package postuninstall

import (
	"context"
	"fmt"
	"testing"
	"time"

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
		testContext = tester.GetTesterUninstall(ctx)
		interval    = 3 * time.Second
		timeout     = 5 * time.Minute
	)

	Describe("Assert that Liqo is correctly uninstalled", func() {
		Context("Test Unjoin", func() {
			var uninstalledTableEntries []TableEntry
			for index := range testContext.Clusters {
				uninstalledTableEntries = append(uninstalledTableEntries,
					Entry(fmt.Sprintf("Check Liqo is correctly uninstalled on cluster %v", index+1),
						testContext.Clusters[index], testContext.Namespace))
			}

			DescribeTable("Liqo Uninstall Check",
				func(homeCluster tester.ClusterContext, namespace string) {
					Eventually(func() error {
						return NoPods(homeCluster.NativeClient, testContext.Namespace)
					}, timeout, interval).ShouldNot(HaveOccurred())
					Eventually(func() error {
						return NoJoined(homeCluster.NativeClient)
					}, timeout, interval).ShouldNot(HaveOccurred())
				},
				uninstalledTableEntries...)

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
