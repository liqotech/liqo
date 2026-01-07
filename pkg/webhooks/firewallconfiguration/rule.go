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

	"github.com/google/uuid"

	firewallapi "github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
	"github.com/liqotech/liqo/pkg/firewall"
	firewallutils "github.com/liqotech/liqo/pkg/firewall/utils"
)

func checkRulesInChain(chain *firewallapi.Chain) error {
	rules := firewall.FromChainToRulesArray(chain)
	if err := checkVoidRuleName(rules); err != nil {
		return forgeChainError(chain, err)
	}
	if err := checkUniqueRuleNames(rules); err != nil {
		return forgeChainError(chain, err)
	}
	if err := checkFilterRules(rules); err != nil {
		return forgeChainError(chain, err)
	}
	return nil
}

func checkVoidRuleName(rules []firewallutils.Rule) error {
	for i := range rules {
		name := rules[i].GetName()
		if name == nil || *name == "" {
			return fmt.Errorf("rule name is void")
		}
	}
	return nil
}

func checkUniqueRuleNames(rules []firewallutils.Rule) error {
	names := map[string]interface{}{}
	for i := range rules {
		name := rules[i].GetName()
		if name == nil {
			return fmt.Errorf("rule name is nil")
		}
		if _, ok := names[*name]; ok {
			return fmt.Errorf("name %v is duplicated", *name)
		}
		names[*name] = nil
	}
	return nil
}

func checkFilterRules(rules []firewallutils.Rule) error {
	for i := range rules {
		if r, ok := rules[i].(*firewallutils.FilterRuleWrapper); ok {
			if err := checkFilterRule(r.FilterRule); err != nil {
				return fmt.Errorf("rule %v is invalid: %w", r.Name, err)
			}
		}
	}
	return nil
}

func checkFilterRule(r *firewallapi.FilterRule) error {
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

func generateRuleNames(chains []firewallapi.Chain) {
	for i := range chains {
		rules := firewall.FromChainToRulesArray(&chains[i])
		for j := range rules {
			if rules[j].GetName() == nil || *rules[j].GetName() == "" {
				rules[j].SetName(uuid.NewString())
			}
		}
	}
}
