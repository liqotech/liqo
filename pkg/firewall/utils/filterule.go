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

package utils

import (
	"bytes"
	"fmt"
	"strconv"

	"github.com/google/nftables"
	"github.com/google/nftables/binaryutil"
	"github.com/google/nftables/expr"
	"github.com/google/nftables/userdata"
	"k8s.io/klog/v2"

	firewallv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
)

var _ Rule = &FilterRuleWrapper{}

// FilterRuleWrapper is a wrapper for a FilterRule.
type FilterRuleWrapper struct {
	*firewallv1beta1.FilterRule
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
	// TODO: this ugly exception is caused by an error in the expr retrieved by nftables library.
	// In particular, the expr retrieved by the library when the action is ctmark
	// Retrieved expr: &{0 false 3}
	// Generated expr: &{1 true 3}
	// We think that this error should be caused by a library bug.
	// We are going to investigate it further.
	if fr.FilterRule.Action == firewallv1beta1.ActionCtMark {
		return true
	}
	if err != nil {
		return false
	}
	if len(currentrule.Exprs) != len(newrule.Exprs) {
		return false
	}
	for i := range currentrule.Exprs {
		foundEqual := false
		currentbytes, err := expr.Marshal(byte(currentrule.Table.Family), currentrule.Exprs[i])
		if err != nil {
			klog.Errorf("Error while marshaling current rule %s", err.Error())
			return false
		}
		for j := range newrule.Exprs {
			newbytes, err := expr.Marshal(byte(newrule.Table.Family), newrule.Exprs[j])
			if err != nil {
				klog.Errorf("Error while marshaling new rule %s", err.Error())
				return false
			}
			if bytes.Equal(currentbytes, newbytes) {
				foundEqual = true
				break
			}
		}
		if !foundEqual {
			return false
		}
	}
	return true
}

// forgeFilterRule forges a nftables rule from a FilterRule.
func forgeFilterRule(fr *firewallv1beta1.FilterRule, chain *nftables.Chain) (*nftables.Rule, error) {
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
	case firewallv1beta1.ActionCtMark:
		err := applyCtMarkAction(fr.Value, rule)
		if err != nil {
			return nil, fmt.Errorf("cannot apply ctmark action: %w", err)
		}
	case firewallv1beta1.ActionSetMetaMarkFromCtMark:
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
	rule.Exprs = append(rule.Exprs,
		&expr.Immediate{
			Register: 1,
			Data:     binaryutil.NativeEndian.PutUint32(uint32(valueInt)),
		}, &expr.Ct{
			Register:       1,
			SourceRegister: true,
			Key:            expr.CtKeyMARK,
		},
	)
	return nil
}

func applySetMetaMarkFromCtMarkAction(rule *nftables.Rule) {
	rule.Exprs = append(rule.Exprs,
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
	)
}
