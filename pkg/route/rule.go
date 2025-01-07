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

package route

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
)

// EnsureRulePresence ensures the presence of the given rule.
func EnsureRulePresence(rule *networkingv1beta1.Rule, tableID uint32) error {
	rules, err := GetRulesByTableID(tableID)
	if err != nil {
		return err
	}

	_, exists, err := ExistsRule(rule, rules)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	return AddRule(rule, tableID)
}

// EnsureRuleAbsence ensures the absence of the given rule.
func EnsureRuleAbsence(rule *networkingv1beta1.Rule, tableID uint32) error {
	rules, err := GetRulesByTableID(tableID)
	if err != nil {
		return err
	}

	existingrule, exists, err := ExistsRule(rule, rules)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	return netlink.RuleDel(existingrule)
}

// AddRule adds the given rule to the rules list.
func AddRule(rule *networkingv1beta1.Rule, tableID uint32) error {
	newrule := netlink.NewRule()
	newrule.Table = int(tableID)

	if rule.Src != nil {
		_, srcnet, err := net.ParseCIDR(rule.Src.String())
		if err != nil {
			return err
		}
		newrule.Src = srcnet
	}

	if rule.Dst != nil {
		_, dstnet, err := net.ParseCIDR(rule.Dst.String())
		if err != nil {
			return err
		}
		newrule.Dst = dstnet
	}

	if rule.Iif != nil {
		newrule.IifName = *rule.Iif
	}

	if rule.Oif != nil {
		newrule.OifName = *rule.Oif
	}

	if rule.FwMark != nil {
		newrule.Mark = *rule.FwMark
	}

	err := netlink.RuleAdd(newrule)
	if err != nil {
		return fmt.Errorf("unable to add rule %v: %w", rule, err)
	}
	return nil
}

// GetRulesByTableID returns all the rules associated with the given table ID.
func GetRulesByTableID(tableID uint32) ([]netlink.Rule, error) {
	rulelist, err := netlink.RuleListFiltered(netlink.FAMILY_ALL, &netlink.Rule{
		Table: int(tableID),
	}, netlink.RT_FILTER_TABLE)
	if err != nil {
		return nil, err
	}
	var rules []netlink.Rule
	for i := range rulelist {
		if rulelist[i].Table == int(tableID) {
			rules = append(rules, rulelist[i])
		}
	}
	return rules, nil
}

// ExistsRule checks if the given rule is already present in the rules list.
func ExistsRule(rule *networkingv1beta1.Rule, rules []netlink.Rule) (*netlink.Rule, bool, error) {
	for i := range rules {
		if RuleIsEqual(rule, &rules[i]) {
			return &rules[i], true, nil
		}
	}
	return nil, false, nil
}

// RuleIsEqual checks if the given rule is equal to the given netlink rule.
func RuleIsEqual(rule *networkingv1beta1.Rule, netlinkRule *netlink.Rule) bool {
	if rule == nil || netlinkRule == nil {
		return false
	}

	if rule.Src != nil && netlinkRule.Src != nil && rule.Src.String() != netlinkRule.Src.String() {
		return false
	}
	if rule.Src == nil && netlinkRule.Src != nil {
		return false
	}

	if rule.Dst != nil && netlinkRule.Dst != nil && rule.Dst.String() != netlinkRule.Dst.String() {
		return false
	}
	if rule.Dst == nil && netlinkRule.Dst != nil {
		return false
	}

	if rule.Iif != nil && *rule.Iif != netlinkRule.IifName {
		return false
	}
	if rule.Iif == nil && netlinkRule.IifName != "" {
		return false
	}

	if rule.Oif != nil && *rule.Oif != netlinkRule.OifName {
		return false
	}
	if rule.Oif == nil && netlinkRule.OifName != "" {
		return false
	}

	if rule.FwMark != nil && *rule.FwMark != netlinkRule.Mark {
		return false
	}
	return true
}

// CleanRules deletes all the rules which are not contained anymore in the given rules list.
func CleanRules(rules []networkingv1beta1.Rule, tableID uint32) error {
	existingrules, err := GetRulesByTableID(tableID)
	if err != nil {
		return err
	}
	for i := range existingrules {
		if !IsContainedRule(&existingrules[i], rules) {
			if err := netlink.RuleDel(&existingrules[i]); err != nil {
				return err
			}
		}
	}
	return nil
}

// IsContainedRule checks if the given rule is contained in the given rules list.
func IsContainedRule(existingrule *netlink.Rule, rules []networkingv1beta1.Rule) bool {
	for i := range rules {
		if RuleIsEqual(&rules[i], existingrule) {
			return true
		}
	}
	return false
}
