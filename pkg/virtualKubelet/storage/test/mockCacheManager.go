package test

import (
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	"github.com/pkg/errors"
	"k8s.io/client-go/tools/cache"
)

type MockManager struct {
	HomeCache    map[string]map[apimgmt.ApiType]interface{}
	ForeignCache map[string]map[apimgmt.ApiType]interface{}
}

func (m *MockManager) AddHomeEntry(namespace string, api apimgmt.ApiType, obj interface{}) {
	m.HomeCache[namespace][api] = obj
}

func (m *MockManager) AddForeignEntry(namespace string, api apimgmt.ApiType, obj interface{}) {
	m.ForeignCache[namespace][api] = obj
}

func (m *MockManager) AddHomeNamespace(s string) error {
	m.HomeCache[s] = make(map[apimgmt.ApiType]interface{})
	return nil
}

func (m *MockManager) AddForeignNamespace(s string) error {
	m.ForeignCache[s] = make(map[apimgmt.ApiType]interface{})
	return nil
}

func (m *MockManager) StartHomeNamespace(s string, c chan struct{}) error {
	panic("implement me")
}

func (m *MockManager) StartForeignNamespace(s string, c chan struct{}) error {
	panic("implement me")
}

func (m *MockManager) RemoveNamespace(s string) {
	panic("implement me")
}

func (m *MockManager) AddHomeEventHandlers(apiType apimgmt.ApiType, s string, funcs *cache.ResourceEventHandlerFuncs) error {
	panic("implement me")
}

func (m *MockManager) AddForeignEventHandlers(apiType apimgmt.ApiType, s string, funcs *cache.ResourceEventHandlerFuncs) error {
	panic("implement me")
}

func (m *MockManager) GetHomeNamespacedObject(apiType apimgmt.ApiType, s string, s2 string) (interface{}, error) {
	obj := m.HomeCache[s][apiType]
	if obj == nil {
		return nil, errors.New("object not found")
	}

	return obj, nil
}

func (m *MockManager) GetForeignNamespacedObject(apiType apimgmt.ApiType, s string, s2 string) (interface{}, error) {
	panic("implement me")
}

func (m *MockManager) ListHomeNamespacedObject(apiType apimgmt.ApiType, s string) ([]interface{}, error) {
	panic("implement me")
}

func (m *MockManager) ListForeignNamespacedObject(apiType apimgmt.ApiType, s string) ([]interface{}, error) {
	panic("implement me")
}

func (m *MockManager) ResyncListHomeNamespacedObject(apiType apimgmt.ApiType, s string) ([]interface{}, error) {
	panic("implement me")
}

func (m *MockManager) ResyncListForeignNamespacedObject(apiType apimgmt.ApiType, s string) ([]interface{}, error) {
	panic("implement me")
}
