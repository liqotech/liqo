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

package fake

// NamespaceStartStopper is a fake manager.NamespaceStartStopper.
type NamespaceStartStopper struct {
	StartNamespaceCalled        int
	StartNamespaceArgumentsCall map[string]string

	StopNamespaceCalled        int
	StopNamespaceArgumentsCall map[string]string
}

// NewNamespaceStartStopper creates a new NamespaceManager.
func NewNamespaceStartStopper() *NamespaceStartStopper {
	return &NamespaceStartStopper{
		StartNamespaceArgumentsCall: map[string]string{},
		StopNamespaceArgumentsCall:  map[string]string{},
	}
}

// StartNamespace starts the reflection for a given namespace.
func (nm *NamespaceStartStopper) StartNamespace(local, remote string) {
	nm.StartNamespaceCalled++
	nm.StartNamespaceArgumentsCall[local] = remote
}

// StopNamespace stops the reflection for a given namespace.
func (nm *NamespaceStartStopper) StopNamespace(local, remote string) {
	nm.StopNamespaceCalled++
	nm.StopNamespaceArgumentsCall[local] = remote
}
