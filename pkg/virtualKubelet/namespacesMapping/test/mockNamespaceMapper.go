package test

import (
	"errors"
)

type MockNamespaceMapper struct {
	Cache map[string]string
}

func (m *MockNamespaceMapper) NatNamespace(namespace string, create bool) (string, error) {
	if ns, ok := m.Cache[namespace]; !ok {
		if create {
			m.Cache[namespace] = namespace + "-natted"
			return m.Cache[namespace], nil
		}
		return "", errors.New("not found")
	} else {
		return ns, nil
	}
}

func (m *MockNamespaceMapper) DeNatNamespace(namespace string) (string, error) {
	for k, v := range m.Cache {
		if v == namespace {
			return k, nil
		}
	}
	return "", errors.New("not found")
}
