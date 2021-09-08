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
	"github.com/pkg/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"

	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
)

type MockManager struct {
	HomeCache    map[string]map[apimgmt.ApiType]map[string]v1.Object
	ForeignCache map[string]map[apimgmt.ApiType]map[string]v1.Object
}

func (m *MockManager) CheckNamespaceCaching(_ *wait.Backoff, _ string, _ string, _ apimgmt.ApiType) error {
	return nil
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

// GetHomeAPIByIndex is a mock implementation of the corresponding function (unimplemented).
func (m *MockManager) GetHomeAPIByIndex(apiType apimgmt.ApiType, s, s2 string) (interface{}, error) {
	panic("implement me")
}

// GetForeignAPIByIndex is a mock implementation of the corresponding function (unimplemented).
func (m *MockManager) GetForeignAPIByIndex(apiType apimgmt.ApiType, s, s2 string) (interface{}, error) {
	panic("implement me")
}

// Clear is a function used in tests only to clear the mock's state.
func (m *MockManager) Clear() {
	for k := range m.HomeCache {
		delete(m.HomeCache, k)
	}

	for k := range m.ForeignCache {
		delete(m.ForeignCache, k)
	}
}
