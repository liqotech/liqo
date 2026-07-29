// Copyright 2019-2026 The Liqo Authors
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
	"slices"
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
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
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

	// fabricPodLabelKey and fabricPodLabelValue select the fabric pods, one per node.
	fabricPodLabelKey   = "app.kubernetes.io/name"
	fabricPodLabelValue = "fabric"
	// geneveContainerName is the container of the gateway pod owning the geneve interfaces.
	geneveContainerName = "geneve"
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
				restartTime := time.Now()
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

				// Wait for the connections to be re-established after the restart, instead of assuming
				// a duration for it: a fixed wait hides how long the failover actually takes, and the
				// checks below would otherwise be free to observe the state preceding the restart.
				for i := range testContext.Clusters {
					Eventually(func() error {
						return checkConnectionsReady(testContext.Clusters[i].ControllerClient, restartTime)
					}, timeout, interval).Should(Succeed())
				}

				// Check that the internal fabric caught up with the new gateway pods.
				for i := range testContext.Clusters {
					Eventually(func() error {
						return checkInternalFabricConverged(testContext.Clusters[i].ControllerClient)
					}, timeout, interval).Should(Succeed())
				}

				// Check that what the internal fabric caught up with is also what the datapath needs.
				for i := range testContext.Clusters {
					Eventually(func() error {
						return checkSourceIPsMatchRoutes(&testContext.Clusters[i], testContext.Namespace)
					}, timeout, interval).Should(Succeed())
					Eventually(func() error {
						return checkGeneveTunnelsProgrammed(&testContext.Clusters[i])
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

					restartTime := time.Now()

					// Check if there is only one active gateway pod per remote cluster.
					for j := range testContext.Clusters {
						numActiveGateway := testContext.Clusters[j].NumPeeredConsumers + testContext.Clusters[j].NumPeeredProviders
						Eventually(func() error {
							return checkUniqueActiveGatewayPod(testContext.Clusters[j].ControllerClient, numActiveGateway)
						}, timeout, interval).Should(Succeed())
					}

					for j := range testContext.Clusters {
						Eventually(func() error {
							return checkConnectionsReady(testContext.Clusters[j].ControllerClient, restartTime)
						}, timeout, interval).Should(Succeed())
					}

					// Connections only cover the inter-gateway tunnel: wait for the internal
					// fabric to catch up with the new gateway pods before probing the datapath.
					for j := range testContext.Clusters {
						Eventually(func() error {
							return checkInternalFabricConverged(testContext.Clusters[j].ControllerClient)
						}, timeout, interval).Should(Succeed())
					}

					// Check that what the internal fabric caught up with is also what the datapath needs.
					for j := range testContext.Clusters {
						Eventually(func() error {
							return checkSourceIPsMatchRoutes(&testContext.Clusters[j], testContext.Namespace)
						}, timeout, interval).Should(Succeed())
						Eventually(func() error {
							return checkGeneveTunnelsProgrammed(&testContext.Clusters[j])
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

func checkConnectionsReady(cl client.Client, restartTime time.Time) error {
	connectionList := &networkingv1beta1.ConnectionList{}
	if err := cl.List(ctx, connectionList); err != nil {
		return fmt.Errorf("unable to list connections: %w", err)
	}

	for i := range connectionList.Items {
		conn := &connectionList.Items[i]
		if conn.Status.Value != networkingv1beta1.Connected {
			return fmt.Errorf("connection %s/%s is not connected (status: %s)",
				conn.Namespace, conn.Name, conn.Status.Value)
		}
		if !conn.Status.Latency.Timestamp.After(restartTime) {
			return fmt.Errorf("connection %s/%s latency timestamp %s is not after restart time %s",
				conn.Namespace, conn.Name, conn.Status.Latency.Timestamp, restartTime)
		}
	}
	return nil
}

// checkInternalFabricConverged checks that the internal fabric has been reconfigured for the
// gateway pods that are currently active.
//
// Connections only tell that the inter-gateway tunnel is up: they are probed over the tunnel
// interface and never traverse the geneve tunnels connecting the nodes to the gateways. After a
// gateway pod is replaced, the geneve tunnels are reprogrammed from two resources, and until both
// caught up the node-to-gateway datapath may still point to the previous pod:
//
//   - InternalFabric.Spec.GatewayIP, the address the nodes use as the remote end of their geneve
//     tunnel, which must be the IP of the active gateway pod;
//   - InternalNode.Status.NodeIP, the address the gateway uses as the remote end of its own geneve
//     tunnel towards each node. Local is the source a node uses to reach a gateway pod scheduled on
//     itself, Remote the one it uses to reach a gateway pod on another node: which of the two is
//     needed depends on where the active gateway pod is currently scheduled.
//
// With CNIs where the two sources differ (e.g. Flannel, where a node reaches local pods from cni0
// and remote pods from flannel.1) a stale value makes the gateway drop every packet coming from
// that node, so probe the datapath only once both resources match the current placement.
func checkInternalFabricConverged(cl client.Client) error {
	activeGatewayPods, err := listActiveGatewayPods(cl)
	if err != nil {
		return err
	}

	activeGatewayIPs := make(map[string]any)
	for i := range activeGatewayPods.Items {
		activeGatewayIPs[activeGatewayPods.Items[i].Status.PodIP] = struct{}{}
	}

	internalFabricList := &networkingv1beta1.InternalFabricList{}
	if err := cl.List(ctx, internalFabricList); err != nil {
		return fmt.Errorf("unable to list internalfabrics: %w", err)
	}

	for i := range internalFabricList.Items {
		internalFabric := &internalFabricList.Items[i]
		if _, ok := activeGatewayIPs[internalFabric.Spec.GatewayIP.String()]; !ok {
			return fmt.Errorf("internalfabric %s/%s still points to %s, which is not an active gateway pod",
				internalFabric.Namespace, internalFabric.Name, internalFabric.Spec.GatewayIP)
		}
	}

	internalNodeList := &networkingv1beta1.InternalNodeList{}
	if err := cl.List(ctx, internalNodeList); err != nil {
		return fmt.Errorf("unable to list internalnodes: %w", err)
	}

	for i := range internalNodeList.Items {
		internalNode := &internalNodeList.Items[i]
		for j := range activeGatewayPods.Items {
			pod := &activeGatewayPods.Items[j]
			if pod.Spec.NodeName == internalNode.Name {
				if internalNode.Status.NodeIP.Local == nil {
					return fmt.Errorf("internalnode %s hosts the active gateway pod %s/%s but has no local source IP yet",
						internalNode.Name, pod.Namespace, pod.Name)
				}
			} else if internalNode.Status.NodeIP.Remote == nil {
				return fmt.Errorf("internalnode %s has no remote source IP yet to reach the active gateway pod %s/%s",
					internalNode.Name, pod.Namespace, pod.Name)
			}
		}
	}

	return nil
}

// checkSourceIPsMatchRoutes checks that the source IPs recorded in the InternalNodes are the ones
// the nodes actually use to reach the active gateway pods.
//
// checkInternalFabricConverged only requires those fields to be set, and a value sampled while the
// CNI was still programming the routes of a node is set as well: a node which joins a cluster whose
// peerings are already established samples the source towards the gateways as soon as it sees them,
// and when the route is not there yet the lookup falls back to the default route and records an
// address the node stops using seconds later. The gateway then builds a geneve tunnel whose remote
// never matches the packets that node sends, and the datapath fails as a plain curl timeout, minutes
// later and with nothing pointing at the cause. Comparing what was recorded with what the node
// really uses reports the mismatch itself.
func checkSourceIPsMatchRoutes(cluster *tester.ClusterContext, liqoNamespace string) error {
	activeGatewayPods, err := listActiveGatewayPods(cluster.ControllerClient)
	if err != nil {
		return err
	}

	internalNodeList := &networkingv1beta1.InternalNodeList{}
	if err := cluster.ControllerClient.List(ctx, internalNodeList); err != nil {
		return fmt.Errorf("unable to list internalnodes: %w", err)
	}

	fabricPods := &corev1.PodList{}
	if err := cluster.ControllerClient.List(ctx, fabricPods, client.InNamespace(liqoNamespace),
		client.MatchingLabels{fabricPodLabelKey: fabricPodLabelValue}); err != nil {
		return fmt.Errorf("unable to list the fabric pods: %w", err)
	}

	for i := range internalNodeList.Items {
		internalNode := &internalNodeList.Items[i]
		fabric := fabricPodOnNode(fabricPods, internalNode.Name)
		if fabric == nil {
			return fmt.Errorf("no fabric pod found on node %s", internalNode.Name)
		}

		for j := range activeGatewayPods.Items {
			pod := &activeGatewayPods.Items[j]

			recorded, kind := internalNode.Status.NodeIP.Remote, "remote"
			if pod.Spec.NodeName == internalNode.Name {
				recorded, kind = internalNode.Status.NodeIP.Local, "local"
			}
			if recorded == nil {
				return fmt.Errorf("internalnode %s has no %s source IP yet for the active gateway pod %s/%s",
					internalNode.Name, kind, pod.Namespace, pod.Name)
			}

			actual, err := sourceIPTowards(cluster, fabric, liqoNamespace, pod.Status.PodIP)
			if err != nil {
				return err
			}

			if actual != recorded.String() {
				return fmt.Errorf(
					"internalnode %s records %s source IP %s to reach the gateway pod %s/%s (%s), but node %s sources from %s",
					internalNode.Name, kind, recorded, pod.Namespace, pod.Name, pod.Status.PodIP, internalNode.Name, actual)
			}
		}
	}

	return nil
}

// checkGeneveTunnelsProgrammed checks that every active gateway pod has a geneve interface towards
// every node of its cluster.
//
// The gateway skips the nodes whose source IP is not set yet, and a gateway which becomes active
// needs the field that the node it runs on, and the node the previous gateway ran on, have never had
// populated: until the fabric fills them the peering has no internal fabric at all, and every check
// crossing it fails. Asserting the interfaces are there makes the datapath probes start once the
// gateway is really able to carry them.
func checkGeneveTunnelsProgrammed(cluster *tester.ClusterContext) error {
	activeGatewayPods, err := listActiveGatewayPods(cluster.ControllerClient)
	if err != nil {
		return err
	}

	internalNodeList := &networkingv1beta1.InternalNodeList{}
	if err := cluster.ControllerClient.List(ctx, internalNodeList); err != nil {
		return fmt.Errorf("unable to list internalnodes: %w", err)
	}

	for j := range activeGatewayPods.Items {
		pod := &activeGatewayPods.Items[j]

		stdout, stderr, err := util.ExecCmdInContainer(ctx, cluster.Config, cluster.NativeClient,
			pod.Name, pod.Namespace, geneveContainerName, "ip -d link show type geneve")
		if err != nil {
			return fmt.Errorf("unable to list the geneve interfaces of gateway pod %s/%s: %w (%s)",
				pod.Namespace, pod.Name, err, stderr)
		}
		programmed := parseGeneveRemotes(stdout)

		for i := range internalNodeList.Items {
			internalNode := &internalNodeList.Items[i]

			expected := internalNode.Status.NodeIP.Remote
			if pod.Spec.NodeName == internalNode.Name {
				expected = internalNode.Status.NodeIP.Local
			}
			if expected == nil {
				return fmt.Errorf("internalnode %s has no source IP yet for the active gateway pod %s/%s",
					internalNode.Name, pod.Namespace, pod.Name)
			}

			if !slices.Contains(programmed, expected.String()) {
				return fmt.Errorf("gateway pod %s/%s has no geneve interface towards %s (node %s), but only %v",
					pod.Namespace, pod.Name, expected, internalNode.Name, programmed)
			}
		}
	}

	return nil
}

// listActiveGatewayPods returns the gateway pods which are currently active, once they all have an
// address: the checks above compare that address with what the other resources point to.
func listActiveGatewayPods(cl client.Client) (*corev1.PodList, error) {
	activeGatewayPods := &corev1.PodList{}
	if err := cl.List(ctx, activeGatewayPods, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(gateway.ForgeActiveGatewayPodLabels()),
	}); err != nil {
		return nil, fmt.Errorf("unable to list active gateway pods: %w", err)
	}

	for i := range activeGatewayPods.Items {
		pod := &activeGatewayPods.Items[i]
		if pod.Status.PodIP == "" {
			return nil, fmt.Errorf("active gateway pod %s/%s has no IP yet", pod.Namespace, pod.Name)
		}
	}

	return activeGatewayPods, nil
}

// fabricPodOnNode returns the fabric pod running on the given node, nil when there is none.
func fabricPodOnNode(pods *corev1.PodList, node string) *corev1.Pod {
	for i := range pods.Items {
		if pods.Items[i].Spec.NodeName == node {
			return &pods.Items[i]
		}
	}
	return nil
}

// sourceIPTowards returns the address the node of the given fabric pod uses to reach dst. The fabric
// runs in the host network, so its routing table is the one of the node. Its image carries no
// iproute2, only the ip applet of busybox: the plain "route get" both provide is enough here, as the
// source address is read out of the fields of the answer.
func sourceIPTowards(cluster *tester.ClusterContext, fabric *corev1.Pod, liqoNamespace, dst string) (string, error) {
	stdout, stderr, err := util.ExecCmd(ctx, cluster.Config, cluster.NativeClient,
		fabric.Name, liqoNamespace, fmt.Sprintf("ip route get %s", dst))
	if err != nil {
		return "", fmt.Errorf("unable to get the route to %s from node %s: %w (%s)",
			dst, fabric.Spec.NodeName, err, stderr)
	}

	fields := strings.Fields(stdout)
	for i := 0; i < len(fields)-1; i++ {
		if fields[i] == "src" {
			return fields[i+1], nil
		}
	}

	return "", fmt.Errorf("no source address in the route to %s from node %s: %q", dst, fabric.Spec.NodeName, stdout)
}

// parseGeneveRemotes returns the remote addresses of the interfaces listed by
// "ip -d link show type geneve", which carry them on their geneve detail line.
func parseGeneveRemotes(out string) []string {
	remotes := []string{}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		for i := 0; i < len(fields)-1; i++ {
			if fields[i] == "remote" {
				remotes = append(remotes, fields[i+1])
				break
			}
		}
	}
	return remotes
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
