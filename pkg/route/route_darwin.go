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

// EnsureRoutesPresence is a stub for macOS.
func EnsureRoutesPresence(_ []networkingv1beta1.Route, _ uint32) error {
	panic("EnsureRoutesPresence is not supported on darwin")
}

// EnsureRoutesAbsence is a stub for macOS.
func EnsureRoutesAbsence(_ []networkingv1beta1.Route, _ uint32) error {
	panic("EnsureRoutesAbsence is not supported on darwin")
}

// ExistsRoute is a stub for macOS.
func ExistsRoute(_ *networkingv1beta1.Route, _ uint32) (interface{}, bool, error) {
	panic("ExistsRoute is not supported on darwin")
}

// IsEqualRoute is a stub for macOS.
func IsEqualRoute(_, _ interface{}) bool {
	panic("IsEqualRoute is not supported on darwin")
}

// CleanRoutes is a stub for macOS.
func CleanRoutes(_ []networkingv1beta1.Route, _ uint32) error {
	panic("CleanRoutes is not supported on darwin")
}

// IsContainedRoute is a stub for macOS.
func IsContainedRoute(_ interface{}, _ []networkingv1beta1.Route) bool {
	panic("IsContainedRoute is not supported on darwin")
}
