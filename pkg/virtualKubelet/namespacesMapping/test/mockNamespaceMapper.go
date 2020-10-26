package test

import (
	"errors"
)

type MockNamespaceMapper struct {
	cache map[string]string
}

func (m *MockNamespaceMapper) NatNamespace(namespace string, create bool) (string, error) {
	if ns, ok := m.cache[namespace]; !ok {
		if create {
			m.cache[namespace] = namespace + "-natted"
			return m.cache[namespace], nil
		}
		return "", errors.New("not found")
	} else {
		return ns, nil
	}
}

func (m *MockNamespaceMapper) DeNatNamespace(namespace string) (string, error) {
	for k, v := range m.cache {
		if v == namespace {
			return k, nil
		}
	}
	return "", errors.New("not found")
}
