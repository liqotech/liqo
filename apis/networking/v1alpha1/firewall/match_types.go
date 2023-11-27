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

// MatchOperation is the operation of the match.
type MatchOperation string

const (
	// MatchOperationEq is the operation of the match.
	MatchOperationEq MatchOperation = "eq"
	// MatchOperationNeq is the operation of the match.
	MatchOperationNeq MatchOperation = "neq"
)

// MatchIPPosition is the position of the IP in the packet.
type MatchIPPosition string

const (
	// MatchIPPositionSrc is the position of the IP in the packet.
	MatchIPPositionSrc MatchIPPosition = "src"
	// MatchIPPositionDst is the position of the IP in the packet.
	MatchIPPositionDst MatchIPPosition = "dst"
)

// MatchDevPosition is the position of the device in the packet.
type MatchDevPosition string

const (
	// MatchDevPositionIn is the position of the device in the packet.
	MatchDevPositionIn MatchDevPosition = "in"
	// MatchDevPositionOut is the position of the device in the packet.
	MatchDevPositionOut MatchDevPosition = "out"
)

// MatchIP is an IP to be matched.
// +kubebuilder:object:generate=true
type MatchIP struct {
	// Value is the IP or a SUbnet to be matched.
	Value string `json:"value"`
	// Position is the position of the IP in the packet.
	// +kubebuilder:validation:Enum=src;dst
	Position MatchIPPosition `json:"position"`
}

// MatchDev is a device to be matched.
// +kubebuilder:object:generate=true
type MatchDev struct {
	// Value is the name of the device to be matched.
	Value string `json:"value"`
	// Position is the source device of the packet.
	// +kubebuilder:validation:Enum=in;out
	Position MatchDevPosition `json:"position"`
}

// Match is a match to be applied to a rule.
// +kubebuilder:object:generate=true
// +kubebuilder:validation:MaxProperties=2
// +kubebuilder:validation:MinProperties=2
type Match struct {
	// Op is the operation of the match.
	// +kubebuilder:validation:Enum=eq;neq
	Op MatchOperation `json:"op"`
	// IP contains the options to match an IP or a Subnet.
	IP *MatchIP `json:"ip,omitempty"`
	// Dev contains the options to match a device.
	Dev *MatchDev `json:"dev,omitempty"`
}
