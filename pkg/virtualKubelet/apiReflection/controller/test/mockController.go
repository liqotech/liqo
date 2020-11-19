package test

import (
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	"github.com/liqotech/liqo/pkg/virtualKubelet/storage"
)

type MockController struct {
	Manager storage.CacheManagerReaderAdder
}

func (m MockController) SetInformingFunc(apiType apimgmt.ApiType, f func(interface{})) {
	panic("implement me")
}

func (m MockController) CacheManager() storage.CacheManagerReaderAdder {
	return m.Manager
}

func (m MockController) StartController() {
	panic("implement me")
}

func (m MockController) StopController() error {
	panic("implement me")
}

func (m MockController) StopReflection(restart bool) {
	panic("implement me")
}
