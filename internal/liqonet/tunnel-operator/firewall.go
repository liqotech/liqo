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

package tunneloperator

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-iptables/iptables"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	liqonetsignals "github.com/liqotech/liqo/pkg/liqonet/utils/signals"
)

const (
	filterTable = "filter"
)

type firewallRule struct {
	table string
	chain string
	rule  []string
}

func (fr *firewallRule) String() string {
	ruleString := strings.Join(fr.rule, " ")
	return strings.Join([]string{fr.table, fr.chain, ruleString}, " ")
}

func enforceFirewallRules(ctx context.Context, wg *sync.WaitGroup, ipt *iptables.IPTables, ifaceName string) {
	wg.Add(1)
	defer wg.Done()
	err := wait.PollUntilContextCancel(ctx, 5*time.Second, false, func(ctx context.Context) (done bool, err error) {
		rules := generateForwardingRules(ifaceName)
		for i := range rules {
			if err := addRule(ipt, &rules[i]); err != nil {
				return false, err
			}
		}
		return false, nil
	})
	if err != nil && ctx.Err() == nil {
		klog.Errorf("Unable to enforce firewall rules: %v", err)
		utilruntime.Must(liqonetsignals.Shutdown())
	}
}

// generateRules generates the firewall rules for the given overlay interface.
func generateForwardingRules(ifaceName string) []firewallRule {
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
