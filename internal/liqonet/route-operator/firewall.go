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

package routeoperator

import (
	"fmt"
	"strings"

	"github.com/coreos/go-iptables/iptables"
)

const (
	filterTable = "filter"
)

// TODO: refactor the iptables package used by tunnel operator in order
// to use it outside the tunnel operator.

type firewallRule struct {
	table string
	chain string
	rule  []string
}

func (fr *firewallRule) String() string {
	ruleString := strings.Join(fr.rule, " ")
	return strings.Join([]string{fr.table, fr.chain, ruleString}, " ")
}

// generateRules generates the firewall rules for the given overlay interface.
func generateRules(ifaceName string) []firewallRule {
	comment := fmt.Sprintf("LIQO accept traffic from/to overlay interface %s", ifaceName)
	return []firewallRule{
		{
			table: filterTable,
			chain: "INPUT",
			rule:  []string{"-i", ifaceName, "-j", "ACCEPT", "-m", "comment", "--comment", comment},
		},
		{
			table: filterTable,
			chain: "FORWARD",
			rule:  []string{"-i", ifaceName, "-j", "ACCEPT", "-m", "comment", "--comment", comment},
		},
		{
			table: filterTable,
			chain: "OUTPUT",
			rule:  []string{"-o", ifaceName, "-j", "ACCEPT", "-m", "comment", "--comment", comment},
		},
	}
}

// addRule appends the rule if it does not exist.
func addRule(ipt *iptables.IPTables, rule *firewallRule) error {
	return ipt.AppendUnique(rule.table, rule.chain, rule.rule...)
}

// deleteRule removes the rule if it exists.
func deleteRule(ipt *iptables.IPTables, rule *firewallRule) error {
	return ipt.DeleteIfExists(rule.table, rule.chain, rule.rule...)
}
