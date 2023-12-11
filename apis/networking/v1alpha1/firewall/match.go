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
	"fmt"
	"net"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
)

func applyMatch(m *Match, rule *nftables.Rule) error {
	op, err := getMatchCmpOp(m)
	if err != nil {
		return err
	}

	switch {
	case m.IP != nil:
		return applyMatchIP(m, rule, op)
	case m.Dev != nil:
		return applyMatchDev(m, rule, op)
	}
	return nil
}

func applyMatchIP(m *Match, rule *nftables.Rule, op expr.CmpOp) error {
	matchIPValueType, err := GetIPValueType(&m.IP.Value)
	if err != nil {
		return err
	}

	switch matchIPValueType {
	case IPValueTypeIP:
		return applyMatchIPSingleIP(m, rule, op)
	case IPValueTypeSubnet:
		return applyMatchIPPoolSubnet(m, rule, op)
	default:
		return fmt.Errorf("invalid match value type %s", matchIPValueType)
	}
}

func applyMatchIPSingleIP(m *Match, rule *nftables.Rule, op expr.CmpOp) error {
	posOffset, err := getMatchIPPositionOffset(m)
	if err != nil {
		return err
	}

	rule.Exprs = append(rule.Exprs,
		&expr.Payload{
			DestRegister: 1,
			Base:         expr.PayloadBaseNetworkHeader,
			Offset:       posOffset,
			Len:          4,
		},
		&expr.Cmp{
			Op:       op,
			Register: 1,
			Data:     net.ParseIP(m.IP.Value).To4(),
		},
	)
	return nil
}

func applyMatchIPPoolSubnet(m *Match, rule *nftables.Rule, op expr.CmpOp) error {
	posOffset, err := getMatchIPPositionOffset(m)
	if err != nil {
		return err
	}

	ip, subnet, err := net.ParseCIDR(m.IP.Value)
	if err != nil {
		return err
	}

	rule.Exprs = append(rule.Exprs,
		&expr.Payload{
			DestRegister: 1,
			Base:         expr.PayloadBaseNetworkHeader,
			Offset:       posOffset,
			Len:          4,
		},
		&expr.Bitwise{
			SourceRegister: 1,
			DestRegister:   1,
			Len:            4,
			Xor:            []byte{0x0, 0x0, 0x0, 0x0},
			Mask:           subnet.Mask,
		},
		&expr.Cmp{
			Op:       op,
			Register: 1,
			Data:     ip.To4(),
		},
	)
	return nil
}

func applyMatchDev(m *Match, rule *nftables.Rule, op expr.CmpOp) error {
	metakey, err := getMatchDevMetaKey(m)
	if err != nil {
		return err
	}

	rule.Exprs = append(rule.Exprs,
		&expr.Meta{
			Register: 1,
			Key:      metakey,
		},
		&expr.Cmp{
			Op:       op,
			Register: 1,
			Data:     ifname(m.Dev.Value),
		},
	)

	return nil
}

func getMatchCmpOp(m *Match) (expr.CmpOp, error) {
	switch m.Op {
	case MatchOperationEq:
		return expr.CmpOpEq, nil
	case MatchOperationNeq:
		return expr.CmpOpNeq, nil
	}
	return expr.CmpOp(0), fmt.Errorf("invalid match operation %s", m.Op)
}

func getMatchIPPositionOffset(m *Match) (uint32, error) {
	switch m.IP.Position {
	case MatchIPPositionSrc:
		return 12, nil
	case MatchIPPositionDst:
		return 16, nil
	}
	return 0, fmt.Errorf("invalid match IP position %s", m.Dev.Position)
}

func getMatchDevMetaKey(m *Match) (expr.MetaKey, error) {
	switch m.Dev.Position {
	case MatchDevPositionIn:
		return expr.MetaKeyIIFNAME, nil
	case MatchDevPositionOut:
		return expr.MetaKeyOIFNAME, nil
	}
	return 0, fmt.Errorf("invalid match IP position %s", m.Dev.Position)
}

func ifname(n string) []byte {
	b := make([]byte, 16)
	copy(b, n+"\x00")
	return b
}
