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

	"k8s.io/klog/v2"

	firewallapi "github.com/liqotech/liqo/apis/networking/v1alpha1/firewall"
)

func checkChain(tableFamily firewallapi.TableFamily, chain *firewallapi.Chain) error {
	if err := checkAllowedTableFamilyChainTypeHook(tableFamily, chain); err != nil {
		return forgeChainError(chain, err)
	}

	if err := checkTotalDefinedRulesSets(chain.Rules); err != nil {
		return forgeChainError(chain, err)
	}

	if err := allowedChainType(chain.Type, chain.Rules); err != nil {
		return forgeChainError(chain, err)
	}

	return nil
}

func checkAllowedTableFamilyChainTypeHook(tableFamily firewallapi.TableFamily, chain *firewallapi.Chain) error {
	if !allowedTableFamilyChainTypeHook(tableFamily, *chain.Type, *chain.Hook) {
		return fmt.Errorf(`in chain %s, the combination of family %s, chain type %s and hook %s is not allowed. 
		Please refer to https://wiki.nftables.org/wiki-nftables/index.php/Netfilter_hooks`,
			*chain.Name, tableFamily, *chain.Type, *chain.Hook,
		)
	}
	return nil
}

func checkTotalDefinedRulesSets(rules firewallapi.RulesSet) error {
	total := totalDefinedRulesSets(rules)
	if total == 0 {
		return fmt.Errorf("no rules set defined")
	}
	if total > 1 {
		return fmt.Errorf("only one rules set must be defined")
	}
	return nil
}

func totalDefinedRulesSets(rules firewallapi.RulesSet) int {
	total := 0
	if rules.NatRules != nil {
		total++
	}
	if rules.FilterRules != nil {
		total++
	}
	if rules.RouteRules != nil {
		total++
	}
	return total
}

func allowedChainType(chaintype *firewallapi.ChainType, rules firewallapi.RulesSet) error {
	switch *chaintype {
	case firewallapi.ChainTypeNAT:
		if rules.NatRules == nil {
			return fmt.Errorf("NAT rules must be defined when using NAT chain. Please fulfill the rules.natRules field")
		}
	case firewallapi.ChainTypeFilter:
		if rules.FilterRules == nil {
			return fmt.Errorf("filter rules must be defined when using Filter chain. Please fulfill the rules.filterRules field")
		}
	case firewallapi.ChainTypeRoute:
		if rules.RouteRules == nil {
			return fmt.Errorf("route rules must be defined when using Route chain. Please fulfill the rules.routeRules field")
		}
	default:
		return fmt.Errorf("chain type not supported")
	}
	return nil
}

// refer to https://wiki.nftables.org/wiki-nftables/index.php/Netfilter_hooks
func allowedTableFamilyChainTypeHook(familiy firewallapi.TableFamily, chainType firewallapi.ChainType, hook firewallapi.ChainHook) bool {
	switch familiy {
	case firewallapi.TableFamilyINet, firewallapi.TableFamilyIPv4, firewallapi.TableFamilyIPv6:
		switch chainType {
		case firewallapi.ChainTypeFilter:
			switch hook {
			case firewallapi.ChainHookIngress:
				if familiy == firewallapi.TableFamilyINet {
					return true
				}
				return false
			default:
				return true
			}
		case firewallapi.ChainTypeNAT:
			switch hook {
			case firewallapi.ChainHookPrerouting:
				return true
			case firewallapi.ChainHookInput:
				return true
			case firewallapi.ChainHookOutput:
				return true
			case firewallapi.ChainHookPostrouting:
				return true
			default:
				return false
			}
		case firewallapi.ChainTypeRoute:
			switch hook {
			case firewallapi.ChainHookOutput:
				return true
			default:
				return false
			}
		default:
			klog.Warningf("unknown chain type %v", chainType)
			return false
		}
	case firewallapi.TableFamilyARP:
		switch chainType {
		case firewallapi.ChainTypeFilter:
			switch hook {
			case firewallapi.ChainHookInput:
				return true
			case firewallapi.ChainHookOutput:
				return true
			default:
				return false
			}
		case firewallapi.ChainTypeNAT:
			return false
		case firewallapi.ChainTypeRoute:
			return false
		default:
			klog.Warningf("unknown chain type %v", chainType)
			return false
		}
	case firewallapi.TableFamilyBridge:
		switch chainType {
		case firewallapi.ChainTypeFilter:
			switch hook {
			case firewallapi.ChainHookIngress:
				return false
			default:
				return true
			}
		case firewallapi.ChainTypeNAT:
			return false
		case firewallapi.ChainTypeRoute:
			return false
		default:
			klog.Warningf("unknown chain type %v", chainType)
			return false
		}
	case firewallapi.TableFamilyNetdev:
		switch chainType {
		case firewallapi.ChainTypeFilter:
			switch hook {
			case firewallapi.ChainHookIngress:
				return true
			default:
				return false
			}
		case firewallapi.ChainTypeNAT:
			return false
		case firewallapi.ChainTypeRoute:
			return false
		default:
			klog.Warningf("unknown chain type %v", chainType)
			return false
		}
	default:
		klog.Warningf("unknown table family %v", familiy)
		return false
	}
}
