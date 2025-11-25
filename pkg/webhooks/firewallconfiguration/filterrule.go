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
	if filterRule.Name == nil {
		return fmt.Errorf("filterrule has nil name")
	}
	for _, match := range filterRule.Match {
		if err := checkRuleMatch(&match); err != nil {
			return fmt.Errorf("filterrule %s: %v", *filterRule.Name, err)
		}
	}

	return nil
}
