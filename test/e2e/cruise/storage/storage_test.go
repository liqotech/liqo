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

package storage

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/k8s"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	liqoconst "github.com/liqotech/liqo/pkg/consts"
	liqoctlmove "github.com/liqotech/liqo/pkg/liqoctl/move"
	"github.com/liqotech/liqo/test/e2e/testconsts"
	"github.com/liqotech/liqo/test/e2e/testutils/config"
	"github.com/liqotech/liqo/test/e2e/testutils/storage"
	"github.com/liqotech/liqo/test/e2e/testutils/tester"
	"github.com/liqotech/liqo/test/e2e/testutils/util"
)

const (
	// clustersRequired is the number of clusters required in this E2E test.
	clustersRequired = 2
	// testName is the name of this E2E test.
	testName = "STORAGE"
)

func TestE2E(t *testing.T) {
	util.CheckIfTestIsSkipped(t, clustersRequired, testName)

	if util.GetEnvironmentVariableOrDie(testconsts.InfrastructureEnvVar) == testconsts.ProviderK3s {
		t.Skipf("Skipping %s test on k3s", testName)
	}

	RegisterFailHandler(Fail)
	RunSpecs(t, "Liqo E2E Suite")
}

var (
	ctx           = context.Background()
	testContext   = tester.GetTester(ctx)
	interval      = config.Interval
	timeout       = config.Timeout
	namespaceName = util.GetNameNamespaceTest(testName)
)

var _ = Describe("Liqo E2E", func() {
	Context("Storage Testing", func() {
		var (
			replica1Name = fmt.Sprintf("%v-1", storage.StatefulSetName)
			options      *k8s.KubectlOptions

			podPhase = func(podName string) corev1.PodPhase {
				pod, err := testContext.Clusters[0].NativeClient.CoreV1().Pods(namespaceName).Get(ctx, podName, metav1.GetOptions{})
				if err != nil {
					return ""
				}
				klog.Infof("Phase of pod %s is %s", podName, pod.Status.Phase)
				return pod.Status.Phase
			}
		)

		BeforeEach(func() {
			options = k8s.NewKubectlOptions("", testContext.Clusters[0].KubeconfigPath, namespaceName)
		})

		AfterEach(func() {
			Eventually(func() error {
				return util.EnsureNamespaceDeletion(ctx, testContext.Clusters[0].NativeClient, namespaceName)
			}, timeout, interval).Should(Succeed())
		})

		It("run stateful app", func() {
			By("Deploying the StatefulSet app")
			err := storage.DeployApp(ctx, &testContext.Clusters[0], namespaceName, 2)
			Expect(err).ToNot(HaveOccurred())

			By("Waiting until each pod of the application is ready")
			storage.WaitDemoApp(GinkgoT(), options, 2)

			By("Checking that the pod is bound to a specific cluster")
			pod, err := testContext.Clusters[0].NativeClient.CoreV1().Pods(namespaceName).Get(ctx, replica1Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())

			By("Cordoning the virtual node")
			node, err := testContext.Clusters[0].NativeClient.CoreV1().Nodes().Get(ctx, pod.Spec.NodeName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			node.Spec.Unschedulable = true
			_, err = testContext.Clusters[0].NativeClient.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
			Expect(err).ToNot(HaveOccurred())

			By("Deleting the pod on the virtual node")
			Expect(testContext.Clusters[0].NativeClient.CoreV1().Pods(namespaceName).Delete(ctx, replica1Name, metav1.DeleteOptions{})).To(Succeed())
			Eventually(func() corev1.PodPhase {
				return podPhase(replica1Name)
			}, timeout, interval).Should(Equal(corev1.PodPending))
			Consistently(func() corev1.PodPhase {
				return podPhase(replica1Name)
			}, 10*time.Second, interval).Should(Equal(corev1.PodPending))

			By("Uncordoning the virtual nodes")
			node, err = testContext.Clusters[0].NativeClient.CoreV1().Nodes().Get(ctx, pod.Spec.NodeName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			node.Spec.Unschedulable = false
			_, err = testContext.Clusters[0].NativeClient.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
			Expect(err).ToNot(HaveOccurred())

			By("Checking that the pod is running again")
			storage.WaitDemoApp(GinkgoT(), options, 2)
		})

		It("move stateful app", func() {
			By("Deploying the StatefulSet app")
			err := storage.DeployApp(ctx, &testContext.Clusters[0], namespaceName, 1)
			Expect(err).ToNot(HaveOccurred())

			By("Waiting until each pod of the application is ready")
			storage.WaitDemoApp(GinkgoT(), options, 1)

			By("Write something in the volume")
			Expect(storage.WriteToVolume(ctx, testContext.Clusters[0].NativeClient, testContext.Clusters[0].Config, namespaceName)).To(Succeed())

			By("Scaling the statefulset to zero replicas")
			Expect(storage.ScaleStatefulSet(ctx, GinkgoT(), options, testContext.Clusters[0].NativeClient, namespaceName, 0)).To(Succeed())

			originPvcList, err := testContext.Clusters[0].NativeClient.CoreV1().PersistentVolumeClaims(namespaceName).List(ctx, metav1.ListOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(originPvcList.Items).To(HaveLen(1))

			originPvc := originPvcList.Items[0]

			virtualNodesList := &corev1.NodeList{}
			Expect(testContext.Clusters[0].ControllerClient.
				List(ctx, virtualNodesList, client.MatchingLabels{liqoconst.TypeLabel: liqoconst.TypeNode})).To(Succeed())
			Expect(len(virtualNodesList.Items)).To(BeNumerically(">=", 1))

			By("Moving the statefulset to the virtual node")
			args := []string{"move", "volume", originPvc.Name, "-n", namespaceName, "--target-node", virtualNodesList.Items[0].Name,
				"--containers-cpu-limits", "500m", "--containers-ram-limits", "500Mi"}
			dockerProxy, ok := os.LookupEnv("DOCKER_PROXY")
			if ok {
				args = append(args, "--restic-server-image", dockerProxy+"/"+liqoctlmove.DefaultResticServerImage)
				args = append(args, "--restic-image", dockerProxy+"/"+liqoctlmove.DefaultResticImage)
			}
			Expect(util.ExecLiqoctl(testContext.Clusters[0].KubeconfigPath, args, GinkgoWriter)).To(Succeed())

			By("Scaling the statefulset to one replica")
			Expect(storage.ScaleStatefulSet(ctx, GinkgoT(), options, testContext.Clusters[0].NativeClient, namespaceName, 1)).To(Succeed())

			By("Checking that the pod is running again, on the virtual node")
			statefulSetPod, err := testContext.Clusters[0].NativeClient.CoreV1().Pods(namespaceName).Get(ctx,
				fmt.Sprintf("%s-0", storage.StatefulSetName), metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(statefulSetPod.Spec.NodeName).To(Equal(virtualNodesList.Items[0].Name))

			By("Checking the content of the volume")
			content, err := storage.ReadFromVolume(ctx, testContext.Clusters[0].NativeClient, testContext.Clusters[0].Config, namespaceName)
			Expect(err).ToNot(HaveOccurred())
			Expect(content).To(Equal("test"))
		})
	})
})

var _ = AfterSuite(func() {
	for i := range testContext.Clusters {
		Eventually(func() error {
			return util.EnsureNamespaceDeletion(ctx, testContext.Clusters[i].NativeClient, namespaceName)
		}, timeout, interval).Should(Succeed())
	}
})
