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

package statuspeer

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pterm/pterm"
	corev1 "k8s.io/api/core/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	liqoconsts "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/status"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

type TestArgsPeer struct {
	peeringType                                    discoveryv1alpha1.PeeringType
	incomingPeeringEnabled, outgoingPeeringEnabled bool
}

type TestArgsNet struct {
	networkingEnabled bool
}

type TestArgs struct {
	peer    TestArgsPeer
	net     TestArgsNet
	verbose bool
}

var _ = Describe("PeerInfo", func() {
	const (
		clusterID           = "fake"
		clusterName         = "fake"
		clusterTenant       = "fake-tenant"
		remoteClusterID     = "remote-fake"
		remoteClusterName   = "remote-fake"
		remoteClusterTenant = "remote-fake-tenant"
		// Local and Remote CIDRs
		podCIDR               = "20.1.0.0/16"
		extCIDR               = "20.2.0.0/16"
		remotePodCIDR         = "20.3.0.0/16"
		remoteExtCIDR         = "20.4.0.0/16"
		remoteRemappedPodCIDR = "20.5.0.0/16"
		remoteRemappedExtCIDR = "20.6.0.0/16"
	)

	var (
		remoteClusterIdentity = discoveryv1alpha1.ClusterIdentity{
			ClusterID:   remoteClusterID,
			ClusterName: remoteClusterName,
		}
		clientBuilder fake.ClientBuilder
		pic           *PeerInfoChecker
		ctx           context.Context
		text          string
		options       status.Options
		baseObjects   = []client.Object{
			testutil.FakeClusterIDConfigMap(liqoconsts.DefaultLiqoNamespace, clusterID, clusterName),
			testutil.FakeLiqoAuthService(corev1.ServiceTypeLoadBalancer),
		}
	)

	BeforeEach(func() {
		Skip("skipping test")

		ctx = context.Background()
		clientBuilder = *fake.NewClientBuilder().WithScheme(scheme.Scheme)
		options = status.Options{Factory: factory.NewForLocal()}
		options.Printer = output.NewFakePrinter(GinkgoWriter)
		options.KubeClient = k8sfake.NewSimpleClientset()
	})

	Context("Creating a new PeerInfoChecker", func() {
		JustBeforeEach(func() {
			pic = NewPeerInfoChecker(&options)
		})
		It("should return a valid PeerInfoChecker", func() {
			Expect(pic.peerInfoSection).To(Equal(output.NewRootSection()))
		})
	})

	DescribeTable("Collecting and Formatting PeerInfoChecker", []interface{}{
		func(args TestArgs) {
			objects := append([]client.Object{}, baseObjects...)

			var outgoingEnabled, incomingEnabled discoveryv1alpha1.PeeringEnabledType
			var outgoingConditionStatus, incomingConditionStatus, networkConditionStatus discoveryv1alpha1.PeeringConditionStatusType

			outgoingEnabled = discoveryv1alpha1.PeeringEnabledNo
			incomingEnabled = discoveryv1alpha1.PeeringEnabledNo
			outgoingConditionStatus = discoveryv1alpha1.PeeringConditionStatusNone
			incomingConditionStatus = discoveryv1alpha1.PeeringConditionStatusNone
			networkConditionStatus = discoveryv1alpha1.PeeringConditionStatusDisabled

			if args.peer.outgoingPeeringEnabled {
				outgoingEnabled = discoveryv1alpha1.PeeringEnabledYes
				outgoingConditionStatus = discoveryv1alpha1.PeeringConditionStatusEstablished
			}

			if args.peer.incomingPeeringEnabled {
				incomingEnabled = discoveryv1alpha1.PeeringEnabledYes
				incomingConditionStatus = discoveryv1alpha1.PeeringConditionStatusEstablished
			}

			var conf *networkingv1alpha1.Configuration
			var conn *networkingv1alpha1.Connection
			var gwServer *networkingv1alpha1.GatewayServer
			var gwClient *networkingv1alpha1.GatewayClient
			if args.net.networkingEnabled {
				networkConditionStatus = discoveryv1alpha1.PeeringConditionStatusEstablished
				conf = testutil.FakeConfiguration(remoteClusterID, podCIDR, extCIDR,
					remotePodCIDR, remoteExtCIDR, remoteRemappedPodCIDR, remoteRemappedExtCIDR)
				conn = testutil.FakeConnection(remoteClusterID)
				gwServer = testutil.FakeGatewayServer(remoteClusterID)
				gwClient = testutil.FakeGatewayClient(remoteClusterID)
				objects = append(objects, conf, conn, gwServer, gwClient)
			}

			objects = append(objects,
				testutil.FakeForeignCluster(remoteClusterIdentity, remoteClusterTenant, args.peer.peeringType,
					outgoingEnabled, incomingEnabled, outgoingConditionStatus, incomingConditionStatus, networkConditionStatus),
			)

			clientBuilder.WithObjects(objects...)
			options.NetworkingEnabled = args.net.networkingEnabled
			options.Verbose = args.verbose
			options.LiqoNamespace = liqoconsts.DefaultLiqoNamespace
			options.CRClient = clientBuilder.Build()
			pic = NewPeerInfoChecker(&options, remoteClusterName)
			pic.Collect(ctx)

			text = pic.Format()
			text = pterm.RemoveColorFromString(text)
			text = testutil.SqueezeWhitespaces(text)

			Expect(pic.HasSucceeded()).To(BeTrue())

			// Peering
			Expect(text).To(ContainSubstring(
				pterm.Sprintf("%s - %s", remoteClusterName, remoteClusterID),
			))
			Expect(text).To(ContainSubstring(
				pterm.Sprintf("Type: %s", args.peer.peeringType),
			))
			if args.peer.incomingPeeringEnabled {
				Expect(text).To(ContainSubstring(
					pterm.Sprintf("Incoming: %s", discoveryv1alpha1.PeeringConditionStatusEstablished),
				))
			} else {
				Expect(text).To(ContainSubstring(
					pterm.Sprintf("Incoming: %s", discoveryv1alpha1.PeeringConditionStatusNone),
				))
			}
			if args.peer.outgoingPeeringEnabled {
				Expect(text).To(ContainSubstring(
					pterm.Sprintf("Outgoing: %s", discoveryv1alpha1.PeeringConditionStatusEstablished),
				))
			} else {
				Expect(text).To(ContainSubstring(
					pterm.Sprintf("Outgoing: %s", discoveryv1alpha1.PeeringConditionStatusNone),
				))
			}

			// Authentication
			Expect(text).To(ContainSubstring(
				pterm.Sprintf("Authentication Status: %s", discoveryv1alpha1.PeeringConditionStatusEstablished),
			))
			if args.verbose {
				Expect(text).To(ContainSubstring(
					pterm.Sprintf("Auth URL: %s", testutil.ForeignAuthURL),
				))
			}

			// Network
			if args.net.networkingEnabled {
				Expect(text).To(ContainSubstring(
					pterm.Sprintf("Network Status: %s", discoveryv1alpha1.PeeringConditionStatusEstablished),
				))
				if args.verbose {
					expectNetworkSectionToBeCorrect(text, clusterName, remoteClusterName, conf)
				}

			} else {
				Expect(text).To(ContainSubstring(
					pterm.Sprintf("Network Status: %s", discoveryv1alpha1.PeeringConditionStatusDisabled),
				))
			}

			// API Server
			Expect(text).To(ContainSubstring(
				pterm.Sprintf("API Server Status: %s", discoveryv1alpha1.PeeringConditionStatusEstablished),
			))
			if args.verbose {
				Expect(text).To(ContainSubstring(
					pterm.Sprintf("API Server URL: %s", testutil.ForeignAPIServerURL),
				))
				Expect(text).To(ContainSubstring(
					pterm.Sprintf("API Server Proxy URL: %s", testutil.ForeignProxyURL),
				))
			}

		}, forgeTestMatrix(),
	}...,
	)
})

func expectNetworkSectionToBeCorrect(text, clusterName, remoteClusterName string, conf *networkingv1alpha1.Configuration) {
	cidrs := pterm.Sprintf("CIDRs")
	local := pterm.Sprintf("Local Cluster")
	local = pterm.Sprintf("%s Pod CIDR: %s External CIDR: %s", local, conf.Spec.Local.CIDR.Pod, conf.Spec.Local.CIDR.External)
	remote := pterm.Sprintf("Remote Cluster")
	remote = pterm.Sprintf("%s Original", remote)
	remote = pterm.Sprintf("%s Pod CIDR: %s External CIDR: %s", remote, conf.Spec.Remote.CIDR.Pod, conf.Spec.Remote.CIDR.External)
	remote = pterm.Sprintf("%s Remapped - how %q remapped %q", remote, clusterName, remoteClusterName)
	remote = pterm.Sprintf("%s Pod CIDR: %s External CIDR: %s", remote, conf.Status.Remote.CIDR.Pod, conf.Status.Remote.CIDR.External)

	Expect(text).To(ContainSubstring(cidrs))
	Expect(text).To(ContainSubstring(local))
	Expect(text).To(ContainSubstring(remote))
}

func forgeTestTableEntry(verbose bool, peeringType discoveryv1alpha1.PeeringType,
	incomingPeeringEnabled bool, outgoingPeeringEnabled bool, networkingEnabled bool,
) TableEntry {
	msg := pterm.Sprintf(`verbose: %t, peeringType: %s, incomingPeeringEnabled: %t, outgoingPeeringEnabled: %t, networkingEnabled: %t`,
		verbose, peeringType, incomingPeeringEnabled, outgoingPeeringEnabled, networkingEnabled)
	return Entry(msg, TestArgs{
		verbose: verbose,
		peer: TestArgsPeer{
			peeringType:            peeringType,
			incomingPeeringEnabled: incomingPeeringEnabled,
			outgoingPeeringEnabled: outgoingPeeringEnabled,
		},
		net: TestArgsNet{
			networkingEnabled: networkingEnabled,
		},
	})
}

func forgeTestMatrix() []TableEntry {
	testMatrix := []TableEntry{}

	for _, verbose := range []bool{true, false} {
		for _, peeringType := range []discoveryv1alpha1.PeeringType{
			discoveryv1alpha1.PeeringTypeInBand,
			discoveryv1alpha1.PeeringTypeOutOfBand,
		} {
			for _, incomingPeeringEnabled := range []bool{true, false} {
				for _, outgoingPeeringEnabled := range []bool{true, false} {

					for _, networkingEnabled := range []bool{true, false} {
						// excludes cases where the network is not enabled and the peering is in-band
						if !networkingEnabled && peeringType == discoveryv1alpha1.PeeringTypeInBand {
							continue
						}
						testMatrix = append(testMatrix, forgeTestTableEntry(verbose, peeringType,
							incomingPeeringEnabled, outgoingPeeringEnabled, networkingEnabled))
					}
				}
			}
		}
	}
	return testMatrix
}
