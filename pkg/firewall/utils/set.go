// Copyright 2019-2026 The Liqo Authors
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

	firewallapi "github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
)

// ConvertSetData converts a set element data to a byte slice based on the provided data type.
// For ipv4_cidr values, it also returns the (exclusive) interval end, which is nil if the interval reaches the end of the address space.
func ConvertSetData(data *string, dataType *firewallapi.SetDataType) (value, intervalEnd []byte, err error) {
	if dataType == nil {
		if data != nil {
			return nil, nil, fmt.Errorf("set element has data but no data type is specified")
		}
		return []byte{}, nil, nil
	}

	if data == nil {
		return nil, nil, fmt.Errorf("set element has nil data for data type %s", *dataType)
	}

	switch *dataType {
	case firewallapi.SetDataTypeInteger:
		intValue, err := strconv.Atoi(*data)
		if err != nil {
			return nil, nil, fmt.Errorf("set element has invalid integer value %s", *data)
		}
		return []byte{
			byte((intValue >> 24) & 0xFF),
			byte((intValue >> 16) & 0xFF),
			byte((intValue >> 8) & 0xFF),
			byte(intValue & 0xFF),
		}, nil, nil

	case firewallapi.SetDataTypeIPAddr:
		ip := net.ParseIP(*data)
		if ip == nil || ip.To4() == nil {
			return nil, nil, fmt.Errorf("set element has invalid IPv4 value %s", *data)
		}
		return ip.To4(), nil, nil

	case firewallapi.SetDataTypeIPCIDR:
		startIP, endIP, err := GetCIDRRange(*data)
		if err != nil {
			return nil, nil, fmt.Errorf("set element has invalid CIDR value %s: %w", *data, err)
		}
		if startIP.To4() == nil {
			return nil, nil, fmt.Errorf("set element has invalid IPv4 CIDR value %s", *data)
		}
		// nftables interval ends are exclusive: the end key must be the first address after the range.
		// If the range reaches the end of the address space, no end key is needed.
		return startIP.To4(), nextIPv4(endIP.To4()), nil

	default:
		return nil, nil, fmt.Errorf("invalid set value type %s", *dataType)
	}
}

// nextIPv4 returns the IPv4 address following ip, or nil if ip is the last IPv4 address (or not a valid IPv4).
func nextIPv4(ip net.IP) []byte {
	if ip == nil {
		return nil
	}
	next := make([]byte, len(ip))
	copy(next, ip)
	for i := len(next) - 1; i >= 0; i-- {
		next[i]++
		if next[i] != 0 {
			return next
		}
	}
	// Overflow: ip was 255.255.255.255.
	return nil
}
