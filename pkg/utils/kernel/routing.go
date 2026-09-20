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

package kernel

import (
	"fmt"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// localRulePriority is the priority at which the default local routing rule is
// re-added by DeprioritizeLocalRule.
const localRulePriority = 10

// DeprioritizeLocalRule moves the default local routing rule from priority 0
// to priority 100, so that the priority-0 rules installed by Liqo (e.g., the
// per-geneve-interface rules routing the remote pod CIDR through the tunnel)
// are evaluated first.
//
// This is required when the local and remote clusters share the same pod CIDR:
// the gateway pod IP belongs to the remote pod CIDR, and the default
// "0:\tfrom all lookup local" rule would otherwise match fabric (geneve)
// traffic directed to a remote pod whose IP is equal to the gateway pod IP,
// delivering it locally instead of forwarding it through the tunnel.
//
// Local delivery is preserved: the local table is still consulted (at priority
// 100) for any packet that did not match a more specific priority-0 rule, so
// the gateway pod IP keeps working for traffic arriving from other interfaces
// (e.g., eth0, API server, health probes), and the geneve interface IPs keep
// being delivered locally.
func DeprioritizeLocalRule() error {
	// List all IPv4 rules pointing to the local table.
	existing, err := netlink.RuleListFiltered(netlink.FAMILY_V4, &netlink.Rule{
		Table: unix.RT_TABLE_LOCAL,
	}, netlink.RT_FILTER_TABLE)
	if err != nil {
		return fmt.Errorf("failed to list local routing rules: %w", err)
	}

	// Check if the deprioritized rule is already in place.
	for i := range existing {
		if existing[i].Priority == localRulePriority && existing[i].Src == nil && existing[i].Dst == nil {
			return nil
		}
	}

	// Add the local rule at the new (lower) priority first, so that local
	// delivery keeps working even if the deletion of the original rule fails.
	newRule := netlink.NewRule()
	newRule.Table = unix.RT_TABLE_LOCAL
	newRule.Priority = localRulePriority

	if err := netlink.RuleAdd(newRule); err != nil {
		return fmt.Errorf("failed to add local routing rule at priority %d: %w", localRulePriority, err)
	}

	// Delete the default local rule (priority 0, no source/destination match).
	for i := range existing {
		if existing[i].Priority == 0 && existing[i].Src == nil && existing[i].Dst == nil && existing[i].IifName == "" {
			if err := netlink.RuleDel(&existing[i]); err != nil {
				return fmt.Errorf("failed to delete default local routing rule: %w", err)
			}
			break
		}
	}

	return nil
}
