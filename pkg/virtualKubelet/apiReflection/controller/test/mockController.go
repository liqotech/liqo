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
