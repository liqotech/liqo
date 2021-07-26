package test

import (
	corev1 "k8s.io/api/core/v1"

	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	"github.com/liqotech/liqo/pkg/virtualKubelet/storage"
)

type MockController struct {
	Manager storage.CacheManagerReaderAdder
}

// SetInformingFunc implementation.
func (m MockController) SetInformingFunc(apiType apimgmt.ApiType, f func(*corev1.Pod)) {
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
