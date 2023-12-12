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

	return IPValueTypeVoid, fmt.Errorf("invalid match value %s", *value)
}
