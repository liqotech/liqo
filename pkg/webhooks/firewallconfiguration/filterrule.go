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
	"fmt"

	firewallapi "github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
)

func checkFilterRulesInChain(chain *firewallapi.Chain, sets []firewallapi.Set) error {
	filterRules := chain.Rules.FilterRules
	for i := range filterRules {
		if err := checkFilterRule(&filterRules[i], sets); err != nil {
			return err
		}
	}
	return nil
}

func checkFilterRule(filterRule *firewallapi.FilterRule, sets []firewallapi.Set) error {
	if filterRule.Name == nil {
		return fmt.Errorf("filterrule has nil name")
	}
	for _, match := range filterRule.Match {
		if err := checkRuleMatch(&match, sets); err != nil {
			return fmt.Errorf("filterrule %s: %v", *filterRule.Name, err)
		}
	}

	switch filterRule.Action {
	case firewallapi.ActionTCPMssClamp:
		return checkFilterRuleTCPMssClamp(filterRule)
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
