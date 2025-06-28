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

package version

// CheckKernelVersion checks if the current kernel version satisfies the minimum requirements.
func CheckKernelVersion(_ *KernelVersion) error {
	panic("CheckKernelVersion is not implemented for darwin, use CheckKernelVersionFromNodes instead")
}

// GetKernelVersion gets the current kernel version.
func GetKernelVersion() (*KernelVersion, error) {
	panic("GetKernelVersion is not implemented for darwin, use GetKernelVersionFromNode instead")
}
