// Copyright 2019-2024 The Liqo Authors
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

	liqov1 "github.com/liqotech/liqo/apis/core/v1alpha1"
	"github.com/liqotech/liqo/pkg/gateway"
	networkflags "github.com/liqotech/liqo/pkg/liqoctl/test/network/flags"
	"github.com/liqotech/liqo/test/e2e/testutils/config"
	"github.com/liqotech/liqo/test/e2e/testutils/tester"
	"github.com/liqotech/liqo/test/e2e/testutils/util"
)

const (
	// clustersRequired is the number of clusters required in this E2E test.
	clustersRequired = 3
	// testName is the name of this E2E test.
	testName = "NETWORK"
)

func TestE2E(t *testing.T) {
	util.CheckIfTestIsSkipped(t, clustersRequired, testName)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Liqo E2E Suite")
}

var (
	ctx         = context.Background()
	testContext = tester.GetTester(ctx)
	interval    = config.Interval
	timeout     = time.Minute * 5

	providers []string
	consumer  string

	// Default network tests args.
	args = networkTestsArgs{
		nodePortNodes: networkflags.NodePortNodesAll,
		nodePortExt:   true,
		podNodePort:   true,
		ip:            true,
		loadBalancer:  true,
		info:          true,
		remove:        true,
		failfast:      true,
	}
)

var _ = BeforeSuite(func() {
	for i := range testContext.Clusters {
		if testContext.Clusters[i].Role == liqov1.ProviderRole {
			providers = append(providers, testContext.Clusters[i].KubeconfigPath)
		}
	}
	for i := range testContext.Clusters {
		if testContext.Clusters[i].Role == liqov1.ConsumerRole {
			consumer = testContext.Clusters[i].KubeconfigPath
			break
		}
	}

	switch testContext.Cni {
	case "flannel":
		overrideArgsFlannel(&args)
	}

	switch testContext.Infrastructure {
	case "cluster-api":
		ovverideArgsClusterAPI(&args)
	case "kind":
		overrideArgsKind(&args)
	}
})

var _ = Describe("Liqo E2E", func() {

	Context("Network", func() {
		When("\"liqoctl test network\" runs", func() {
			It("should succeed both before and after gateway pods restart", func() {
				// Run the tests.
				Eventually(runLiqoctlNetworkTests(args), timeout, interval).Should(Succeed())

				// Restart the gateway pods.
				for i := range testContext.Clusters {
					RestartPods(testContext.Clusters[i].ControllerClient)
				}

				// Run the tests again.
				Eventually(runLiqoctlNetworkTests(args), timeout, interval).Should(Succeed())
			})
		})
	})
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

	return flags
}

func overrideArgsFlannel(args *networkTestsArgs) {
	args.nodePortNodes = networkflags.NodePortNodesWorkers
}

func ovverideArgsClusterAPI(args *networkTestsArgs) {
	args.loadBalancer = false
}

func overrideArgsKind(args *networkTestsArgs) {
	args.loadBalancer = false
}

func RestartPods(cl client.Client) {
	podList := &corev1.PodList{}
	Expect(
		cl.List(ctx, podList, &client.ListOptions{
			LabelSelector: labels.SelectorFromSet(labels.Set{
				gateway.GatewayComponentKey: gateway.GatewayComponentGateway,
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
