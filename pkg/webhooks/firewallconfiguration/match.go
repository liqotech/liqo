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
	"bytes"
	"fmt"
	"net"

	firewallapi "github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
	fwutils "github.com/liqotech/liqo/pkg/firewall/utils"
)

func checkRuleMatch(matchRule *firewallapi.Match, sets []firewallapi.Set) error {
	if matchRule.IP != nil {
		if err := checkMatchIPValue(matchRule.IP, sets); err != nil {
			return err
		}
	}
	return nil
}

func checkMatchIPValue(mIP *firewallapi.MatchIP, sets []firewallapi.Set) error {
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
	case firewallapi.IPValueTypeNamedSet:
		matchedSet := findMatchingSet(mIP, sets)
		if matchedSet == nil {
			return fmt.Errorf("named set %s does not exist", mIP.Value)
		}

		if matchedSet.KeyType != firewallapi.SetDataTypeIPAddr {
			return fmt.Errorf("named set %s is not of type IP", mIP.Value)
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

func findMatchingSet(mIP *firewallapi.MatchIP, sets []firewallapi.Set) *firewallapi.Set {
	setName, err := fwutils.GetIPValueNamedSet(mIP.Value)
	if err != nil {
		return nil
	}

	for _, set := range sets {
		if set.Name == setName {
			return &set
		}
	}
	return nil
}
