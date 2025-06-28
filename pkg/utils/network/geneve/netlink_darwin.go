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

package geneve

// EnsureGeneveInterfacePresence is a stub for macOS.
func EnsureGeneveInterfacePresence(_, _, _ string, _ uint32, _ bool, _ int, _ uint16) error {
	panic("EnsureGeneveInterfacePresence is not supported on darwin")
}

// EnsureGeneveInterfaceAbsence is a stub for macOS.
func EnsureGeneveInterfaceAbsence(_ string) error {
	panic("EnsureGeneveInterfaceAbsence is not supported on darwin")
}

// ForgeGeneveInterface is a stub for macOS.
func ForgeGeneveInterface(_ string, _ interface{}, _ uint32, _ int, _ uint16) interface{} {
	panic("ForgeGeneveInterface is not supported on darwin")
}

// CreateGeneveInterface is a stub for macOS.
func CreateGeneveInterface(_ string, _ interface{}, _ interface{}, _ uint32, _ bool, _ int, _ uint16) error {
	panic("CreateGeneveInterface is not supported on darwin")
}

// ExistGeneveInterface is a stub for macOS.
func ExistGeneveInterface(_ string) interface{} {
	panic("ExistGeneveInterface is not supported on darwin")
}

// ExistGeneveInterfaceAddr is a stub for macOS.
func ExistGeneveInterfaceAddr(_ interface{}, _ interface{}) interface{} {
	panic("ExistGeneveInterfaceAddr is not supported on darwin")
}

// ListGeneveInterfaces is a stub for macOS.
func ListGeneveInterfaces() ([]interface{}, error) {
	panic("ListGeneveInterfaces is not supported on darwin")
}
