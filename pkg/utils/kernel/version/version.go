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

package version

import (
	"errors"
	"fmt"

	"github.com/spf13/pflag"
)

var _ pflag.Value = &KernelVersion{}

// KernelVersion holds information about the kernel.
type KernelVersion struct {
	Kernel int
	Major  int
	Minor  int
	Flavor string
}

// String returns the string representation of the KernelVersion.
func (kv *KernelVersion) String() string {
	return fmt.Sprintf("%d.%d.%d%s", kv.Kernel, kv.Major, kv.Minor, kv.Flavor)
}

// Set parses a string and sets the KernelVersion.
func (kv *KernelVersion) Set(s string) error {
	newvi, err := ParseRelease(s)
	if err != nil {
		return err
	}
	kv.Kernel = newvi.Kernel
	kv.Major = newvi.Major
	kv.Minor = newvi.Minor
	kv.Flavor = newvi.Flavor

	return nil
}

// Type returns the type of the KernelVersion.
func (kv *KernelVersion) Type() string {
	return "string"
}

// CheckRequirements checks if the kernel version is compatible with the requirements.
func (kv *KernelVersion) CheckRequirements(minimumkv *KernelVersion) bool {
	return Compare(kv, minimumkv) >= 0
}

// Compare compares two kernel.VersionInfo structs.
// Returns -1 if a < b, 0 if a == b, 1 it a > b.
func Compare(a, b *KernelVersion) int {
	if a.Kernel < b.Kernel {
		return -1
	} else if a.Kernel > b.Kernel {
		return 1
	}

	if a.Major < b.Major {
		return -1
	} else if a.Major > b.Major {
		return 1
	}

	if a.Minor < b.Minor {
		return -1
	} else if a.Minor > b.Minor {
		return 1
	}

	return 0
}

// ParseRelease parses a string and creates a VersionInfo based on it.
func ParseRelease(release string) (*KernelVersion, error) {
	var (
		kernel, major, minor, parsed int
		flavor, partial              string
	)

	// Ignore error from Sscanf to allow an empty flavor.  Instead, just
	// make sure we got all the version numbers.
	parsed, _ = fmt.Sscanf(release, "%d.%d%s", &kernel, &major, &partial)
	if parsed < 2 {
		return nil, errors.New("Can't parse kernel version " + release)
	}

	// sometimes we have 3.12.25-gentoo, but sometimes we just have 3.12-1-amd64
	parsed, _ = fmt.Sscanf(partial, ".%d%s", &minor, &flavor)
	if parsed < 1 {
		flavor = partial
	}

	return &KernelVersion{
		Kernel: kernel,
		Major:  major,
		Minor:  minor,
		Flavor: flavor,
	}, nil
}
