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

package integration_tests_test

import (
	"fmt"

	"github.com/containernetworking/plugins/pkg/ns"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	liqonetIpam "github.com/liqotech/liqo/pkg/liqonet/ipam"
	"github.com/liqotech/liqo/pkg/utils/slice"
)

var _ = Describe("EndpointReflection", func() {
	/*
		The following tests intend to check that components supporting
		applications that span across more than 2 clusters behave as they
		are supposed to do. In particular, here the focus is mainly on the IPAM
		module and on the Gateway: Virtual Kubelet is excluded from these tests.
		The IPAM module is the main actor here, as it receives requests to map Endpoint IPs
		from the VK (tests simulate this behavior). We'll ask IPAM module to
		map remote endpoint IPs, therefore we expect IPAM to use an
		IP taken from the ExternalCIDR. Whevener there's a remapping of this type
		the IPAM should notify the Gateway about it. The GW in turn will
		insert a new DNAT rule: tests will ensure those rules are present.
	*/
	Describe("Endpoint IP mapping", func() {
		Context("Map the endpoint IP of a remote endpoint", func() {
			It("DNAT rule should be inserted by the natmappingoperator", func() {
				response, err := ipam.MapEndpointIP(ctx, &liqonetIpam.MapRequest{
					ClusterID: clusterID1,
					Ip:        remoteEndpointIP,
				})
				Expect(err).To(BeNil())
				newEndpointIP := response.GetIp()
				Expect(newEndpointIP).To(HavePrefix("10.80.")) // Local NAT ExternalCIDR is 10.80.0.0/24
				Eventually(func() bool {
					rules, err := listRulesInChainInCustomNs(fmt.Sprintf("LIQO-PRRT-MAP-CLS-%s", clusterID1))
					if err != nil {
						Fail(fmt.Sprintf("failed to list rules in chain %s: %s", fmt.Sprintf("LIQO-PRRT-MAP-CLS-%s", clusterID1), err))
					}
					if slice.ContainsString(rules, fmt.Sprintf("-d %s -j DNAT --to-destination %s", response.GetIp(), remoteEndpointIP)) {
						return true
					}
					return false
				}, timeout, interval).Should(BeTrue())
			})
		})
		Context("Ask to map more IPs of a remote endpoints", func() {
			It("DNAT rules should be inserted for each of them"+
				" by the natmappingoperator", func() {
				response, err := ipam.MapEndpointIP(ctx, &liqonetIpam.MapRequest{
					ClusterID: clusterID1,
					Ip:        remoteEndpointIP,
				})
				Expect(err).To(BeNil())
				newEndpointIP := response.GetIp()
				Expect(newEndpointIP).To(HavePrefix("10.80.")) // Local NAT ExternalCIDR is 10.80.0.0/24
				Eventually(func() bool {
					rules, err := listRulesInChainInCustomNs(fmt.Sprintf("LIQO-PRRT-MAP-CLS-%s", clusterID1))
					if err != nil {
						Fail(fmt.Sprintf("failed to list rules in chain %s: %s", fmt.Sprintf("LIQO-PRRT-MAP-CLS-%s", clusterID1), err))
					}
					if slice.ContainsString(rules, fmt.Sprintf("-d %s -j DNAT --to-destination %s", response.GetIp(), remoteEndpointIP)) {
						return true
					}
					return false
				}, timeout, interval).Should(BeTrue())
				response, err = ipam.MapEndpointIP(ctx, &liqonetIpam.MapRequest{
					ClusterID: clusterID1,
					Ip:        remoteEndpointIP2,
				})
				Expect(err).To(BeNil())
				newEndpointIP2 := response.GetIp()
				Expect(newEndpointIP2).To(HavePrefix("10.80.")) // Local NAT ExternalCIDR is 10.80.0.0/24
				Eventually(func() bool {
					rules, err := listRulesInChainInCustomNs(fmt.Sprintf("LIQO-PRRT-MAP-CLS-%s", clusterID1))
					if err != nil {
						Fail(fmt.Sprintf("failed to list rules in chain %s: %s", fmt.Sprintf("LIQO-PRRT-MAP-CLS-%s", clusterID1), err))
					}
					// Should contain both rules
					if slice.ContainsString(rules, fmt.Sprintf("-d %s -j DNAT --to-destination %s", newEndpointIP, remoteEndpointIP)) &&
						slice.ContainsString(rules, fmt.Sprintf("-d %s -j DNAT --to-destination %s", newEndpointIP2, remoteEndpointIP2)) {
						return true
					}
					return false
				}, timeout, interval).Should(BeTrue())
			})
		})
		Context("Map the same endpoint IP on more clusters", func() {
			It("DNAT rules should be inserted by the natmappingoperator", func() {
				response, err := ipam.MapEndpointIP(ctx, &liqonetIpam.MapRequest{
					ClusterID: clusterID1,
					Ip:        remoteEndpointIP,
				})
				Expect(err).To(BeNil())
				newEndpointIPCluster1 := response.GetIp()
				Expect(newEndpointIPCluster1).To(HavePrefix("10.80.")) // Local NAT ExternalCIDR is 10.80.0.0/24
				Eventually(func() bool {
					rules, err := listRulesInChainInCustomNs(fmt.Sprintf("LIQO-PRRT-MAP-CLS-%s", clusterID1))
					if err != nil {
						Fail(fmt.Sprintf("failed to list rules in chain %s: %s", fmt.Sprintf("LIQO-PRRT-MAP-CLS-%s", clusterID1), err))
					}
					if slice.ContainsString(rules, fmt.Sprintf("-d %s -j DNAT --to-destination %s", newEndpointIPCluster1, remoteEndpointIP)) {
						return true
					}
					return false
				}, timeout, interval).Should(BeTrue())
				response, err = ipam.MapEndpointIP(ctx, &liqonetIpam.MapRequest{
					ClusterID: clusterID2,
					Ip:        remoteEndpointIP,
				})
				Expect(err).To(BeNil())
				newEndpointIPCluster2 := response.GetIp()
				Expect(newEndpointIPCluster2).To(HavePrefix("10.80.")) // Local NAT ExternalCIDR is 10.80.0.0/24
				Eventually(func() bool {
					rules, err := listRulesInChainInCustomNs(fmt.Sprintf("LIQO-PRRT-MAP-CLS-%s", clusterID1))
					if err != nil {
						Fail(fmt.Sprintf("failed to list rules in chain %s: %s", fmt.Sprintf("LIQO-PRRT-MAP-CLS-%s", clusterID2), err))
					}
					if slice.ContainsString(rules, fmt.Sprintf("-d %s -j DNAT --to-destination %s", newEndpointIPCluster2, remoteEndpointIP)) {
						return true
					}
					return false
				}, timeout, interval).Should(BeTrue())
			})
		})
	})
	Describe("Terminate an Endpoint IP mapping", func() {
		Context("Ask to terminate to map the IP of a remote endpoint", func() {
			It("IPAM module should return no errors and a DNAT rule should be deleted"+
				" by the natmappingoperator", func() {
				response, err := ipam.MapEndpointIP(ctx, &liqonetIpam.MapRequest{
					ClusterID: clusterID1,
					Ip:        remoteEndpointIP,
				})
				Expect(err).To(BeNil())
				Expect(response.GetIp()).To(HavePrefix("10.80.")) // Local NAT ExternalCIDR is 10.80.0.0/24
				Eventually(func() bool {
					rules, err := listRulesInChainInCustomNs(fmt.Sprintf("LIQO-PRRT-MAP-CLS-%s", clusterID1))
					if err != nil {
						Fail(fmt.Sprintf("failed to list rules in chain %s: %s", fmt.Sprintf("LIQO-PRRT-MAP-CLS-%s", clusterID1), err))
					}
					if slice.ContainsString(rules, fmt.Sprintf("-d %s -j DNAT --to-destination %s", response.GetIp(), remoteEndpointIP)) {
						return true
					}
					return false
				}, timeout, interval).Should(BeTrue())
				_, err = ipam.UnmapEndpointIP(ctx, &liqonetIpam.UnmapRequest{
					ClusterID: clusterID1,
					Ip:        remoteEndpointIP,
				})
				Expect(err).To(BeNil())
				Eventually(func() bool {
					rules, err := listRulesInChainInCustomNs(fmt.Sprintf("LIQO-PRRT-MAP-CLS-%s", clusterID1))
					if err != nil {
						Fail(fmt.Sprintf("failed to list rules in chain %s: %s", fmt.Sprintf("LIQO-PRRT-MAP-CLS-%s", clusterID1), err))
					}
					if !slice.ContainsString(rules, fmt.Sprintf("-d %s -j DNAT --to-destination %s", response.GetIp(), remoteEndpointIP)) {
						return true
					}
					return false
				}, timeout, interval).Should(BeTrue())
			})
		})
		Context("Ask to terminate to map the IP of a remote endpoint while there are other still active", func() {
			It("DNAT rule should be deleted only for the requested endpoint"+
				" by the natmappingoperator", func() {
				// Map remoteEndpointIP and remoteEndpointIP2
				response, err := ipam.MapEndpointIP(ctx, &liqonetIpam.MapRequest{
					ClusterID: clusterID1,
					Ip:        remoteEndpointIP,
				})
				Expect(err).To(BeNil())
				newEndpointIP := response.GetIp()
				Expect(newEndpointIP).To(HavePrefix("10.80.")) // Local NAT ExternalCIDR is 10.80.0.0/24
				response, err = ipam.MapEndpointIP(ctx, &liqonetIpam.MapRequest{
					ClusterID: clusterID1,
					Ip:        remoteEndpointIP2,
				})
				Expect(err).To(BeNil())
				newEndpointIP2 := response.GetIp()
				Expect(newEndpointIP2).To(HavePrefix("10.80.")) // Local NAT ExternalCIDR is 10.80.0.0/24
				Eventually(func() bool {
					rules, err := listRulesInChainInCustomNs(fmt.Sprintf("LIQO-PRRT-MAP-CLS-%s", clusterID1))
					if err != nil {
						Fail(fmt.Sprintf("failed to list rules in chain %s: %s", fmt.Sprintf("LIQO-PRRT-MAP-CLS-%s", clusterID1), err))
					}
					// Should contain both rules
					if slice.ContainsString(rules, fmt.Sprintf("-d %s -j DNAT --to-destination %s", newEndpointIP, remoteEndpointIP)) &&
						slice.ContainsString(rules, fmt.Sprintf("-d %s -j DNAT --to-destination %s", newEndpointIP2, remoteEndpointIP2)) {
						return true
					}
					return false
				}, timeout, interval).Should(BeTrue())
				// Terminate only mapping of remoteEndpointIP
				_, err = ipam.UnmapEndpointIP(ctx, &liqonetIpam.UnmapRequest{
					ClusterID: clusterID1,
					Ip:        remoteEndpointIP,
				})
				Expect(err).To(BeNil())
				Eventually(func() bool {
					rules, err := listRulesInChainInCustomNs(fmt.Sprintf("LIQO-PRRT-MAP-CLS-%s", clusterID1))
					if err != nil {
						Fail(fmt.Sprintf("failed to list rules in chain %s: %s", fmt.Sprintf("LIQO-PRRT-MAP-CLS-%s", clusterID1), err))
					}
					// Rule for remoteEndpointIP2 should be still present
					if !slice.ContainsString(rules, fmt.Sprintf("-d %s -j DNAT --to-destination %s", newEndpointIP, remoteEndpointIP)) &&
						slice.ContainsString(rules, fmt.Sprintf("-d %s -j DNAT --to-destination %s", newEndpointIP2, remoteEndpointIP2)) {
						return true
					}
					return false
				}, timeout, interval).Should(BeTrue())
			})
		})
	})
})

func listRulesInChainInCustomNs(chain string) ([]string, error) {
	var rules []string
	var err error
	err = iptNetns.Do(func(nn ns.NetNS) error {
		rules, err = ipt.ListRulesInChain(chain)
		return err
	})
	if err != nil {
		return nil, err
	}
	return rules, nil
}
