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

	return firewallv1beta1.IPValueTypeVoid, fmt.Errorf("invalid match value %s", *value)
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
