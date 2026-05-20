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

package cidr

import (
	"slices"
	"sort"
	"strings"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
)

// GetPrimary returns the primary CIDR from a list of CIDRs.
func GetPrimary(cidrs []networkingv1beta1.CIDR) *networkingv1beta1.CIDR {
	if len(cidrs) == 0 {
		return nil
	}
	return &cidrs[0]
}

// SetPrimary sets the primary CIDR in a list of CIDRs.
func SetPrimary(cidr networkingv1beta1.CIDR) []networkingv1beta1.CIDR {
	return []networkingv1beta1.CIDR{cidr}
}

// IsVoid checks if a CIDR is void.
func IsVoid(cidr *networkingv1beta1.CIDR) bool {
	if cidr == nil {
		return true
	}
	return cidr.String() == ""
}

// AreAllVoid reports whether a CIDR list is empty or contains only empty entries.
func AreAllVoid(cidrs []networkingv1beta1.CIDR) bool {
	for i := range cidrs {
		if cidrs[i].String() != "" {
			return false
		}
	}
	return true
}

// AllNonVoid reports whether the list is non-empty and every entry is non-empty.
func AllNonVoid(cidrs []networkingv1beta1.CIDR) bool {
	if len(cidrs) == 0 {
		return false
	}
	for i := range cidrs {
		if cidrs[i].String() == "" {
			return false
		}
	}
	return true
}

// EscapeForName converts a CIDR value into a DNS-1123-compliant suffix usable as a
// Kubernetes resource name component by replacing "/" and "." with "-".
// Example: "10.244.0.0/16" -> "10-244-0-0-16".
func EscapeForName(cidr networkingv1beta1.CIDR) string {
	s := string(cidr)
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, ".", "-")
	return s
}

// Strings converts a CIDR slice to a slice of strings, preserving order.
func Strings(cidrs []networkingv1beta1.CIDR) []string {
	if cidrs == nil {
		return nil
	}
	out := make([]string, len(cidrs))
	for i := range cidrs {
		out[i] = cidrs[i].String()
	}
	return out
}

// FromStrings converts a slice of strings to a CIDR slice, preserving order.
func FromStrings(s []string) []networkingv1beta1.CIDR {
	if s == nil {
		return nil
	}
	out := make([]networkingv1beta1.CIDR, len(s))
	for i := range s {
		out[i] = networkingv1beta1.CIDR(s[i])
	}
	return out
}

// EqualOrdered reports whether two CIDR lists are equal element-by-element in order.
func EqualOrdered(a, b []networkingv1beta1.CIDR) bool {
	return slices.Equal(a, b)
}

// EqualAsSet reports whether two CIDR lists contain the same elements regardless of order.
// Duplicates are treated as significant: [A, A, B] is not equal to [A, B, B].
func EqualAsSet(a, b []networkingv1beta1.CIDR) bool {
	if len(a) != len(b) {
		return false
	}
	as := Strings(a)
	bs := Strings(b)
	sort.Strings(as)
	sort.Strings(bs)

	return slices.Equal(as, bs)
}
