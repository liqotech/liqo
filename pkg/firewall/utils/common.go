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
	"strings"

	firewallv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
	"github.com/liqotech/liqo/pkg/utils/network/port"
)

// GetIPValueType parses the match value and returns the type of the value.
func GetIPValueType(value *string) (firewallv1beta1.IPValueType, error) {
	if value == nil {
		return firewallv1beta1.IPValueTypeVoid, nil
	}

	// Check if the value is a pool subnet.
	if _, _, err := net.ParseCIDR(*value); err == nil {
		return firewallv1beta1.IPValueTypeSubnet, nil
	}

	// Check if the value is an IP.
	if net.ParseIP(*value) != nil {
		return firewallv1beta1.IPValueTypeIP, nil
	}

	// Check if the value is an IP range.
	if _, err := GetIPValueTypeRange(*value); err == nil {
		return firewallv1beta1.IPValueTypeRange, nil
	}

	// Check if the value is a named set.
	if _, err := GetIPValueNamedSet(*value); err == nil {
		return firewallv1beta1.IPValueTypeNamedSet, nil
	}

	return firewallv1beta1.IPValueTypeVoid, fmt.Errorf("invalid match value IP %s", *value)
}

// GetIPValueTypeRange parses the match value and returns the type of the value.
func GetIPValueTypeRange(s string) (firewallv1beta1.IPValueType, error) {
	_, _, err := GetIPValueRange(s)
	if err == nil {
		return firewallv1beta1.IPValueTypeRange, nil
	}

	return firewallv1beta1.IPValueTypeVoid, err
}

// GetIPValueRange parses the match value and returns the range of IPs.
func GetIPValueRange(s string) (address1, address2 net.IP, err error) {
	parts := strings.Split(s, "-")
	if len(parts) != 2 {
		return nil, nil, fmt.Errorf("invalid format: %s", s)
	}

	addr1 := strings.TrimSpace(parts[0])
	startIP := net.ParseIP(addr1)

	if startIP == nil {
		return nil, nil, fmt.Errorf("invalid first IP address: %s", addr1)
	}

	addr2 := strings.TrimSpace(parts[1])
	endIP := net.ParseIP(addr2)
	if endIP == nil {
		return nil, nil, fmt.Errorf("invalid second IP address: %s", addr2)
	}

	return startIP, endIP, nil
}

// GetIPValueNamedSet parses the match value and returns the set name.
func GetIPValueNamedSet(s string) (string, error) {
	if !strings.HasPrefix(s, "@") {
		return "", fmt.Errorf("invalid named set format: %s", s)
	}

	setName := strings.TrimPrefix(s, "@")
	if setName == "" {
		return "", fmt.Errorf("empty named set name in value: %s", s)
	}

	return setName, nil
}

// GetPortValueType parses the match value and returns the type of the value.
func GetPortValueType(value *string) (firewallv1beta1.PortValueType, error) {
	if value == nil {
		return firewallv1beta1.PortValueTypeVoid, nil
	}

	// Check if the value is a port range.
	if _, _, err := port.ParsePortRange(*value); err == nil {
		return firewallv1beta1.PortValueTypeRange, nil
	}

	// Check if the value is a port.
	if _, err := strconv.Atoi(*value); err == nil {
		return firewallv1beta1.PortValueTypePort, nil
	}

	return firewallv1beta1.PortValueTypeVoid, fmt.Errorf("invalid match value %s", *value)
}

// GetCIDRRange calculates the start and end IP addresses of a CIDR block.
func GetCIDRRange(cidr string) (net.IP, net.IP, error) {
	ip, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid CIDR format: %s", cidr)
	}

	startIP := ip.Mask(ipNet.Mask)
	endIP := make(net.IP, len(startIP))
	copy(endIP, startIP)

	for i := range endIP {
		endIP[i] |= ^ipNet.Mask[i]
	}

	return startIP, endIP, nil
}
