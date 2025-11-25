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
	// IPValueTypeNamedSet is a string representing the name of an IP set (eg. @my_ip_set).
	IPValueTypeNamedSet IPValueType = "namedset"
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
