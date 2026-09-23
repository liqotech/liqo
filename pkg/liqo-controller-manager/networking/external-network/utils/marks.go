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

package utils

// GwExtMark and GwNodeMark are defined here (rather than in package route, where they are
// conceptually owned) because package remapping also needs to reference them and importing
// route from remapping would create an import cycle (route already imports remapping for
// ForgeFirewallTargetLabels). Package utils sits below both and has no dependency on either,
// so it is used as the shared leaf package.
const (
	// GwExtMark is the fwmark value used to tag traffic arriving on Geneve interfaces (liqo.*).
	// It allows the gw-ext RouteConfiguration to match on FwMark + Dst instead of Iif + Dst,
	// collapsing N*R rules to R rules while still preventing routing loops (packets arriving
	// on the WireGuard interface liqo-tunnel are not marked and do not match).
	// The mark is set per-packet in the prerouting chain (not via conntrack) so it is available
	// for route lookup and does not leak to return traffic.
	//
	// The value 0xFF00 is chosen to avoid collision with the internal-network mark allocator
	// (pkg/liqo-controller-manager/networking/internal-network/route/mark.go), which assigns
	// sequential marks starting from 1, one per node. A high value ensures no overlap even in
	// very large clusters.
	GwExtMark = 0xFF00

	// GwNodeMark is the fwmark value used to tag traffic arriving on WireGuard tunnel interfaces (liqo-tunnel*).
	GwNodeMark = 0xFE00
)
