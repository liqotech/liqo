package test

import (
	"errors"
	"github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
)

type MockController struct {
	cache map[string]interface{}
}

func (c *MockController) GetMirroredObjectByKey(api apiReflection.ApiType, namespace string, name string) interface{} {
	return nil
}

func (c *MockController) GetMirroringObjectByKey(api apiReflection.ApiType, namespace string, name string) (interface{}, error) {
	if c.cache == nil {
		c.cache = map[string]interface{}{}
	}
	obj, ok := c.cache[name]
	if !ok {
		return nil, errors.New("not found")
	}
	return obj, nil
}

func (c *MockController) ListMirroredObjects(api apiReflection.ApiType, namespace string) []interface{} {
	return nil
}

func (c *MockController) AddMirroringObject(object interface{}, name string) {
	if c.cache == nil {
		c.cache = map[string]interface{}{}
	}
	c.cache[name] = object
}
