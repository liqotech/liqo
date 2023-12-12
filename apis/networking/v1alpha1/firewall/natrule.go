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

package firewall

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"github.com/google/nftables/userdata"
)

// GetName returns the name of the rule.
func (nr *NatRule) GetName() *string {
	return nr.Name
}

// SetName sets the name of the rule.
func (nr *NatRule) SetName(name string) {
	nr.Name = &name
}

// Add adds the rule to the chain.
func (nr *NatRule) Add(nftconn *nftables.Conn, chain *nftables.Chain) error {
	rule, err := forgeNatRule(nr, chain)
	if err != nil {
		return err
	}

	nftconn.AddRule(rule)
	return nil
}

// Equal checks if the rule is equal to the given one.
func (nr *NatRule) Equal(currentrule *nftables.Rule) bool {
	currentrule.Chain.Table = currentrule.Table
	newrule, err := forgeNatRule(nr, currentrule.Chain)
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

func forgeNatRule(nr *NatRule, chain *nftables.Chain) (*nftables.Rule, error) {
	rule := &nftables.Rule{
		Table:    chain.Table,
		Chain:    chain,
		UserData: userdata.AppendString([]byte{}, userdata.TypeComment, *nr.Name),
	}

	for i := range nr.Match {
		if err := applyMatch(&nr.Match[i], rule); err != nil {
			return nil, err
		}
	}

	if err := applyNatRule(nr, rule); err != nil {
		return nil, err
	}

	return rule, nil
}

func applyNatRule(nr *NatRule, rule *nftables.Rule) error {
	ipType, err := GetIPValueType(nr.To)
	if err != nil {
		return err
	}

	natType, err := getNatRuleType(nr)
	if err != nil {
		return err
	}

	switch ipType {
	case IPValueTypeIP:
		return applyNatIP(nr.To, natType, rule)
	case IPValueTypeSubnet:
		return applyNatSubnet(nr.To, natType, rule)
	case IPValueTypeVoid:
		return applyNatVoid(rule)
	}
	return nil
}

func applyNatIP(ip *string, natType expr.NATType, rule *nftables.Rule) error {
	if ip == nil {
		return fmt.Errorf("\"to\" argument cannot be nil for nat type snat/dnat")
	}
	ipNet := net.ParseIP(*ip)
	if ipNet == nil {
		return fmt.Errorf("invalid ip %s", *ip)
	}

	rule.Exprs = append(rule.Exprs,
		&expr.Immediate{
			Register: 1,
			Data:     ipNet.To4(),
		},
		&expr.NAT{
			Type:       natType,
			RegAddrMin: 1,
			Family:     uint32(rule.Table.Family),
		})
	return nil
}

func applyNatVoid(rule *nftables.Rule) error {
	rule.Exprs = append(rule.Exprs, &expr.Masq{})
	return nil
}

func applyNatSubnet(ip *string, natType expr.NATType, rule *nftables.Rule) error {
	if ip == nil {
		return fmt.Errorf("\"to\" argument cannot be nil for nat type snat/dnat")
	}
	_, subnet, err := net.ParseCIDR(*ip)
	if err != nil {
		return err
	}

	mask := binary.BigEndian.Uint32(subnet.Mask)
	start := binary.BigEndian.Uint32(subnet.IP)

	// find the final address
	lastIP := make(net.IP, 4)
	binary.BigEndian.PutUint32(lastIP, (start&mask)|(mask^0xffffffff))

	rule.Exprs = append(rule.Exprs,
		&expr.Immediate{
			Register: 1,
			Data:     subnet.IP,
		},
		&expr.Immediate{
			Register: 2,
			Data:     lastIP,
		},
		&expr.NAT{
			Type:       natType,
			RegAddrMin: 1,
			RegAddrMax: 2,
			Prefix:     true,
			Family:     uint32(rule.Table.Family),
		},
	)
	return nil
}

func getNatRuleType(natrule *NatRule) (expr.NATType, error) {
	switch natrule.NatType {
	case NatTypeDestination:
		return expr.NATTypeDestNAT, nil
	case NatTypeSource, NatTypeMasquerade:
		return expr.NATTypeSourceNAT, nil
	default:
		return expr.NATType(0), fmt.Errorf("invalid nat type %s", natrule.NatType)
	}
}
