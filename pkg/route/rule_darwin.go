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

//go:build darwin
// +build darwin

package route

import (
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
)

// EnsureRulePresence is a stub for macOS.
func EnsureRulePresence(_ *networkingv1beta1.Rule, _ uint32) error {
	panic("EnsureRulePresence is not supported on darwin")
}

// EnsureRuleAbsence is a stub for macOS.
func EnsureRuleAbsence(_ *networkingv1beta1.Rule, _ uint32) error {
	panic("EnsureRuleAbsence is not supported on darwin")
}

// AddRule is a stub for macOS.
func AddRule(_ *networkingv1beta1.Rule, _ uint32) error {
	panic("AddRule is not supported on darwin")
}

// GetRulesByTableID is a stub for macOS.
func GetRulesByTableID(_ uint32) ([]interface{}, error) {
	panic("GetRulesByTableID is not supported on darwin")
}

// ExistsRule is a stub for macOS.
func ExistsRule(_ *networkingv1beta1.Rule, _ []interface{}) (interface{}, bool, error) {
	panic("ExistsRule is not supported on darwin")
}

// RuleIsEqual is a stub for macOS.
func RuleIsEqual(_ *networkingv1beta1.Rule, _ interface{}) bool {
	panic("RuleIsEqual is not supported on darwin")
}

// CleanRules is a stub for macOS.
func CleanRules(_ []networkingv1beta1.Rule, _ uint32) error {
	panic("CleanRules is not supported on darwin")
}

// IsContainedRule is a stub for macOS.
func IsContainedRule(_ interface{}, _ []networkingv1beta1.Rule) bool {
	panic("IsContainedRule is not supported on darwin")
}
