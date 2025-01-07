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

package args

import "net"

// CIDR implements the flag.Value interface and allows to parse strings in CIDR format lists
// in the form: "x.x.x.x/y".
type CIDR struct {
	network net.IPNet
}

// String returns the stringified list.
func (c *CIDR) String() string {
	return c.network.String()
}

// Set parses the provided string in net.IPnet.
func (c *CIDR) Set(str string) error {
	_, ipNet, err := net.ParseCIDR(str)
	if err != nil {
		return err
	}
	c.network = *ipNet
	return nil
}

// Type returns the cidrList type.
func (c *CIDR) Type() string {
	return "cidr"
}
