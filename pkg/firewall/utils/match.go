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
	"fmt"
	"net"
	"strconv"

	"github.com/google/nftables"
	"github.com/google/nftables/binaryutil"
	"github.com/google/nftables/expr"
	"golang.org/x/sys/unix"

	firewallv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
	"github.com/liqotech/liqo/pkg/utils/network/port"
)

func applyMatch(m *firewallv1beta1.Match, rule *nftables.Rule) error {
	op, err := getMatchCmpOp(m)
	if err != nil {
		return err
	}

	if m.Proto != nil {
		err = applyMatchProto(m, rule)
		if err != nil {
			return err
		}
	}
	if m.Dev != nil {
		err = applyMatchDev(m, rule, op)
		if err != nil {
			return err
		}
	}
	if m.IP != nil {
		err = applyMatchIP(m, rule, op)
		if err != nil {
			return err
		}
	}
	if m.Port != nil {
		err = applyMatchPort(m, rule, op)
		if err != nil {
			return err
		}
	}
	return nil
}

func applyMatchIP(m *firewallv1beta1.Match, rule *nftables.Rule, op expr.CmpOp) error {
	matchIPValueType, err := firewallv1beta1.GetIPValueType(&m.IP.Value)
	if err != nil {
		return err
	}

	switch matchIPValueType {
	case firewallv1beta1.IPValueTypeIP:
		return applyMatchIPSingleIP(m, rule, op)
	case firewallv1beta1.IPValueTypeSubnet:
		return applyMatchIPPoolSubnet(m, rule, op)
	default:
		return fmt.Errorf("invalid match value type %s", matchIPValueType)
	}
}

func applyMatchIPSingleIP(m *firewallv1beta1.Match, rule *nftables.Rule, op expr.CmpOp) error {
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

func applyMatchIPPoolSubnet(m *firewallv1beta1.Match, rule *nftables.Rule, op expr.CmpOp) error {
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

func applyMatchPortSinglePort(m *firewallv1beta1.Match, rule *nftables.Rule, op expr.CmpOp) error {
	posOffset, err := getMatchPortPositionOffset(m)
	if err != nil {
		return err
	}
	p, err := strconv.Atoi(m.Port.Value)
	if err != nil {
		return err
	}

	rule.Exprs = append(rule.Exprs,
		&expr.Payload{
			DestRegister: 1,
			Base:         expr.PayloadBaseTransportHeader,
			Offset:       posOffset,
			Len:          2,
		},
		&expr.Cmp{
			Op:       op,
			Register: 1,
			Data:     binaryutil.BigEndian.PutUint16(uint16(p)),
		},
	)
	return nil
}

func applyMatchPortRange(m *firewallv1beta1.Match, rule *nftables.Rule) error {
	posOffset, err := getMatchPortPositionOffset(m)
	if err != nil {
		return err
	}

	startport, endport, err := port.ParsePortRange(m.Port.Value)
	if err != nil {
		return err
	}

	rule.Exprs = append(rule.Exprs,
		// [ payload load 2b @ transport header + 2 => reg 1 ]
		&expr.Payload{
			DestRegister: 1,
			Base:         expr.PayloadBaseTransportHeader,
			Offset:       posOffset,
			Len:          2,
		},
		// [ cmp gte reg 1 0x0000e60f ]
		&expr.Cmp{
			Op:       expr.CmpOpGte,
			Register: 1,
			Data:     binaryutil.BigEndian.PutUint16(startport),
		},
		// [ cmp lte reg 1 0x0000fa0f ]
		&expr.Cmp{
			Op:       expr.CmpOpLte,
			Register: 1,
			Data:     binaryutil.BigEndian.PutUint16(endport),
		},
	)
	return nil
}

func applyMatchProto(m *firewallv1beta1.Match, rule *nftables.Rule) error {
	p, err := getMatchProtoValue(m)
	if err != nil {
		return fmt.Errorf("invalid match proto value %s", m.Proto.Value)
	}

	rule.Exprs = append(rule.Exprs,
		&expr.Meta{Key: expr.MetaKeyL4PROTO, Register: 1},
		&expr.Cmp{
			Op:       expr.CmpOpEq,
			Register: 1,
			Data:     []byte{p},
		},
	)
	return nil
}

func applyMatchDev(m *firewallv1beta1.Match, rule *nftables.Rule, op expr.CmpOp) error {
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

func applyMatchPort(m *firewallv1beta1.Match, rule *nftables.Rule, op expr.CmpOp) error {
	matchPortValueType, err := firewallv1beta1.GetPortValueType(&m.IP.Value)
	if err != nil {
		return err
	}

	switch matchPortValueType {
	case firewallv1beta1.PortValueTypePort:
		return applyMatchPortSinglePort(m, rule, op)
	case firewallv1beta1.PortValueTypeRange:
		return applyMatchPortRange(m, rule)
	default:
		return fmt.Errorf("invalid match value type %s", matchPortValueType)
	}
}

func getMatchCmpOp(m *firewallv1beta1.Match) (expr.CmpOp, error) {
	switch m.Op {
	case firewallv1beta1.MatchOperationEq:
		return expr.CmpOpEq, nil
	case firewallv1beta1.MatchOperationNeq:
		return expr.CmpOpNeq, nil
	}
	return expr.CmpOp(0), fmt.Errorf("invalid match operation %s", m.Op)
}

func getMatchIPPositionOffset(m *firewallv1beta1.Match) (uint32, error) {
	switch m.IP.Position {
	case firewallv1beta1.MatchPositionSrc:
		return 12, nil
	case firewallv1beta1.MatchPositionDst:
		return 16, nil
	}
	return 0, fmt.Errorf("invalid match IP position %s", m.Dev.Position)
}

func getMatchPortPositionOffset(m *firewallv1beta1.Match) (uint32, error) {
	switch m.Port.Position {
	case firewallv1beta1.MatchPositionSrc:
		return 0, nil
	case firewallv1beta1.MatchPositionDst:
		return 2, nil
	}
	return 0, fmt.Errorf("invalid match IP position %s", m.Dev.Position)
}

func getMatchProtoValue(m *firewallv1beta1.Match) (uint8, error) {
	switch m.Proto.Value {
	case firewallv1beta1.L4ProtoTCP:
		return unix.IPPROTO_TCP, nil
	case firewallv1beta1.L4ProtoUDP:
		return unix.IPPROTO_UDP, nil
	}
	return 0, fmt.Errorf("invalid match IP position %s", m.Dev.Position)
}

func getMatchDevMetaKey(m *firewallv1beta1.Match) (expr.MetaKey, error) {
	switch m.Dev.Position {
	case firewallv1beta1.MatchDevPositionIn:
		return expr.MetaKeyIIFNAME, nil
	case firewallv1beta1.MatchDevPositionOut:
		return expr.MetaKeyOIFNAME, nil
	}
	return 0, fmt.Errorf("invalid match IP position %s", m.Dev.Position)
}

func ifname(n string) []byte {
	b := make([]byte, 16)
	copy(b, n+"\x00")
	return b
}
