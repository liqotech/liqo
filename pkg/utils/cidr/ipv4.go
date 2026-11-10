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
	"fmt"
	"strings"

	netutils "k8s.io/utils/net"
)

// IPv4Only parses a comma-separated list of CIDRs (e.g., "fd00:10:244::/56,10.244.0.0/16"),
// as found in the Kubernetes flags of dual-stack clusters, and returns only the IPv4 entries,
// preserving their order. The Liqo network fabric is IPv4 only, hence an error is returned
// if the list contains no IPv4 CIDR.
func IPv4Only(list string) ([]string, error) {
	var out []string
	for _, c := range strings.Split(list, ",") {
		c = strings.TrimSpace(c)
		if c != "" && netutils.IsIPv4CIDRString(c) {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("liqo requires an IPv4 CIDR; got %q", list)
	}
	return out, nil
}
