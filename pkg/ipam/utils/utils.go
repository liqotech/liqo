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
	"fmt"
	"net"
	"net/netip"

	"go4.org/netipx"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/liqotech/liqo/pkg/consts"
)

// MapIPToNetwork creates a new IP address obtained by means of the old IP address and the new network.
func MapIPToNetwork(newNetwork, oldIP string) (newIP string, err error) {
	if newNetwork == consts.DefaultCIDRValue {
		return oldIP, nil
	}
	// Parse newNetwork
	ip, network, err := net.ParseCIDR(newNetwork)
	if err != nil {
		return "", err
	}
	// Get mask
	mask := network.Mask
	// Get slice of bytes for newNetwork
	// Type net.IP has underlying type []byte
	parsedNewIP := ip.To4()
	// Get oldIP as slice of bytes
	parsedOldIP := net.ParseIP(oldIP)
	if parsedOldIP == nil {
		return "", fmt.Errorf("cannot parse oldIP")
	}
	parsedOldIP = parsedOldIP.To4()
	// Substitute the last 32-mask bits of newNetwork with bits taken by the old ip
	for i := 0; i < len(mask); i++ {
		// Step 1: NOT(mask[i]) = mask[i] ^ 0xff. They are the 'host' bits
		// Step 2: BITWISE AND between the host bits and parsedOldIP[i] zeroes the network bits in parsedOldIP[i]
		// Step 3: BITWISE OR copies the result of step 2 in newIP[i]
		parsedNewIP[i] |= (mask[i] ^ 0xff) & parsedOldIP[i]
	}
	newIP = parsedNewIP.String()
	return
}

// GetMask retrieves the mask from a CIDR.
func GetMask(network string) uint8 {
	_, subnet, err := net.ParseCIDR(network)
	utilruntime.Must(err)
	ones, _ := subnet.Mask.Size()
	return uint8(ones)
}

// SetMask forges a new cidr from a network cidr and a mask.
func SetMask(network string, mask uint8) string {
	_, n, err := net.ParseCIDR(network)
	utilruntime.Must(err)
	newMask := net.CIDRMask(int(mask), 32)
	n.Mask = newMask
	return n.String()
}

// Next used to get the second half of a given network.
func Next(network string) string {
	prefix, err := netip.ParsePrefix(network)
	utilruntime.Must(err)
	// Step 1: Get last IP address of network
	// Step 2: Get next IP address
	firstIP := netipx.RangeOfPrefix(prefix).To().Next()
	prefix = netip.PrefixFrom(firstIP, prefix.Bits())
	return prefix.String()
}

// IsValidCIDR returns an error if the received CIDR is invalid.
func IsValidCIDR(cidr string) error {
	_, _, err := net.ParseCIDR(cidr)
	return err
}

// GetFirstIP returns the first IP address of a network.
func GetFirstIP(network string) (string, error) {
	firstIP, _, err := net.ParseCIDR(network)
	if err != nil {
		return "", err
	}
	return firstIP.String(), nil
}

// GetTunnelIP returns the IP address of the tunnel, which is the first external CIDR ip.
func GetTunnelIP(externalCIDR string) (string, error) {
	ipPrefix, err := netip.ParsePrefix(externalCIDR)
	if err != nil {
		return "", err
	}
	return ipPrefix.Addr().Next().String(), nil
}

// SplitNetwork returns the two halves that make up a given network.
func SplitNetwork(network string) []string {
	halves := make([]string, 2)

	// Get halves mask length.
	mask := GetMask(network)
	mask++

	// Get first half CIDR.
	halves[0] = SetMask(network, mask)

	// Get second half CIDR.
	halves[1] = Next(halves[0])

	return halves
}

// GetUnknownSourceIP returns the IP address used to map unknown sources.
func GetUnknownSourceIP(extCIDR string) (string, error) {
	if extCIDR == "" {
		return "", fmt.Errorf("ExternalCIDR not set")
	}
	firstExtCIDRip, err := GetFirstIP(extCIDR)
	if err != nil {
		return "", fmt.Errorf("cannot get first IP of ExternalCIDR")
	}
	return firstExtCIDRip, nil
}
