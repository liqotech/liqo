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

package firewall

import (
	"github.com/google/nftables"
	"github.com/google/nftables/userdata"

	firewallapi "github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
	firewallutils "github.com/liqotech/liqo/pkg/firewall/utils"
)

func addRules(nftconn *nftables.Conn, chain *firewallapi.Chain, nftchain *nftables.Chain) error {
	apirules := FromChainToRulesArray(chain)
	nftrules, err := nftconn.GetRules(nftchain.Table, nftchain)
	if err != nil {
		return err
	}
	for i := range apirules {
		if exist := existRule(nftrules, apirules[i]); !exist {
			if err := apirules[i].Add(nftconn, nftchain); err != nil {
				return err
			}
		}
	}
	return nil
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
