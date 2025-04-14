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
	for _, matchIP := range filterRule.Match{
		if err := checkFilterRuleActions(filterRule); err != nil {
			return err;
		}
		if err := checkFilterRuleIPValue(filterRule, &matchIP); err != nil {
			return err
		}
		if err := checkFilterRulePosition(filterRule, &matchIP); err != nil {
			return err
		}
	}

	return nil
}

func checkFilterRuleActions(filterRule *firewallapi.FilterRule) error{
	switch filterRule.Action {
	case firewallapi.ActionAccept, firewallapi.ActionDrop, firewallapi.ActionReject:
		return nil
	default:
		return fmt.Errorf("filterrule %s has an invalid action", *filterRule.Name)
	}
}

func checkFilterRuleIPValue(filterRule *firewallapi.FilterRule, m *firewallapi.Match) error {
	if m.IP == nil {
		return fmt.Errorf("filterrule %s has no IP match value", *filterRule.Name)
	}
	if m.IP.Value == "" {
		return fmt.Errorf("filterrule %s has no value in IP match", *filterRule.Name)
	}else{
		_, err := commonUtils.GetIPValueTypeRange(m.IP.Value)
		if err != nil {
			return fmt.Errorf("%s", err)
		}
	}
	return nil
}

func checkFilterRulePosition(filterRule *firewallapi.FilterRule, m *firewallapi.Match) error {
	switch m.IP.Position {
	case firewallapi.MatchPositionSrc:
		return nil
	case firewallapi.MatchPositionDst:
		return nil
	}
	return fmt.Errorf("invalid match IP position %s for filterRule: %s", m.IP.Position, *filterRule.Name)
}




