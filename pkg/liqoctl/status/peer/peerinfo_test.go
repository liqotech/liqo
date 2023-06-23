// Copyright 2019-2023 The Liqo Authors
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
	"k8s.io/apimachinery/pkg/api/resource"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	liqoconsts "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/status"
	"github.com/liqotech/liqo/pkg/liqoctl/status/utils/resources"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

type TestArgsPeer struct {
	peeringType                                    discoveryv1alpha1.PeeringType
	incomingPeeringEnabled, outgoingPeeringEnabled bool
}

type TestArgsNet struct {
	internalNetworkEnabled, remapped bool
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
		// NetworkCOnfig
		// podCIDR is the CIDR of the local pod network used for testing.
		podCIDR    = "20.1.0.0/16"
		extCIDR    = "20.2.0.0/16"
		podCIDRNAT = "20.3.0.0/16"
		extCIDRNAT = "20.4.0.0/16"
		podCIDRalt = "20.5.0.0/16"
		extCIDRalt = "20.6.0.0/16"
	)

	var (
		remoteClusterIdentity = discoveryv1alpha1.ClusterIdentity{
			ClusterID:   remoteClusterID,
			ClusterName: remoteClusterName,
		}
		clientBuilder   fake.ClientBuilder
		pic             *PeerInfoChecker
		ctx             context.Context
		text            string
		options         status.Options
		sharedResources = corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("1000m"),
			corev1.ResourceMemory:           resource.MustParse("16G"),
			corev1.ResourcePods:             resource.MustParse("100"),
			corev1.ResourceEphemeralStorage: resource.MustParse("400Gi"),
		}
		acquiredResources = corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("500m"),
			corev1.ResourceMemory:           resource.MustParse("8G"),
			corev1.ResourcePods:             resource.MustParse("50"),
			corev1.ResourceEphemeralStorage: resource.MustParse("200Gi"),
		}
		baseObjects = []client.Object{
			testutil.FakeClusterIDConfigMap(liqoconsts.DefaultLiqoNamespace, clusterID, clusterName),
			testutil.FakeLiqoAuthService(corev1.ServiceTypeLoadBalancer),
			testutil.FakeLiqoGatewayService(corev1.ServiceTypeLoadBalancer),
			testutil.FakeTunnelEndpoint(&remoteClusterIdentity, remoteClusterTenant),
			testutil.FakeSharedResourceOffer(&remoteClusterIdentity, remoteClusterTenant, clusterName, sharedResources),
			testutil.FakeAcquiredResourceOffer(&remoteClusterIdentity, remoteClusterTenant, acquiredResources),
		}
	)

	BeforeEach(func() {
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
			networkConditionStatus = discoveryv1alpha1.PeeringConditionStatusExternal

			if args.peer.outgoingPeeringEnabled {
				outgoingEnabled = discoveryv1alpha1.PeeringEnabledYes
				outgoingConditionStatus = discoveryv1alpha1.PeeringConditionStatusEstablished
			}

			if args.peer.incomingPeeringEnabled {
				incomingEnabled = discoveryv1alpha1.PeeringEnabledYes
				incomingConditionStatus = discoveryv1alpha1.PeeringConditionStatusEstablished
			}

			var localNc, remoteNc *netv1alpha1.NetworkConfig
			if args.net.internalNetworkEnabled {
				networkConditionStatus = discoveryv1alpha1.PeeringConditionStatusEstablished
				if args.net.remapped {
					localNc = testutil.FakeNetworkConfig(true, clusterName, remoteClusterTenant,
						podCIDR, extCIDR, podCIDRNAT, extCIDRNAT)
					remoteNc = testutil.FakeNetworkConfig(false, remoteClusterName, remoteClusterTenant,
						podCIDR, extCIDR, podCIDRNAT, extCIDRNAT)
				} else {
					localNc = testutil.FakeNetworkConfig(true, clusterName, remoteClusterTenant,
						podCIDR, extCIDR, "None", "None")
					remoteNc = testutil.FakeNetworkConfig(false, remoteClusterName, remoteClusterTenant,
						podCIDRalt, extCIDRalt, "None", "None")
				}
				objects = append(objects, localNc, remoteNc)
			}

			objects = append(objects,
				testutil.FakeForeignCluster(remoteClusterIdentity, remoteClusterTenant, args.peer.peeringType,
					outgoingEnabled, incomingEnabled, outgoingConditionStatus, incomingConditionStatus, networkConditionStatus),
			)

			clientBuilder.WithObjects(objects...)
			options.InternalNetworkEnabled = args.net.internalNetworkEnabled
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
			if args.net.internalNetworkEnabled {
				Expect(text).To(ContainSubstring(
					pterm.Sprintf("Network Status: %s", discoveryv1alpha1.PeeringConditionStatusEstablished),
				))
				if args.verbose {
					expectNetworkSectionToBeCorrect(text, clusterName, remoteClusterName,
						args.net.remapped, localNc, remoteNc)
				}

			} else {
				Expect(text).To(ContainSubstring(
					pterm.Sprintf("Network Status: %s", discoveryv1alpha1.PeeringConditionStatusExternal),
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

			// Resources
			if args.peer.outgoingPeeringEnabled {
				Expect(text).To(ContainSubstring(
					pterm.Sprintf("Total acquired - resources offered by %q to %q", remoteClusterName, clusterName),
				))
				expectResourcesToBeContainedIn(text, acquiredResources)
			}
			if args.peer.incomingPeeringEnabled {
				Expect(text).To(ContainSubstring(
					pterm.Sprintf("Total shared - resources offered by %q to %q", clusterName, remoteClusterName),
				))
				expectResourcesToBeContainedIn(text, sharedResources)
			}

		}, forgeTestMatrix(),
	}...,
	)
})

func expectNetworkSectionToBeCorrect(text, clusterName, remoteClusterName string, remapped bool, localNc, remoteNc *netv1alpha1.NetworkConfig) {
	local := pterm.Sprintf("Local CIDRs Original")
	local = pterm.Sprintf("%s Pod CIDR: %s External CIDR: %s", local, localNc.Spec.PodCIDR, localNc.Spec.ExternalCIDR)
	remote := pterm.Sprintf("Remote CIDRs Original")
	remote = pterm.Sprintf("%s Pod CIDR: %s External CIDR: %s", remote, remoteNc.Spec.PodCIDR, remoteNc.Spec.ExternalCIDR)
	local = pterm.Sprintf("%s Remapped - how %q has been remapped by %q",
		local, clusterName, remoteClusterName)
	remote = pterm.Sprintf("%s Remapped - how %q remapped %q",
		remote, clusterName, remoteClusterName)
	if remapped {
		local = pterm.Sprintf("%s Pod CIDR: %s External CIDR: %s",
			local, localNc.Status.PodCIDRNAT, localNc.Status.ExternalCIDRNAT)
		remote = pterm.Sprintf("%s Pod CIDR: %s External CIDR: %s",
			remote, remoteNc.Status.PodCIDRNAT, remoteNc.Status.ExternalCIDRNAT)
	} else {
		local = pterm.Sprintf("%s Pod CIDR: remapping not necessary External CIDR: remapping not necessary", local)
		remote = pterm.Sprintf("%s Pod CIDR: remapping not necessary External CIDR: remapping not necessary", remote)
	}
	Expect(text).To(ContainSubstring(local))
	Expect(text).To(ContainSubstring(remote))
}

func expectResourcesToBeContainedIn(text string, genericResources corev1.ResourceList) {
	res := map[corev1.ResourceName]string{
		corev1.ResourceCPU:              resources.CPU(genericResources),
		corev1.ResourceMemory:           resources.Memory(genericResources),
		corev1.ResourcePods:             resources.Pods(genericResources),
		corev1.ResourceEphemeralStorage: resources.EphemeralStorage(genericResources),
	}
	for k, v := range res {
		Expect(text).To(ContainSubstring(
			pterm.Sprintf("%s: %s", k, v),
		))
	}
}

func forgeTestTableEntry(verbose bool, peeringType discoveryv1alpha1.PeeringType,
	incomingPeeringEnabled bool, outgoingPeeringEnabled bool, remapped bool, internalNetworkEnabled bool,
) TableEntry {
	msg := pterm.Sprintf(`verbose: %t, peeringType: %s, incomingPeeringEnabled: %t, outgoingPeeringEnabled: %t, 
		remapped: %t, internalNetworkEnabled: %t`,
		verbose, peeringType, incomingPeeringEnabled, outgoingPeeringEnabled, remapped, internalNetworkEnabled)
	return Entry(msg, TestArgs{
		verbose: verbose,
		peer: TestArgsPeer{
			peeringType:            peeringType,
			incomingPeeringEnabled: incomingPeeringEnabled,
			outgoingPeeringEnabled: outgoingPeeringEnabled,
		},
		net: TestArgsNet{
			remapped:               remapped,
			internalNetworkEnabled: internalNetworkEnabled,
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
					for _, remapped := range []bool{true, false} {
						for _, internalNetworkEnabled := range []bool{true, false} {
							// excludes cases where the network is not enabled and the peering is in-band
							if !internalNetworkEnabled && peeringType == discoveryv1alpha1.PeeringTypeInBand {
								continue
							}
							// avoid to test the same case twice, when the network is not enabled
							// or the verbose flag is not set the remapped value does not affects the output.
							if remapped {
								if !internalNetworkEnabled || !verbose {
									continue
								}
							}
							testMatrix = append(testMatrix, forgeTestTableEntry(verbose, peeringType,
								incomingPeeringEnabled, outgoingPeeringEnabled, remapped, internalNetworkEnabled))
						}
					}
				}
			}
		}
	}
	return testMatrix
}
