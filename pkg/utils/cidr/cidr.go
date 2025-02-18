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

package cidr

import networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"

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
