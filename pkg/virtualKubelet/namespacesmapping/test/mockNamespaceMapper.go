// Copyright 2019-2021 The Liqo Authors
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

package test

import (
	"errors"
)

// MockNamespaceMapper implements a mock namespace mapping mechanism.
type MockNamespaceMapper struct {
	Cache map[string]string
}

// NatNamespace handle the home to foreign namespace translation. It returns an error if the mapping is not found.
func (m *MockNamespaceMapper) NatNamespace(namespace string) (string, error) {
	ns, ok := m.Cache[namespace]
	if !ok {
		return "", errors.New("not found")
	}
	return ns, nil
}

// DeNatNamespace handle the foreign to home namespace translation. It returns an error if the mapping is not found.
func (m *MockNamespaceMapper) DeNatNamespace(namespace string) (string, error) {
	for k, v := range m.Cache {
		if v == namespace {
			return k, nil
		}
	}
	return "", errors.New("not found")
}

// MappedNamespaces returns the entire namespace mapping map.
func (m *MockNamespaceMapper) MappedNamespaces() map[string]string {
	panic("implement me")
}

// NewNamespace creates a new namespace in the local cache.
func (m *MockNamespaceMapper) NewNamespace(namespace string) {
	m.Cache[namespace] = namespace + "-natted"
}

// Clear is a function used in tests only to clear the mock's state.
func (m *MockNamespaceMapper) Clear() {
	for k := range m.Cache {
		delete(m.Cache, k)
	}
}
