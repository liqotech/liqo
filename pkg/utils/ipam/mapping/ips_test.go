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

package mapping

import (
	"fmt"
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("RemapMask", func() {
	DescribeTable("IPv4 remapping",
		func(ipStr, cidrStr string, expectedStr string) {
			addr := net.ParseIP(ipStr)
			Expect(addr).NotTo(BeNil())

			_, mask, err := net.ParseCIDR(cidrStr)
			Expect(err).NotTo(HaveOccurred()) // Check no error occurs
			Expect(mask).NotTo(BeNil())

			expected := net.ParseIP(expectedStr).To4()
			Expect(expected).NotTo(BeNil())

			result := RemapMask(addr, *mask)
			Expect(result).To(Equal(expected))
		},
		Entry("IPv4 remapping", "192.168.1.20", "10.0.0.0/27", "10.0.0.20"),
		Entry("IPv4 remapping", "192.168.2.20", "10.0.0.0/27", "10.0.0.20"),
		Entry("IPv4 remapping", "192.168.1.50", "10.0.0.0/27", "10.0.0.18"),
		Entry("IPv4 remapping", "192.168.2.50", "10.0.0.0/27", "10.0.0.18"),
		Entry("IPv4 remapping", "192.168.1.1", "255.255.255.128/25", "255.255.255.129"),
		Entry("IPv4 remapping", "172.16.0.1", "255.255.255.128/25", "255.255.255.129"),
		Entry("IPv4 remapping", "192.168.1.200", "255.255.255.128/25", "255.255.255.200"),
		Entry("IPv4 remapping", "192.168.1.10", "255.255.255.0/24", "255.255.255.10"),
		Entry("IPv4 remapping", "192.168.1.100", "255.255.255.0/24", "255.255.255.100"),
		Entry("IPv4 remapping", "172.16.0.1", "255.255.240.0/20", "255.255.240.1"),
		Entry("IPv4 remapping", "172.16.1.1", "255.255.240.0/20", "255.255.241.1"),
		Entry("IPv4 remapping", "192.168.1.10", "255.4.224.0/19", "255.4.225.10"),
		Entry("IPv4 remapping", "192.168.2.10", "255.4.224.0/19", "255.4.226.10"),
		Entry("IPv4 remapping", "10.0.0.1", "255.255.192.0/18", "255.255.192.1"),
		Entry("IPv4 remapping", "10.255.1.1", "78.5.78.143/18", "78.5.65.1"),
		Entry("IPv4 remapping", "192.168.1.20", "192.168.0.0/16", "192.168.1.20"),
		Entry("IPv4 remapping", "192.168.2.20", "192.168.0.0/16", "192.168.2.20"),
		Entry("IPv4 remapping", "172.16.1.1", "172.16.0.0/12", "172.16.1.1"),
		Entry("IPv4 remapping", "172.16.2.1", "172.16.0.0/12", "172.16.2.1"),
		Entry("IPv4 remapping", "10.0.0.1", "255.192.0.0/10", "255.192.0.1"),
		Entry("IPv4 remapping", "10.1.0.1", "40.32.0.0/10", "40.1.0.1"),
		Entry("IPv4 remapping", "10.0.0.1", "255.0.0.0/8", "255.0.0.1"),
		Entry("IPv4 remapping", "10.0.0.50", "255.0.0.0/8", "255.0.0.50"),
		Entry("IPv4 remapping", "10.1.1.1", "20.0.0.0/8", "20.1.1.1"),
		Entry("IPv4 remapping", "10.2.1.1", "20.0.0.0/8", "20.2.1.1"),
		Entry("IPv4 remapping", "192.168.1.10", "240.0.0.0/4", "240.168.1.10"),
		Entry("IPv4 remapping", "192.168.2.10", "240.0.0.0/4", "240.168.2.10"),
		Entry("IPv4 remapping", "192.168.1.100", "0.0.0.0/0", "192.168.1.100"),
		Entry("IPv4 remapping", "10.0.0.1", "255.255.255.255/32", "255.255.255.255"),
		Entry("IPv4 remapping", "172.16.1.0", "172.16.0.0/16", "172.16.1.0"),
		Entry("IPv4 remapping", "172.16.1.255", "172.16.0.0/16", "172.16.1.255"),
		Entry("IPv4 remapping", "10.10.10.10", "192.168.1.0/24", "192.168.1.10"),
		Entry("IPv4 remapping", "192.168.1.255", "10.0.0.0/8", "10.168.1.255"),
		Entry("IPv4 remapping", "224.0.0.1", "192.168.1.0/24", "192.168.1.1"),
		Entry("IPv4 remapping", "127.0.0.1", "10.0.0.0/8", "10.0.0.1"),
		Entry("IPv4 remapping", "192.168.1.1", "10.0.0.0/30", "10.0.0.1"),
		Entry("IPv4 remapping", "192.168.1.2", "10.0.0.0/30", "10.0.0.2"),
		Entry("IPv4 remapping", "192.168.1.10", "10.0.0.0/29", "10.0.0.2"),
		Entry("IPv4 remapping", "172.16.5.10", "192.168.0.0/21", "192.168.5.10"),
		Entry("IPv4 remapping", "10.0.5.20", "172.16.0.0/13", "172.16.5.20"),
		Entry("IPv4 remapping", "192.168.1.100", "10.0.0.0/27", "10.0.0.4"),
		Entry("IPv4 remapping", "172.20.100.5", "192.168.0.0/17", "192.168.100.5"),
		Entry("IPv4 remapping", "192.168.3.50", "10.0.0.0/23", "10.0.1.50"),
		Entry("IPv4 remapping", "10.1.1.2", "192.168.1.0/30", "192.168.1.2"),
		Entry("IPv4 remapping", "192.168.5.33", "172.16.1.0/26", "172.16.1.33"),
	)

	DescribeTable("IPv6 address remapping",
		func(ipStr, cidrStr string, expectedStr string) {
			addr := net.ParseIP(ipStr)
			Expect(addr).NotTo(BeNil())

			_, mask, err := net.ParseCIDR(cidrStr)
			Expect(err).NotTo(HaveOccurred())
			Expect(mask).NotTo(BeNil())

			expected := net.ParseIP(expectedStr).To16()
			Expect(expected).NotTo(BeNil())

			result := RemapMask(addr, *mask)

			fmt.Printf("expected: %s\n", expected)
			fmt.Printf("result: %s\n", result)

			Expect(result).To(Equal(expected))
		},
		Entry("IPv6 remapping", "2001:db8::1", "2001:db8::/32", "2001:db8::1"),
		Entry("IPv6 remapping", "2001:db8:1::1", "2001:db8::/32", "2001:db8:1::1"),
		Entry("IPv6 remapping", "2001:db8::1", "2001:db8:1::/48", "2001:db8:1::1"),
		Entry("IPv6 remapping", "2001:db8:2::1", "2001:db8:1::/48", "2001:db8:1::1"),
		Entry("IPv6 remapping", "2001:db8::1", "2001:db8:1:2::/64", "2001:db8:1:2::1"),
		Entry("IPv6 remapping", "2001:db8:1::1", "2001:db8:1:2::/64", "2001:db8:1:2::1"),
		Entry("IPv6 remapping", "2001:db8::1", "2001:db8:1:2:3::/80", "2001:db8:1:2:3::1"),
		Entry("IPv6 remapping", "2001:db8:1::1", "2001:db8:1:2:3::/80", "2001:db8:1:2:3::1"),
		Entry("IPv6 remapping", "2001:db8::1", "2001:db8:1:2:3:4::/96", "2001:db8:1:2:3:4::1"),
		Entry("IPv6 remapping", "2001:db8:1::1", "2001:db8:1:2:3:4::/96", "2001:db8:1:2:3:4::1"),
		Entry("IPv6 remapping", "2001:db8::1", "2001:db8:1:2:3:4:5::/112", "2001:db8:1:2:3:4:5:1"),
		Entry("IPv6 remapping", "2001:db8:1::1", "2001:db8:1:2:3:4:5::/112", "2001:db8:1:2:3:4:5:1"),
		Entry("IPv6 remapping", "2001:db8:1::1", "2001:db8:1:2:3:4:5:6/128", "2001:db8:1:2:3:4:5:6"),
		Entry("IPv6 remapping", "2001:db8::1", "2001:db8::/16", "2001:db8::1"),
		Entry("IPv6 remapping", "2001:db8:abcd::1", "2001:db8::/56", "2001:db8::1"),
		Entry("IPv6 remapping", "2001:db8:abcd:1234::1", "2001:db8::/64", "2001:db8::1"),
		Entry("IPv6 remapping", "fc00::1", "fd00::/8", "fd00::1"),
		Entry("IPv6 remapping", "fd00:1234::1", "fc00::/7", "fd00:1234::1"),
		Entry("IPv6 remapping", "fe80::1", "fd00::/8", "fd80::1"),
		Entry("IPv6 remapping", "2001:db8:abcd::1", "2001:db8:1234::/32", "2001:db8:abcd::1"),
		Entry("IPv6 remapping", "2001::1", "2002::/16", "2002::1"),
		Entry("IPv6 remapping", "2001:db8:abcd::1", "2001:db8::/49", "2001:db8::1"),
		Entry("IPv6 remapping", "2001:db8:abcd:1234::1", "2001:db8::/57", "2001:db8:0:34::1"),
		Entry("IPv6 remapping", "2001:db8:abcd:1234::1", "2001:db8::/69", "2001:db8::1"),
		Entry("IPv6 remapping", "2001:db8:abcd:1234::1", "2001:db8::/73", "2001:db8::1"),
		Entry("IPv6 remapping", "2001:db8::1", "2001:db8::/127", "2001:db8::1"),
		Entry("IPv6 remapping", "2001:db8:abcd::1", "2001:db8::/37", "2001:db8:3cd::1"),
		Entry("IPv6 remapping", "2001:fdb8:abcd::45a3:1", "2001:d2f::/53", "2001:d2f::45a3:1"),
		Entry("IPv6 remapping", "2001:db8:abcd:1234::1", "2001:db8::/61", "2001:db8:0:4::1"),
	)
})
