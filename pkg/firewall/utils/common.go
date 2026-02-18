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
	"bytes"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"k8s.io/klog/v2"

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

// filterUnstableExprs removes expression types that the nftables library doesn't preserve
// when reading rules back from the kernel. These expressions are used during rule creation
// but are optimized away or transformed by the kernel, causing false positives in equality checks.
//
// Filtered expression types:
// - Counter: Values change over time (handled by filterCounterExprs)
// - Rt: Route lookups (e.g., TCPMSS) are compiled into the rule but not preserved
// - Byteorder: Byte order conversions are applied but not stored as separate expressions
//
// This prevents infinite delete-recreate loops when nftables monitoring is enabled.
func filterUnstableExprs(exprs []expr.Any) []expr.Any {
	filtered := make([]expr.Any, 0, len(exprs))
	for i := range exprs {
		switch exprs[i].(type) {
		case *expr.Counter:
			// Skip: counter values change over time
			continue
		case *expr.Rt:
			// Skip: route lookups are compiled but not preserved by nftables library
			continue
		case *expr.Byteorder:
			// Skip: byte order conversions are applied but not stored
			continue
		default:
			filtered = append(filtered, exprs[i])
		}
	}
	return filtered
}

// logExpressionDetails logs expression details with the provided log function.
func logExpressionDetails(label string, exprs []expr.Any, logFunc func(format string, args ...interface{})) {
	logFunc("%s:", label)
	for i, e := range exprs {
		logFunc("  [%d] %T: %+v", i, e, e)
	}
}

// compareRuleExpressions compares the expressions of two nftables rules for equality.
// It filters out unstable expressions, logs the comparison details, and performs
// byte-by-byte comparison of marshaled expressions.
func compareRuleExpressions(ruleName string, currentrule, newrule *nftables.Rule) bool {
	// Filter out unstable expressions that the nftables library doesn't preserve
	// when reading rules back from the kernel (Counter, Rt, Byteorder).
	currentExprs := filterUnstableExprs(currentrule.Exprs)
	newExprs := filterUnstableExprs(newrule.Exprs)

	klog.Infof("Rule comparison: %s - current exprs: %d (filtered from %d), new exprs: %d (filtered from %d)",
		ruleName, len(currentExprs), len(currentrule.Exprs), len(newExprs), len(newrule.Exprs))

	// Log detailed expression comparison for debugging
	logExpressionDetails(fmt.Sprintf("Current expressions for rule %s", ruleName), currentExprs, klog.V(4).Infof)
	logExpressionDetails(fmt.Sprintf("New expressions for rule %s", ruleName), newExprs, klog.V(4).Infof)

	if len(currentExprs) != len(newExprs) {
		klog.Warningf("Rule %s: expression count mismatch (current: %d, new: %d)", ruleName, len(currentExprs), len(newExprs))
		logExpressionDetails("Current expressions", currentExprs, klog.Infof)
		logExpressionDetails("New expressions", newExprs, klog.Infof)
		return false
	}

	for i := range currentExprs {
		foundEqual := false
		currentbytes, err := expr.Marshal(byte(currentrule.Table.Family), currentExprs[i])
		if err != nil {
			klog.Errorf("Error while marshaling current rule %s", err.Error())
			return false
		}
		for j := range newExprs {
			newbytes, err := expr.Marshal(byte(newrule.Table.Family), newExprs[j])
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
			klog.Infof("Rule %s: expression %d/%d not found in new rule - %T: %+v", ruleName, i+1, len(currentExprs), currentExprs[i], currentExprs[i])
			logExpressionDetails("Available new expressions", newExprs, klog.Infof)
			return false
		}
	}
	klog.V(4).Infof("Rule %s: all expressions match, rules are equal", ruleName)
	return true
}
