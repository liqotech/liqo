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

package common

import "github.com/vishvananda/netlink"

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
