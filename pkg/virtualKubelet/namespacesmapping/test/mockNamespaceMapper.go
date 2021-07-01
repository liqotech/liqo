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
