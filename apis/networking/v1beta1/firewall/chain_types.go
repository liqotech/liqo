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

import "math"

// ChainType defines what this chain will be used for.
// https://wiki.nftables.org/wiki-nftables/index.php/Configuring_chains#Base_chain_types
type ChainType string

// Possible ChainType values.
const (
	ChainTypeFilter ChainType = "filter"
	ChainTypeRoute  ChainType = "route"
	ChainTypeNAT    ChainType = "nat"
)

// ChainPolicy defines what this chain default policy will be.
type ChainPolicy string

// Possible ChainPolicy values.
const (
	ChainPolicyDrop   ChainPolicy = "drop"
	ChainPolicyAccept ChainPolicy = "accept"
)

// ChainHook specifies at which step in packet processing the Chain should be executed.
// https://wiki.nftables.org/wiki-nftables/index.php/Configuring_chains#Base_chain_hooks
type ChainHook string

// Possible ChainHook values.
var (
	ChainHookPrerouting  ChainHook = "prerouting"
	ChainHookInput       ChainHook = "input"
	ChainHookForward     ChainHook = "forward"
	ChainHookOutput      ChainHook = "output"
	ChainHookPostrouting ChainHook = "postrouting"
	ChainHookIngress     ChainHook = "ingress"
)

// ChainPriority orders the chain relative to Netfilter internal operations.
// https://wiki.nftables.org/wiki-nftables/index.php/Configuring_chains#Base_chain_priority
type ChainPriority int32

// Possible ChainPriority values.
// from /usr/include/linux/netfilter_ipv4.h.
var (
	ChainPriorityFirst           ChainPriority = math.MinInt32
	ChainPriorityConntrackDefrag ChainPriority = -400
	ChainPriorityRaw             ChainPriority = -300
	ChainPrioritySELinuxFirst    ChainPriority = -225
	ChainPriorityConntrack       ChainPriority = -200
	ChainPriorityMangle          ChainPriority = -150
	ChainPriorityNATDest         ChainPriority = -100
	//nolint:revive // We need a variable with zero value.
	ChainPriorityFilter           ChainPriority = 0
	ChainPrioritySecurity         ChainPriority = 50
	ChainPriorityNATSource        ChainPriority = 100
	ChainPrioritySELinuxLast      ChainPriority = 225
	ChainPriorityConntrackHelper  ChainPriority = 300
	ChainPriorityConntrackConfirm ChainPriority = math.MaxInt32
	ChainPriorityLast             ChainPriority = math.MaxInt32
)

// Chain is a chain of rules to be applied to a table.
// +kubebuilder:object:generate=true
type Chain struct {
	// Name is the name of the chain.
	Name *string `json:"name"`
	// Rules is a list of rules to be applied to the chain.
	Rules RulesSet `json:"rules"`
	// Type defines what this chain will be used for.
	// +kubebuilder:validation:Enum="filter";"route";"nat"
	Type *ChainType `json:"type"`
	// Policy defines what this chain default policy will be.
	// +kubebuilder:validation:Enum="drop";"accept"
	Policy *ChainPolicy `json:"policy"`
	// Hook specifies at which step in packet processing the Chain should be executed.
	// +kubebuilder:validation:Enum="prerouting";"input";"forward";"output";"postrouting";"ingress"
	Hook *ChainHook `json:"hook"`
	// Priority orders the chain relative to Netfilter internal operations.
	// +kubebuilder:default=0
	Priority *ChainPriority `json:"priority"`
}
