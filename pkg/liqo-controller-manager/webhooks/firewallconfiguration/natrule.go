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

package firewallconfiguration

import (
	"fmt"

	firewallapi "github.com/liqotech/liqo/apis/networking/v1alpha1/firewall"
)

func checkNatRulesInChain(chain *firewallapi.Chain) error {
	natrules := chain.Rules.NatRules
	if natrules == nil {
		return fmt.Errorf("natrules is nil")
	}
	for i := range natrules {
		if err := checkNatRuleChainHook(*chain.Hook, &natrules[i]); err != nil {
			return err
		}
	}
	return nil
}

func checkNatRuleChainHook(hook firewallapi.ChainHook, rule *firewallapi.NatRule) error {
	switch hook {
	case firewallapi.ChainHookPostrouting:
		switch rule.NatType {
		case firewallapi.NatTypeDestination:
			return fmt.Errorf("natrule %s is DNAT that is incompatible with postrouting", *rule.GetName())
		case firewallapi.NatTypeSource:
			return nil
		}
	case firewallapi.ChainHookPrerouting:
		switch rule.NatType {
		case firewallapi.NatTypeDestination:
			return nil
		case firewallapi.NatTypeSource:
			return fmt.Errorf("natrule %s is SNAT that is incompatible with prerouting", *rule.GetName())
		}
	case firewallapi.ChainHookInput:
		switch rule.NatType {
		case firewallapi.NatTypeDestination:
			return fmt.Errorf("natrule %s is DNAT that is incompatible with input", *rule.GetName())
		case firewallapi.NatTypeSource:
			return nil
		}
	case firewallapi.ChainHookOutput:
		switch rule.NatType {
		case firewallapi.NatTypeDestination:
			return nil
		case firewallapi.NatTypeSource:
			return fmt.Errorf("natrule %s is SNAT that is incompatible with output", *rule.GetName())
		}
	default:
		return fmt.Errorf("hook %s is not a valid hook", hook)
	}
	return nil
}
