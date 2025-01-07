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

//go:build linux || freebsd || openbsd

package version

import (
	"fmt"

	"golang.org/x/sys/unix"
	"k8s.io/klog/v2"
)

// CheckKernelVersion checks if the current kernel version satisfies the minimum requirements.
func CheckKernelVersion(minimum *KernelVersion) error {
	current, err := GetKernelVersion()
	if err != nil {
		return fmt.Errorf("failed to get the current kernel version: %w", err)
	}

	if !current.CheckRequirements(minimum) {
		return fmt.Errorf("kernel version %s does not satisfy the minimum requirements %s", current.String(), minimum.String())
	}

	klog.Infof("Kernel version %s satisfies the minimum requirements %s", current.String(), minimum.String())
	return nil
}

// GetKernelVersion gets the current kernel version.
func GetKernelVersion() (*KernelVersion, error) {
	uts, err := uname()
	if err != nil {
		return nil, err
	}

	// Remove the \x00 from the release for Atoi to parse correctly
	return ParseRelease(unix.ByteSliceToString(uts.Release[:]))
}

func uname() (*unix.Utsname, error) {
	uts := &unix.Utsname{}

	if err := unix.Uname(uts); err != nil {
		return nil, err
	}
	return uts, nil
}
