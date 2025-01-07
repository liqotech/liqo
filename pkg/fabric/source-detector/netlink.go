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

package sourcedetector

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
)

// GetSrcIPFromDstIP returns the source IP used to reach the given destination IP.
func GetSrcIPFromDstIP(dstIP string) (string, error) {
	dstIPNet := net.ParseIP(dstIP)
	if dstIPNet == nil {
		return "", fmt.Errorf("unable to parse dstIP %q", dstIP)
	}
	routes, err := netlink.RouteGet(dstIPNet)
	if err != nil {
		return "", fmt.Errorf("unable to get routes for dstIP %q: %w", dstIP, err)
	}
	switch len(routes) {
	case 0:
		return "", fmt.Errorf("no routes for dstIP %q", dstIP)
	case 1:
		if routes[0].Src == nil {
			return "", fmt.Errorf("no src for dstIP %q", dstIP)
		}
		return routes[0].Src.String(), nil
	default:
		return "", fmt.Errorf("multiple routes for dstIP %q", dstIP)
	}
}
