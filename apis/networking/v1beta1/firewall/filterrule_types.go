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

// FilterAction is the action to be applied to the rule.
type FilterAction string

const (
	// ActionCtMark is the action to be applied to the rule.
	// It is used to mark the connection using the conntrack.
	ActionCtMark FilterAction = "ctmark"
	// ActionSetMetaMarkFromCtMark is the action to be applied to the rule.
	// It is used to set the meta mark from the conntrack mark.
	ActionSetMetaMarkFromCtMark FilterAction = "metamarkfromctmark"
)

// FilterRule is a rule to be applied to a filter chain.
// +kubebuilder:object:generate=true
type FilterRule struct {
	// Name is the name of the rule.
	Name *string `json:"name,omitempty"`
	// Match is the match to be applied to the rule.
	// They can be multiple and they are applied with an AND operator.
	Match []Match `json:"match"`
	// Action is the action to be applied to the rule.
	// +kubebuilder:validation:Enum=ctmark;metamarkfromctmark
	Action FilterAction `json:"action"`
	// Value is the value to be used for the action.
	Value *string `json:"value,omitempty"`
}
