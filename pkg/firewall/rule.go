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

package firewall

import (
	"fmt"

	"github.com/google/nftables"
	"github.com/google/nftables/binaryutil"
	"github.com/google/nftables/userdata"

	firewallapi "github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
	firewallutils "github.com/liqotech/liqo/pkg/firewall/utils"
)

func addRules(nftconn *nftables.Conn, chain *firewallapi.Chain, nftchain *nftables.Chain) (bool, error) {
	notrackApplied := false
	tunnels := GetTunnelInterfaces()
	apirules := FromChainToRulesArray(chain, tunnels)
	if err := ensureSetsForChain(nftconn, nftchain.Table, apirules); err != nil {
		return false, err
	}
	nftrules, err := nftconn.GetRules(nftchain.Table, nftchain)
	if err != nil {
		return false, err
	}
	for i := range apirules {
		if exist := existRule(nftrules, apirules[i]); !exist {
			if err := apirules[i].Add(nftconn, nftchain); err != nil {
				return false, err
			}
			if isNotrackRule(apirules[i]) {
				notrackApplied = true
			}
		}
	}
	return notrackApplied, nil
}

func isNotrackRule(rule firewallutils.Rule) bool {
	fr, ok := rule.(*firewallutils.FilterRuleWrapper)
	return ok && fr.Action == firewallapi.ActionNotrack
}

func existRule(nftrules []*nftables.Rule, rule firewallutils.Rule) bool {
	for i := range nftrules {
		name, ok := userdata.GetString(nftrules[i].UserData, userdata.TypeComment)
		if !ok {
			continue
		}
		if rule.GetName() != nil && *rule.GetName() == name {
			return true
		}
	}
	return false
}

// isRuleOutdated checks if the rule has to be deleted.
// A rule must be deleted when it's properties change
// or when it is not contained in the FirewallConfiguration CRD.
func isRuleOutdated(nftrule *nftables.Rule, rules []firewallutils.Rule) (outdated bool, ruleName string) {
	nftRuleName, nftRuleHasName := userdata.GetString(nftrule.UserData, userdata.TypeComment)
	if !nftRuleHasName {
		return true, nftRuleName
	}
	for i := range rules {
		// if rule names are not equal, continue till the end of the list
		// if the rule is not present, delete it
		if rules[i].GetName() == nil || *rules[i].GetName() != nftRuleName {
			continue
		}
		// if rule names are equal, check if the rule has been modified
		if !rules[i].Equal(nftrule) {
			// if the rule has been modified, delete it
			return true, nftRuleName
		}
		// if the rule has not been modified, do not delete it
		return false, nftRuleName
	}
	return true, nftRuleName
}

// ensureSetsForChain ensures that all nftables sets required by the rules in the chain
// are created before the rules are applied.
func ensureSetsForChain(nftconn *nftables.Conn, table *nftables.Table, apirules []firewallutils.Rule) error {
	for _, rule := range apirules {
		for _, m := range getMatchesFromRule(rule) {
			if m.Set != nil && len(m.Set.Values) > 0 {
				if err := ensureSet(nftconn, table, m.Set); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// ensureSet creates an nftables set with the given interface names if it does not already exist.
// The set name is derived from the number of interfaces (e.g. "tunnel-list-2" for 2 tunnels).
// This is deterministic: GetTunnelInterfaces always returns sorted interface names,
// so the same K tunnels always produce the same set name and elements.
func ensureSet(nftconn *nftables.Conn, table *nftables.Table, matchSet *firewallapi.MatchSet) error {
	setName := fmt.Sprintf("tunnel-list-%d", len(matchSet.Values))
	existingSets, err := nftconn.GetSets(table)
	if err == nil {
		for _, s := range existingSets {
			if s.Name == setName {
				return nil
			}
		}
	}
	namedSet := &nftables.Set{
		Table:        table,
		Name:         setName,
		KeyType:      nftables.TypeIFName,
		KeyByteOrder: binaryutil.NativeEndian,
	}
	var elements []nftables.SetElement
	for _, val := range matchSet.Values {
		elements = append(elements, nftables.SetElement{
			Key: firewallutils.Ifname(val),
		})
	}
	return nftconn.AddSet(namedSet, elements)
}

// getMatchesFromRule extracts the slice of matches from a given firewall rule
//
//	returning nil if the rule has no matches (e.g., RouteRule)
func getMatchesFromRule(rule firewallutils.Rule) []firewallapi.Match {
	switch r := rule.(type) {
	case *firewallutils.FilterRuleWrapper:
		return r.Match
	case *firewallutils.NatRuleWrapper:
		return r.Match
	default:
		return nil
	}
}
