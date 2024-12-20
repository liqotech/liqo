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
//

package peer

import (
	"context"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pterm/pterm"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/info"
	"github.com/liqotech/liqo/pkg/liqoctl/info/common"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	cidrutils "github.com/liqotech/liqo/pkg/utils/cidr"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

var _ = Describe("NetworkChecker tests", func() {
	remoteClusterID := "fake"
	expectedAlerts := []string{"This is an alert", "this another"}

	var (
		clientBuilder fake.ClientBuilder
		nc            *NetworkChecker
		ctx           context.Context
		options       info.Options
	)

	BeforeEach(func() {
		ctx = context.Background()
		clientBuilder = *fake.NewClientBuilder().WithScheme(scheme.Scheme)

		options = info.Options{Factory: factory.NewForLocal()}
		options.Printer = output.NewFakePrinter(GinkgoWriter)
		options.KubeClient = k8sfake.NewSimpleClientset()
	})

	Describe("Testing the NetworkChecker", func() {
		Context("Collecting and retrieving the data", func() {
			It("should collect the data and return the right result", func() {
				expectedPodCIDR := "10.0.0.0/24"
				expectedRemappedPod := "11.0.0.0/24"
				expectedExternalCIDR := "10.0.1.0/24"
				expectedRemappedExternal := "11.0.1.0/24"

				foreignclusterRes := testutil.FakeForeignCluster(liqov1beta1.ClusterID(remoteClusterID), &liqov1beta1.Modules{
					Networking: liqov1beta1.Module{
						Enabled: true,
						Conditions: []liqov1beta1.Condition{
							{
								Status: liqov1beta1.ConditionStatusEstablished,
							},
						},
					},
					Authentication: liqov1beta1.Module{Enabled: true},
					Offloading:     liqov1beta1.Module{Enabled: true},
				})
				configurationRes := testutil.FakeConfiguration(
					remoteClusterID, expectedPodCIDR, expectedExternalCIDR, expectedPodCIDR, expectedExternalCIDR,
					expectedRemappedPod, expectedRemappedExternal)

				gatewayRes := testutil.FakeGatewayClient(remoteClusterID)

				// Set up the fake clients
				clientBuilder.WithObjects(configurationRes, gatewayRes)
				options.CRClient = clientBuilder.Build()

				options.ClustersInfo = map[liqov1beta1.ClusterID]*liqov1beta1.ForeignCluster{
					liqov1beta1.ClusterID(remoteClusterID): foreignclusterRes,
				}

				By("Collecting the data")
				nc = &NetworkChecker{}
				nc.Collect(ctx, options)

				By("Verifying that no errors have been raised")
				Expect(nc.GetCollectionErrors()).To(BeEmpty(), "Unexpected collection errors detected")

				By("Checking the correctness of the data in the struct")
				rawData, err := nc.GetDataByClusterID(liqov1beta1.ClusterID(remoteClusterID))
				Expect(err).NotTo(HaveOccurred(), "An error occurred while getting the data")

				data := rawData.(Network)

				Expect(data.Status).To(Equal(common.ModuleHealthy), "Unexpected status")

				Expect(data.CIDRs.Remote.Pod).To(HaveLen(1), "Unexpected alerts")
				Expect(data.CIDRs.Remote.Pod).To(ContainElement(networkingv1beta1.CIDR(expectedPodCIDR)), "Unexpected remote Pod CIDR")

				Expect(data.CIDRs.Remote.External).To(HaveLen(1), "Unexpected alerts")
				Expect(data.CIDRs.Remote.External).To(ContainElement(networkingv1beta1.CIDR(expectedExternalCIDR)), "Unexpected remote external CIDR")

				Expect(data.CIDRs.Remapped).NotTo(BeNil(), "Expected remapped CIDRs but empty received")

				Expect(data.CIDRs.Remapped.Pod).To(HaveLen(1), "Unexpected alerts")
				Expect(data.CIDRs.Remapped.Pod).To(ContainElement(networkingv1beta1.CIDR(expectedRemappedPod)), "Unexpected remapped Pod CIDR")

				Expect(data.CIDRs.Remapped.External).To(HaveLen(1), "Unexpected alerts")
				Expect(data.CIDRs.Remapped.External).To(ContainElement(networkingv1beta1.CIDR(expectedRemappedExternal)), "Unexpected remapped external CIDR")

				// Check gateway parameters
				Expect(data.Gateway.Role).To(Equal(GatewayServerType), "Unexpected Gateway type")
				Expect(data.Gateway.Address).To(Equal(gatewayRes.Spec.Endpoint.Addresses), "Unexpected gateway addresses")
				Expect(data.Gateway.Port).To(Equal(gatewayRes.Spec.Endpoint.Port), "Unexpected gateway addresses")
			})
		})

		DescribeTable("collectGatewayInfo function test", func(gatewayType GatewayType) {
			var objects []client.Object
			switch gatewayType {
			case GatewayServerType:
				objects = append(objects, testutil.FakeGatewayClient(remoteClusterID))
			case GatewayClientType:
				objects = append(objects, testutil.FakeGatewayServer(remoteClusterID))
			default:
				objects = append(objects,
					testutil.FakeGatewayClient(remoteClusterID), testutil.FakeGatewayServer(remoteClusterID))
			}
			// Set up the fake clients
			clientBuilder.WithObjects(objects...)
			options.CRClient = clientBuilder.Build()
			peerNetwork := Network{}

			nc = &NetworkChecker{}
			err := nc.collectGatewayInfo(ctx, options.CRClient, liqov1beta1.ClusterID(remoteClusterID), &peerNetwork)

			if gatewayType == "" {
				// Both gateway present so an error should have been raised
				Expect(err).To(HaveOccurred(), "Expected error as multiple gateways present")
			} else {
				Expect(err).NotTo(HaveOccurred(), "Unexpected error occurred")
				Expect(peerNetwork.Gateway.Role).To(Equal(gatewayType), "Unexpected gateway type")
				var endpoint networkingv1beta1.EndpointStatus
				if gatewayType == GatewayServerType {
					gateway, _ := objects[0].(*networkingv1beta1.GatewayClient)
					endpoint = gateway.Spec.Endpoint
				} else {
					gateway, _ := objects[0].(*networkingv1beta1.GatewayServer)
					endpoint = *gateway.Status.Endpoint
				}
				Expect(endpoint.Addresses).To(Equal(peerNetwork.Gateway.Address), "Unexpected gateway address")
				Expect(endpoint.Port).To(Equal(peerNetwork.Gateway.Port), "Unexpected gateway port")
			}
		},
			Entry("Gateway server", GatewayServerType),
			Entry("Gateway client", GatewayClientType),
			Entry("Both gateways present", GatewayType("")),
		)

		DescribeTable("FormatForClusterID function test", func(testCase Network) {
			nc = &NetworkChecker{}
			nc.data = map[liqov1beta1.ClusterID]Network{
				liqov1beta1.ClusterID(remoteClusterID): testCase,
			}

			text := nc.FormatForClusterID(liqov1beta1.ClusterID(remoteClusterID), options)
			text = pterm.RemoveColorFromString(text)
			text = strings.TrimSpace(testutil.SqueezeWhitespaces(text))

			if testCase.Status == common.ModuleDisabled {
				Expect(text).To(Equal("Status: Disabled"))
			} else {
				Expect(text).To(ContainSubstring(
					pterm.Sprintf("Status: %s", testCase.Status),
				), "Unexpected status shown")

				// Check that alerts are shown when list is not empty
				for _, alert := range testCase.Alerts {
					Expect(text).To(ContainSubstring(alert), "Alert not shown")
				}

				// Check CIDRs
				if testCase.CIDRs.Remapped != nil {
					Expect(text).To(ContainSubstring(
						pterm.Sprintf("Pod CIDR: %s → Remapped to %s",
							joinCidrs(testCase.CIDRs.Remote.Pod), joinCidrs(testCase.CIDRs.Remapped.Pod))),
						"Unexpected POD CIDR shown",
					)
					Expect(text).To(ContainSubstring(
						pterm.Sprintf("External CIDR: %s → Remapped to %s",
							joinCidrs(testCase.CIDRs.Remote.External), joinCidrs(testCase.CIDRs.Remapped.External))),
						"Unexpected External CIDR shown",
					)
				} else {
					Expect(text).To(ContainSubstring(
						pterm.Sprintf("Pod CIDR: %s",
							joinCidrs(testCase.CIDRs.Remote.Pod))), "Unexpected POD CIDR shown",
					)
					Expect(text).To(ContainSubstring(
						pterm.Sprintf("External CIDR: %s",
							joinCidrs(testCase.CIDRs.Remote.External))),
						"Unexpected External CIDR shown",
					)
				}

				// Check Gateway visualization
				Expect(text).To(ContainSubstring(pterm.Sprintf("Role: %s", testCase.Gateway.Role)), "Unexpected gateway role")
				for _, address := range testCase.Gateway.Address {
					Expect(text).To(ContainSubstring(address), "Unexpected gateway address")
				}
				Expect(text).To(ContainSubstring(pterm.Sprintf("Port: %d", testCase.Gateway.Port)), "Unexpected gateway port")
			}
		},
			Entry("Disabled module", Network{Status: common.ModuleDisabled}),
			Entry("Healthy module remapped CIDR", Network{
				Status: common.ModuleHealthy,
				CIDRs: CIDRInfo{
					Remote: networkingv1beta1.ClusterConfigCIDR{
						Pod:      cidrutils.SetPrimary("fakepod"),
						External: cidrutils.SetPrimary("fakeexternal"),
					},
					Remapped: &networkingv1beta1.ClusterConfigCIDR{
						Pod:      cidrutils.SetPrimary("fakeremappedpod"),
						External: cidrutils.SetPrimary("fakeremappedexternal"),
					},
				},
				Gateway: GatewayInfo{
					Address: []string{"fakeaddress", "fakeaddress2"},
					Port:    4310,
					Role:    GatewayServerType,
				},
			}),
			Entry("Healthy module", Network{
				Status: common.ModuleHealthy,
				CIDRs: CIDRInfo{
					Remote: networkingv1beta1.ClusterConfigCIDR{
						Pod:      cidrutils.SetPrimary("fakepod"),
						External: cidrutils.SetPrimary("fakeexternal"),
					},
				},
				Gateway: GatewayInfo{
					Address: []string{"10.0.0.0/24"},
					Port:    4320,
					Role:    GatewayClientType,
				},
			}),
			Entry("Healthy module CIDR", Network{
				Status: common.ModuleUnhealthy,
				Alerts: expectedAlerts,
			}),
		)
	})
})
