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
)

func checkNatRulesInChain(chain *firewallapi.Chain) error {
	natrules := chain.Rules.NatRules
	for i := range natrules {
		if err := checkNatRuleChainHook(*chain.Hook, &natrules[i]); err != nil {
			return err
		}
		if err := checkNatRuleTo(&natrules[i]); err != nil {
			return err
		}
	}
	return nil
}

func checkNatRuleTo(natrule *firewallapi.NatRule) error {
	switch natrule.NatType {
	case firewallapi.NatTypeDestination, firewallapi.NatTypeSource:
		if natrule.To == nil {
			return fmt.Errorf("natrule %s is %s but has no To field", *natrule.Name, natrule.NatType)
		}
	case firewallapi.NatTypeMasquerade:
		if natrule.To != nil {
			return fmt.Errorf("natrule %s is Masquerade or masquerade but has a To field", *natrule.Name)
		}
	}
	return nil
}

func checkNatRuleChainHook(hook firewallapi.ChainHook, rule *firewallapi.NatRule) error {
	switch hook {
	case firewallapi.ChainHookPostrouting:
		switch rule.NatType {
		case firewallapi.NatTypeDestination:
			return fmt.Errorf("natrule %s is DNAT that is incompatible with postrouting", *rule.Name)
		case firewallapi.NatTypeSource, firewallapi.NatTypeMasquerade:
			return nil
		}
	case firewallapi.ChainHookPrerouting:
		switch rule.NatType {
		case firewallapi.NatTypeDestination:
			return nil
		case firewallapi.NatTypeSource, firewallapi.NatTypeMasquerade:
			return fmt.Errorf("natrule %s is SNAT that is incompatible with prerouting", *rule.Name)
		}
	case firewallapi.ChainHookInput:
		switch rule.NatType {
		case firewallapi.NatTypeDestination:
			return fmt.Errorf("natrule %s is DNAT that is incompatible with input", *rule.Name)
		case firewallapi.NatTypeSource, firewallapi.NatTypeMasquerade:
			return nil
		}
	case firewallapi.ChainHookOutput:
		switch rule.NatType {
		case firewallapi.NatTypeDestination:
			return nil
		case firewallapi.NatTypeSource, firewallapi.NatTypeMasquerade:
			return fmt.Errorf("natrule %s is SNAT that is incompatible with output", *rule.Name)
		}
	default:
		return fmt.Errorf("hook %s is not a valid hook", hook)
	}
	return nil
}
