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

package tunnel_test

import (
	"net"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/liqotech/liqo/pkg/gateway"
	"github.com/liqotech/liqo/pkg/gateway/tunnel"
)

// stripMask removes the "/30" suffix from an address in CIDR notation.
func stripMask(cidr string) string {
	return strings.TrimSuffix(cidr, "/30")
}

var _ = Describe("GetRemoteInterfaceIP", func() {

	It("should match the legacy constants for the tunnel index 0", func() {
		// The remote of a server is the client, and vice versa.
		Expect(tunnel.GetRemoteInterfaceIP(gateway.ModeServer, 0)).To(Equal(stripMask(tunnel.ClientInterfaceIP)))
		Expect(tunnel.GetRemoteInterfaceIP(gateway.ModeClient, 0)).To(Equal(stripMask(tunnel.ServerInterfaceIP)))
	})

	DescribeTable("should return the expected address", func(mode gateway.Mode, idx int, want string) {
		Expect(tunnel.GetRemoteInterfaceIP(mode, idx)).To(Equal(want))
	},
		Entry("server mode, idx 1", gateway.ModeServer, 1, "169.254.18.6"),
		Entry("server mode, idx 2", gateway.ModeServer, 2, "169.254.18.10"),
		Entry("client mode, idx 1", gateway.ModeClient, 1, "169.254.18.5"),
		Entry("client mode, idx 2", gateway.ModeClient, 2, "169.254.18.9"),
		Entry("server mode, last index", gateway.ModeServer, tunnel.MaxWireguardInterfaces-1, "169.254.18.254"),
		Entry("client mode, last index", gateway.ModeClient, tunnel.MaxWireguardInterfaces-1, "169.254.18.253"),
	)

	DescribeTable("should be the local address of the other side", func(mode, otherMode gateway.Mode) {
		for idx := 0; idx < tunnel.MaxWireguardInterfaces; idx++ {
			Expect(tunnel.GetRemoteInterfaceIP(mode, idx)).To(
				Equal(stripMask(tunnel.GetInterfaceIP(otherMode, idx))),
				"Mismatch at index %d", idx)
		}
	},
		Entry("server's remote is the client's local", gateway.ModeServer, gateway.ModeClient),
		Entry("client's remote is the server's local", gateway.ModeClient, gateway.ModeServer),
	)

	DescribeTable("should return distinct valid addresses inside the reserved subnet", func(mode gateway.Mode) {
		_, subnet, err := net.ParseCIDR("169.254.18.0/24")
		Expect(err).NotTo(HaveOccurred())

		seen := map[string]int{}
		for idx := 0; idx < tunnel.MaxWireguardInterfaces; idx++ {
			addr := tunnel.GetRemoteInterfaceIP(mode, idx)
			ip := net.ParseIP(addr)

			Expect(ip).NotTo(BeNil(), "Invalid address %q at index %d", addr, idx)
			Expect(subnet.Contains(ip)).To(BeTrue(), "Address %s at index %d is outside %s", addr, idx, subnet)

			prev, dup := seen[addr]
			Expect(dup).To(BeFalse(), "Address %s returned for both index %d and %d", addr, prev, idx)
			seen[addr] = idx
		}
	},
		Entry("server mode", gateway.ModeServer),
		Entry("client mode", gateway.ModeClient),
	)

	It("should return different addresses for server and client with the same index", func() {
		for idx := 0; idx < tunnel.MaxWireguardInterfaces; idx++ {
			Expect(tunnel.GetRemoteInterfaceIP(gateway.ModeServer, idx)).
				NotTo(Equal(tunnel.GetRemoteInterfaceIP(gateway.ModeClient, idx)), "Same address at index %d", idx)
		}
	})

	It("should return an empty string with an invalid mode", func() {
		Expect(tunnel.GetRemoteInterfaceIP(gateway.Mode("invalid"), 0)).To(BeEmpty())
		Expect(tunnel.GetRemoteInterfaceIP(gateway.Mode(""), 3)).To(BeEmpty())
	})
})

func TestTunnel(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Tunnel test suite")
}
