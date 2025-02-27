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
	"fmt"
	"net"
	"regexp"
	"strconv"

	"github.com/liqotech/liqo/pkg/utils/network/port"
)

// IPValueType is the type of the match value.
type IPValueType string

const (
	// IPValueTypeIP is a string representing an ip.
	IPValueTypeIP IPValueType = "ip"
	// IPValueTypeSubnet is a string representing a subnet (eg. 10.0.0.0/24).
	IPValueTypeSubnet IPValueType = "subnet"
	// IPValueTypeVoid is a void match value.
	IPValueTypeVoid IPValueType = "void"
	// IPValueTypeRange is a string representing a range of IPs (eg. 10.0.0.1-10.0.0.20).
	IPValueTypeRange IPValueType = "range"
)

// PortValueType is the type of the match value.
type PortValueType string

const (
	// PortValueTypePort is a string representing a port.
	PortValueTypePort PortValueType = "port"
	// PortValueTypeRange is a string representing a range of ports (eg. 3000-4000).
	PortValueTypeRange PortValueType = "range"
	// PortValueTypeVoid is a void match value.
	PortValueTypeVoid PortValueType = "void"
)

// GetIPValueType parses the match value and returns the type of the value.
func GetIPValueType(value *string) (IPValueType, error) {
	if value == nil {
		return IPValueTypeVoid, nil
	}

	// Check if the value is a pool subnet.
	if _, _, err := net.ParseCIDR(*value); err == nil {
		return IPValueTypeSubnet, nil
	}

	// Check if the value is an IP.
	if net.ParseIP(*value) != nil {
		return IPValueTypeIP, nil
	}

	// Check if the value is an IP range.
	if _, err := getIPValueParseRange(*value); err == nil {
		return IPValueTypeRange, nil
	}

	return IPValueTypeVoid, fmt.Errorf("invalid match value IP %s", *value)
}

// GetPortValueType parses the match value and returns the type of the value.
func GetPortValueType(value *string) (PortValueType, error) {
	if value == nil {
		return PortValueTypeVoid, nil
	}

	// Check if the value is a port range.
	if _, _, err := port.ParsePortRange(*value); err == nil {
		return PortValueTypeRange, nil
	}

	// Check if the value is a port.
	if _, err := strconv.Atoi(*value); err != nil {
		return PortValueTypePort, nil
	}

	return PortValueTypeVoid, fmt.Errorf("invalid match value PORT %s", *value)
}

// support function, should be moved to pkg/utils
func getIPValueParseRange(s string) (IPValueType, error) {
	var rangeRegex = regexp.MustCompile(`^\s*(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})\s*-\s*(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})\s*$`)
	matches := rangeRegex.FindStringSubmatch(s)

	addr1, addr2 := matches[1], matches[2]

	ipAddr1 := net.ParseIP(addr1)
	ipAddr2 := net.ParseIP(addr2)

	if matches != nil && ipAddr1 != nil && ipAddr2 != nil {
		return IPValueTypeRange, nil
	}
	return IPValueTypeVoid, fmt.Errorf("invalid match value %s - %s, matches: %s", ipAddr1, ipAddr2, matches)
}
