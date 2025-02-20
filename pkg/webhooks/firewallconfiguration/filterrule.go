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

package firewallconfiguration

import (
	"bytes"
	"fmt"
	"net"

	firewallapi "github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
	fwutils "github.com/liqotech/liqo/pkg/firewall/utils"
)

func checkFilterRules(rules []firewallapi.FilterRule) error {
	for i := range rules {
		if err := checkFilterRule(&rules[i]); err != nil {
			return err
		}
	}
	return nil
}

func checkFilterRule(r *firewallapi.FilterRule) error {
	if r.Name == nil {
		return fmt.Errorf("filterrule has nil name")
	}

	for _, match := range r.Match {
		if match.IP != nil {
			if err := checkFilterRuleIPValue(match.IP); err != nil {
				return fmt.Errorf("filterrule %s err: %w", *r.Name, err)
			}
		}
	}

	switch r.Action {
	case firewallapi.ActionTCPMssClamp:
		return checkFilterRuleTCPMssClamp(r)
	default:
		return nil
	}
}

func checkFilterRuleTCPMssClamp(r *firewallapi.FilterRule) error {
	for i := range r.Match {
		if r.Match[i].Proto.Value == firewallapi.L4ProtoTCP && r.Match[i].Op == firewallapi.MatchOperationEq {
			return nil
		}
	}
	return fmt.Errorf("tcp mss clamp rule should have a match for tcp protocol")
}

func checkFilterRuleIPValue(mIP *firewallapi.MatchIP) error {
	IPValueType, err := fwutils.GetIPValueType(&mIP.Value)
	if err != nil {
		return err
	}

	switch IPValueType {
	case firewallapi.IPValueTypeIP:
		if net.ParseIP(mIP.Value) == nil {
			return fmt.Errorf("invalid IP %s", mIP.Value)
		}
	case firewallapi.IPValueTypeSubnet:
		if _, _, err := net.ParseCIDR(mIP.Value); err != nil {
			return fmt.Errorf("invalid subnet %s", mIP.Value)
		}
	case firewallapi.IPValueTypeRange:
		if err := checkGranularRangeIP(mIP); err != nil {
			return fmt.Errorf("invalid range %s", mIP.Value)
		}
	default:
		return fmt.Errorf("invalid IP value type %s", IPValueType)
	}
	return nil
}

func checkGranularRangeIP(mIP *firewallapi.MatchIP) error {
	addr1, addr2, err := fwutils.GetIPValueRange(mIP.Value)
	if err != nil {
		return err
	}

	if addr1 == nil || addr2 == nil {
		return fmt.Errorf("missing one of the IPs in range: %s", mIP.Value)
	}

	if bytes.Compare(addr1, addr2) > 0 {
		return fmt.Errorf("invalid IP range (start > end): %s", mIP.Value)
	}

	return nil
}
