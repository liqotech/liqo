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

package tunnel

import (
	"fmt"

	"github.com/vishvananda/netlink"

	"github.com/liqotech/liqo/pkg/gateway"
)

const (
	// wireguardNetworkBase is the base prefix of the /24 subnet reserved for Wireguard interfaces (169.254.18.0/24).
	// Each tunnel is assigned a /30 block within this subnet.
	wireguardNetworkBase = "169.254.18"
	// serverOctetOffset is the offset of the server IP within each /30 block.
	serverOctetOffset = 1
	// clientOctetOffset is the offset of the client IP within each /30 block.
	clientOctetOffset = 2
	// subnetSize is the number of addresses in each /30 block, used to compute per-tunnel IP offsets.
	subnetSize = 4
	// maxWireguardInterfaces is the maximum number of Wireguard interfaces that can be created,
	// derived from the available /30 blocks in the 169.254.18.0/24 subnet.
	MaxWireguardInterfaces = 64
	// ServerInterfaceIP (169.254.18.1/30) is the IP address of the Wireguard interface
	// in server mode for tunnel index 0.
	ServerInterfaceIP = wireguardNetworkBase + ".1/30"
	// ClientInterfaceIP (169.254.18.2/30) is the IP address of the Wireguard interface
	// in client mode for tunnel index 0.
	ClientInterfaceIP = wireguardNetworkBase + ".2/30"
)

// AddAddress adds an IP address to the Wireguard interface.
func AddAddress(link netlink.Link, ip string) error {
	addr, err := netlink.ParseAddr(ip)
	if err != nil {
		return err
	}

	return netlink.AddrAdd(link, addr)
}

// GetLink returns the Wireguard interface.
func GetLink(name string) (netlink.Link, error) {
	return netlink.LinkByName(name)
}

// GetInterfaceIP returns the IP address of the Wireguard interface.
func GetInterfaceIP(mode gateway.Mode, idx int) string {
	var fourthOctet int

	switch mode {
	case gateway.ModeServer:
		fourthOctet = (subnetSize * idx) + serverOctetOffset

	case gateway.ModeClient:
		fourthOctet = (subnetSize * idx) + clientOctetOffset

	default:
		return ""
	}

	return fmt.Sprintf("%s.%d/30", wireguardNetworkBase, fourthOctet)
}

func GetTunnelName(idx int) string {
	if idx == 0 {
		return TunnelInterfaceName
	}
	return fmt.Sprintf("%s%d", TunnelInterfaceName, idx)
}

// GetRemoteInterfaceIP returns the IP address of the remote Wireguard interface.
func GetRemoteInterfaceIP(mode gateway.Mode) (string, error) {
	switch mode {
	case gateway.ModeServer:
		ip, err := netlink.ParseIPNet(ClientInterfaceIP)
		return ip.IP.String(), err
	case gateway.ModeClient:
		ip, err := netlink.ParseIPNet(ServerInterfaceIP)
		return ip.IP.String(), err
	}
	return "", fmt.Errorf("invalid mode %v", mode)
}
