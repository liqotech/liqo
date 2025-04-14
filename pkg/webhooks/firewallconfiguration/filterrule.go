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

package firewallconfiguration

import (
	"fmt"
	"net"

	firewallapi "github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
	commonUtils "github.com/liqotech/liqo/pkg/firewall/utils"
)

func checkFilterRulesInChain(chain *firewallapi.Chain) error {
	filterRules := chain.Rules.FilterRules
	for i := range filterRules {
		if err := checkFilterRule(&filterRules[i]); err != nil {
			return err
		}
	}
	return nil
}

func checkFilterRule(filterRule *firewallapi.FilterRule) error {
	for _, match := range filterRule.Match{
		if match.IP != nil {
			if err := checkFilterRuleActions(&filterRule.Action); err != nil {
				return fmt.Errorf("filterrule %s err: %s", *filterRule.Name, err)
			}
			if err := checkFilterRuleIPValue(match.IP); err != nil {
				return fmt.Errorf("filterrule %s err: %s", *filterRule.Name, err)
			}
			if err := checkFilterRulePosition(&match.IP.Position); err != nil {
				return fmt.Errorf("filterrule %s err: %s", *filterRule.Name, err)
			}
			if err := checkFilterRuleOperation(&match.Op); err != nil {
				return fmt.Errorf("filterrule %s err: %s", *filterRule.Name, err)
			}
		}
	}

	return nil
}

func checkFilterRuleActions(action *firewallapi.FilterAction) error{
	switch *action {
	case firewallapi.ActionAccept, firewallapi.ActionDrop, firewallapi.ActionReject:
		return nil
	default:
		return fmt.Errorf("Invalid action %s", action)
	}
}

func checkFilterRuleIPValue(mIP *firewallapi.MatchIP) error {
	IPValueType, err := commonUtils.GetIPValueType(&mIP.Value)
	if err != nil {
		return err
	}
	if IPValueType == firewallapi.IPValueTypeRange {
		if err := checkGranularRangeIP(mIP); err != nil {
			return err
		}
	}
	if IPValueType == firewallapi.IPValueTypeSubnet {
		_, _, err := net.ParseCIDR(mIP.Value)
		if err != nil {
			return err
		}
	}
	if IPValueType == firewallapi.IPValueTypeIP {
		if net.ParseIP(mIP.Value) == nil {
			return err
		}
	}
	return nil
}

func checkFilterRulePosition(position *firewallapi.MatchPosition) error {
	switch *position {
	case firewallapi.MatchPositionSrc:
		return nil
	case firewallapi.MatchPositionDst:
		return nil
	}
	return fmt.Errorf("invalid match IP position %s", position)
}

func checkFilterRuleOperation(op *firewallapi.MatchOperation) error {
	switch *op {
	case firewallapi.MatchOperationEq:
		return nil
	case firewallapi.MatchOperationNeq:
		return nil
	}
	return fmt.Errorf("invalid match IP operation %s", op)
}

func checkGranularRangeIP(mIP *firewallapi.MatchIP) error {
	_, err := commonUtils.GetIPValueTypeRange(mIP.Value)
	if err != nil {
		return err
	}
	
	// cannot return error since is also called inside GetIPValueTypeRange
	addr1, addr2, _ := commonUtils.GetIPValueRange(mIP.Value)
	mask := net.CIDRMask(24, 32)
	network := addr1.Mask(mask)

	subnet := net.IPNet{
		IP:   network,
		Mask: mask,
	}

	// check if the two IPs are in the same /24 subnet
	if !subnet.Contains(addr2) {
		return fmt.Errorf("IP range %s - %s spans different /24 subnets", addr1, addr2)
	}
	return nil
}