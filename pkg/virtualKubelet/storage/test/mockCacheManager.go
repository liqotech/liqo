package test

import (
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	"github.com/pkg/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

type MockManager struct {
	HomeCache    map[string]map[apimgmt.ApiType]map[string]v1.Object
	ForeignCache map[string]map[apimgmt.ApiType]map[string]v1.Object
}

func (m *MockManager) AddHomeEntry(namespace string, api apimgmt.ApiType, obj v1.Object) {
	if m.HomeCache == nil {
		m.HomeCache = map[string]map[apimgmt.ApiType]map[string]v1.Object{}
	}
	if m.HomeCache[namespace] == nil {
		m.HomeCache[namespace] = map[apimgmt.ApiType]map[string]v1.Object{}
	}
	if m.HomeCache[namespace][api] == nil {
		m.HomeCache[namespace][api] = map[string]v1.Object{}
	}
	m.HomeCache[namespace][api][obj.GetName()] = obj
}

func (m *MockManager) AddForeignEntry(namespace string, api apimgmt.ApiType, obj v1.Object) {
	if m.ForeignCache == nil {
		m.ForeignCache = map[string]map[apimgmt.ApiType]map[string]v1.Object{}
	}
	if m.ForeignCache[namespace] == nil {
		m.ForeignCache[namespace] = map[apimgmt.ApiType]map[string]v1.Object{}
	}
	if m.ForeignCache[namespace][api] == nil {
		m.ForeignCache[namespace][api] = map[string]v1.Object{}
	}
	m.ForeignCache[namespace][api][obj.GetName()] = obj
}

func (m *MockManager) AddHomeNamespace(s string) error {
	m.HomeCache[s] = make(map[apimgmt.ApiType]map[string]v1.Object)
	return nil
}

func (m *MockManager) AddForeignNamespace(s string) error {
	m.ForeignCache[s] = make(map[apimgmt.ApiType]map[string]v1.Object)
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
	obj := m.HomeCache[s][apiType][s2]
	if obj == nil {
		return nil, errors.New("object not found")
	}

	return obj, nil
}

func (m *MockManager) GetForeignNamespacedObject(apiType apimgmt.ApiType, s string, s2 string) (interface{}, error) {
	obj := m.ForeignCache[s][apiType][s2]
	if obj == nil {
		return nil, errors.New("object not found")
	}

	return obj, nil
}

func (m *MockManager) ListHomeNamespacedObject(apiType apimgmt.ApiType, s string) ([]interface{}, error) {
	res := []interface{}{}
	for _, v := range m.HomeCache[s][apiType] {
		res = append(res, v)
	}
	return res, nil
}

func (m *MockManager) ListForeignNamespacedObject(apiType apimgmt.ApiType, s string) ([]interface{}, error) {
	res := []interface{}{}
	for _, v := range m.ForeignCache[s][apiType] {
		res = append(res, v)
	}
	return res, nil
}

func (m *MockManager) ResyncListHomeNamespacedObject(apiType apimgmt.ApiType, s string) ([]interface{}, error) {
	panic("implement me")
}

func (m *MockManager) ResyncListForeignNamespacedObject(apiType apimgmt.ApiType, s string) ([]interface{}, error) {
	panic("implement me")
}

func (m *MockManager) ListHomeApiByIndex(apiType apimgmt.ApiType, s string, s2 string) ([]interface{}, error) {
	panic("implement me")
}

func (m *MockManager) ListForeignApiByIndex(apiType apimgmt.ApiType, s string, s2 string) ([]interface{}, error) {
	panic("implement me")
}
