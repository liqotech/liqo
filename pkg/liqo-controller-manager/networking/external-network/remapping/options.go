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

package remapping

import (
	"fmt"

	"github.com/liqotech/liqo/pkg/utils/network"
)

// Options contains the options for the remapping controller.
type Options struct {
	// DefaultInterfaceName is the name of the interface where the default rout points in main table.
	DefaultInterfaceName string
}

// NewOptions returns a new Options struct.
func NewOptions() (*Options, error) {
	// We assumes that the default interface created by the CNI inside a pod, is the same for each pod.
	defaultInterfaceName, err := network.GetDefaultInterfaceName()
	if err != nil {
		return nil, fmt.Errorf("cannot get the default interface name: %w", err)
	}
	return &Options{
		DefaultInterfaceName: defaultInterfaceName,
	}, nil
}
