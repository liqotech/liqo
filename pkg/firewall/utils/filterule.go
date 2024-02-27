// Copyright 2019-2024 The Liqo Authors
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

package utils

import (
	"bytes"
	"fmt"
	"strconv"

	"github.com/google/nftables"
	"github.com/google/nftables/binaryutil"
	"github.com/google/nftables/expr"
	"github.com/google/nftables/userdata"

	firewallv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1/firewall"
)

var _ Rule = &FilterRuleWrapper{}

// FilterRuleWrapper is a wrapper for a FilterRule.
type FilterRuleWrapper struct {
	*firewallv1alpha1.FilterRule
}

// GetName returns the name of the rule.
func (fr *FilterRuleWrapper) GetName() *string {
	return fr.Name
}

// SetName sets the name of the rule.
func (fr *FilterRuleWrapper) SetName(name string) {
	fr.Name = &name
}

// Add adds the rule to the chain.
func (fr *FilterRuleWrapper) Add(nftconn *nftables.Conn, chain *nftables.Chain) error {
	rule, err := forgeFilterRule(fr.FilterRule, chain)
	if err != nil {
		return err
	}

	nftconn.AddRule(rule)
	return nil
}

// Equal checks if the rule is equal to the given one.
func (fr *FilterRuleWrapper) Equal(currentrule *nftables.Rule) bool {
	currentrule.Chain.Table = currentrule.Table
	newrule, err := forgeFilterRule(fr.FilterRule, currentrule.Chain)
	if err != nil {
		return false
	}
	if len(currentrule.Exprs) != len(newrule.Exprs) {
		return false
	}
	for i := range currentrule.Exprs {
		currentbytes, err := expr.Marshal(byte(currentrule.Table.Family), currentrule.Exprs[i])
		if err != nil {
			return false
		}
		newbytes, err := expr.Marshal(byte(newrule.Table.Family), newrule.Exprs[i])
		if err != nil {
			return false
		}
		if !bytes.Equal(currentbytes, newbytes) {
			return false
		}
	}
	return true
}

// forgeFilterRule forges a nftables rule from a FilterRule.
func forgeFilterRule(fr *firewallv1alpha1.FilterRule, chain *nftables.Chain) (*nftables.Rule, error) {
	rule := &nftables.Rule{
		Table:    chain.Table,
		Chain:    chain,
		UserData: userdata.AppendString([]byte{}, userdata.TypeComment, *fr.Name),
	}

	for i := range fr.Match {
		if err := applyMatch(&fr.Match[i], rule); err != nil {
			return nil, err
		}
	}

	switch fr.Action {
	case firewallv1alpha1.ActionCtMark:
		err := applyCtMarkAction(fr.Value, rule)
		if err != nil {
			return nil, fmt.Errorf("cannot apply ctmark action: %w", err)
		}
	case firewallv1alpha1.ActionSetMetaMarkFromCtMark:
		applySetMetaMarkFromCtMarkAction(rule)
	default:
	}
	return rule, nil
}

func applyCtMarkAction(value *string, rule *nftables.Rule) error {
	valueInt, err := strconv.Atoi(*value)
	if err != nil {
		return fmt.Errorf("cannot convert value to int: %w", err)
	}
	rule.Exprs = []expr.Any{
		//	[ immediate reg 1 0x00000001 ]
		&expr.Immediate{
			Register: 1,
			Data:     binaryutil.NativeEndian.PutUint32(uint32(valueInt)),
		},
		// [ ct set mark with reg 1 ]
		&expr.Ct{
			Key:            expr.CtKeyMARK,
			Register:       1,
			SourceRegister: true,
		},
	}
	return nil
}

func applySetMetaMarkFromCtMarkAction(rule *nftables.Rule) {
	rule.Exprs = []expr.Any{
		&expr.Ct{
			Register:       1,
			Key:            expr.CtKeyMARK,
			SourceRegister: false,
		},
		&expr.Meta{
			Key:            expr.MetaKeyMARK,
			SourceRegister: true,
			Register:       1,
		},
	}
}
