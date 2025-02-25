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

//nolint:gosec // Need to run liqoctl command
package network

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/gateway"
	"github.com/liqotech/liqo/pkg/gateway/concurrent"
	networkflags "github.com/liqotech/liqo/pkg/liqoctl/test/network/flags"
	"github.com/liqotech/liqo/pkg/liqoctl/test/network/setup"
	"github.com/liqotech/liqo/test/e2e/testconsts"
	"github.com/liqotech/liqo/test/e2e/testutils/config"
	"github.com/liqotech/liqo/test/e2e/testutils/tester"
	"github.com/liqotech/liqo/test/e2e/testutils/util"
)

const (
	// clustersRequired is the number of clusters required in this E2E test.
	clustersRequired = 3
	// testName is the name of this E2E test.
	testName = "NETWORK"
	// StressMax is the maximum number of stress iterations.
	stressMax = 3
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
	timeout       = time.Minute * 5
	namespaceName = setup.NamespaceName

	providers []string
	consumer  string

	// Default network tests defaultArgs.
	defaultArgs = networkTestsArgs{
		nodePortNodes: networkflags.NodePortNodesAll,
		nodePortExt:   true,
		podNodePort:   true,
		ip:            true,
		loadBalancer:  true,
		info:          true,
		remove:        true,
		failfast:      true,
		basic:         false,
	}
)

var _ = BeforeSuite(func() {
	for i := range testContext.Clusters {
		if testContext.Clusters[i].Role == liqov1beta1.ProviderRole {
			providers = append(providers, testContext.Clusters[i].KubeconfigPath)
		}
	}
	for i := range testContext.Clusters {
		if testContext.Clusters[i].Role == liqov1beta1.ConsumerRole {
			consumer = testContext.Clusters[i].KubeconfigPath
			break
		}
	}

	switch testContext.Cni {
	case "flannel":
		overrideArgsFlannel(&defaultArgs)
	}

	switch testContext.Infrastructure {
	case "kubeadm":
		overrideArgsKubeadm(&defaultArgs)
	case "k3s":
		overrideArgsK3s(&defaultArgs)
	case "kind":
		overrideArgsKind(&defaultArgs)
	case "eks":
		overrideArgsEKS(&defaultArgs)
	case "gke":
		overrideArgsGKE(&defaultArgs)
	case "aks":
		overrideArgsAKS(&defaultArgs)
	}
})

var _ = Describe("Liqo E2E", func() {

	Context("Network", func() {
		When("\"liqoctl test network\" runs", func() {
			It("should succeed both before and after gateway pods restart", func() {
				// Run the tests.
				Eventually(func() error {
					return runLiqoctlNetworkTests(defaultArgs)
				}, timeout, interval).Should(Succeed())

				// Restart the gateway pods.
				for i := range testContext.Clusters {
					RestartPods(testContext.Clusters[i].ControllerClient)
				}

				// Check if there is only one active gateway pod per remote cluster.
				for i := range testContext.Clusters {
					numActiveGateway := testContext.Clusters[i].NumPeeredConsumers + testContext.Clusters[i].NumPeeredProviders
					Eventually(func() error {
						return checkUniqueActiveGatewayPod(testContext.Clusters[i].ControllerClient, numActiveGateway)
					}, timeout, interval).Should(Succeed())
				}

				// Run the tests again.
				Eventually(func() error {
					return runLiqoctlNetworkTests(defaultArgs)
				}, timeout, interval).Should(Succeed())
			})

			It("should succeed both before and after gateway pods restart (stress gateway deletion and run basic tests)", func() {
				args := defaultArgs
				args.basic = true
				args.remove = false
				for i := 0; i < stressMax; i++ {
					// Restart the gateway pods.
					for j := range testContext.Clusters {
						RestartPods(testContext.Clusters[j].ControllerClient)
					}

					// Check if there is only one active gateway pod per remote cluster.
					for j := range testContext.Clusters {
						numActiveGateway := testContext.Clusters[j].NumPeeredConsumers + testContext.Clusters[j].NumPeeredProviders
						Eventually(func() error {
							return checkUniqueActiveGatewayPod(testContext.Clusters[j].ControllerClient, numActiveGateway)
						}, timeout, interval).Should(Succeed())
					}

					if i == stressMax-1 {
						args.remove = true
					}

					// Run the tests.
					Eventually(func() error {
						return runLiqoctlNetworkTests(args)
					}, timeout, interval).Should(Succeed())
				}
			})
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

type networkTestsArgs struct {
	nodePortNodes networkflags.NodePortNodes
	nodePortExt   bool
	podNodePort   bool
	ip            bool
	loadBalancer  bool
	info          bool
	remove        bool
	failfast      bool
	basic         bool
}

func runLiqoctlNetworkTests(args networkTestsArgs) error {
	cmd := exec.CommandContext(ctx, testContext.LiqoctlPath, forgeFlags(args)...)

	fmt.Fprintf(GinkgoWriter, "Running command: %s\n", strings.Join(cmd.Args, " "))

	stdout, err := cmd.StdoutPipe()
	Expect(err).ToNot(HaveOccurred())
	stderr, err := cmd.StderrPipe()
	Expect(err).ToNot(HaveOccurred())

	Expect(cmd.Start()).To(Succeed())

	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		Expect(util.Second(fmt.Fprintln(GinkgoWriter, scanner.Text()))).To(Succeed())
	}
	scanner = bufio.NewScanner(stdout)
	for scanner.Scan() {
		Expect(util.Second(fmt.Fprintln(GinkgoWriter, scanner.Text()))).To(Succeed())
	}

	return cmd.Wait()
}

func forgeFlags(args networkTestsArgs) []string {
	flags := []string{
		"test", "network",
		"--kubeconfig", consumer,
		"--remote-kubeconfigs", strings.Join(providers, ","),
	}
	if args.nodePortNodes != "" {
		flags = append(flags, "--np-nodes", args.nodePortNodes.String())
	}
	if args.nodePortExt {
		flags = append(flags, "--np-ext")
	}
	if args.podNodePort {
		flags = append(flags, "--pod-np")
	}
	if args.ip {
		flags = append(flags, "--ip")
	}
	if args.loadBalancer {
		flags = append(flags, "--lb")
	}
	if args.info {
		flags = append(flags, "--info")
	}
	if args.remove {
		flags = append(flags, "--rm")
	}
	if args.failfast {
		flags = append(flags, "--fail-fast")
	}
	if args.basic {
		flags = append(flags, "--basic")
	}

	return flags
}

func overrideArgsFlannel(args *networkTestsArgs) {
	args.nodePortNodes = networkflags.NodePortNodesWorkers
}

func overrideArgsKubeadm(args *networkTestsArgs) {
	args.loadBalancer = false
}

func overrideArgsK3s(args *networkTestsArgs) {
	args.loadBalancer = false
}

func overrideArgsKind(args *networkTestsArgs) {
	args.loadBalancer = false
}

func overrideArgsEKS(args *networkTestsArgs) {
	args.failfast = false
	args.nodePortExt = false // nodeport are not exposed
}

func overrideArgsGKE(args *networkTestsArgs) {
	cni, ok := os.LookupEnv("CNI")
	if !ok {
		panic(fmt.Errorf("CNI environment variable not set"))
	}

	if cni != "v1" && cni != "v2" {
		panic(fmt.Errorf("CNI environment %q variable not valid", cni))
	}

	args.failfast = false
}

func overrideArgsAKS(args *networkTestsArgs) {
	args.failfast = false
	args.nodePortExt = false // nodeport are not exposed
}

func RestartPods(cl client.Client) {
	podList := &corev1.PodList{}
	Expect(
		cl.List(ctx, podList, &client.ListOptions{
			LabelSelector: labels.SelectorFromSet(labels.Set{
				gateway.GatewayComponentKey: gateway.GatewayComponentGateway,
				concurrent.ActiveGatewayKey: concurrent.ActiveGatewayValue,
			}),
		}),
	).To(Succeed())

	for i := range podList.Items {
		pod := &podList.Items[i]
		Expect(cl.Delete(ctx, pod)).To(Succeed())
	}

	// Sleep few seconds to be sure that the deployment controller has updated the number of ready replicas.
	time.Sleep(2 * time.Second)

	Eventually(func() error {
		deploymentList := &appsv1.DeploymentList{}
		if err := cl.List(ctx, deploymentList, &client.ListOptions{
			LabelSelector: labels.SelectorFromSet(labels.Set{
				gateway.GatewayComponentKey: gateway.GatewayComponentGateway,
			}),
		}); err != nil {
			return err
		}

		for i := range deploymentList.Items {
			deployment := &deploymentList.Items[i]
			if deployment.Status.ReadyReplicas != *deployment.Spec.Replicas {
				return fmt.Errorf("deployment %s is not ready", deployment.Name)
			}
		}
		return nil
	}, timeout, interval).Should(Succeed())
}

// checkUniqueActiveGatewayPod checks if there is only one active gateway pod.
func checkUniqueActiveGatewayPod(cl client.Client, numActiveGateway int) error {
	// Sleep few seconds to be sure that the new leader is elected.
	time.Sleep(2 * time.Second)

	podList := &corev1.PodList{}
	if err := cl.List(ctx, podList, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set{
			gateway.GatewayComponentKey: gateway.GatewayComponentGateway,
			concurrent.ActiveGatewayKey: concurrent.ActiveGatewayValue,
		}),
	}); err != nil {
		return fmt.Errorf("unable to list active gateway pods: %w", err)
	}

	if len(podList.Items) != numActiveGateway {
		return fmt.Errorf("expected %d active gateway pod, got %d", numActiveGateway, len(podList.Items))
	}

	return nil
}
