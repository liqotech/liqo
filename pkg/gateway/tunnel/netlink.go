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

package tunnel

import (
	"fmt"

	"github.com/vishvananda/netlink"

	"github.com/liqotech/liqo/pkg/gateway"
)

const (
	// ServerInterfaceIP is the IP address of the Wireguard interface in server mode.
	ServerInterfaceIP = "169.254.18.1/30"
	// ClientInterfaceIP is the IP address of the Wireguard interface in client mode.
	ClientInterfaceIP = "169.254.18.2/30"
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
func GetInterfaceIP(mode gateway.Mode) string {
	switch mode {
	case gateway.ModeServer:
		return ServerInterfaceIP
	case gateway.ModeClient:
		return ClientInterfaceIP
	}
	return ""
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
