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

package crdclient

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

// NewFakeCustomInformer creates a new FakeCustomInformer, registers the callbacks
// and start the watching routine that implements the caching functionality
// and the callbak notifications.
func NewFakeCustomInformer(handlers cache.ResourceEventHandlerFuncs,
	keyer KeyerFunc,
	groupResource schema.GroupResource) (cache.Store, chan struct{}) {
	i := &fakeInformer{
		FakeCustomStore: cache.FakeCustomStore{},
		funcs:           handlers,
		keyer:           keyer,
		data:            make(map[string]runtime.Object),
		groupResource:   groupResource,
	}

	i.AddFunc = i.AddFake
	i.UpdateFunc = i.UpdateFake
	i.DeleteFunc = i.DeleteFake
	i.ListFunc = i.ListFake
	i.ListKeysFunc = i.ListKeysFake
	i.GetFunc = i.GetFake
	i.GetByKeyFunc = i.GetByKeyFake
	i.ReplaceFunc = i.ReplaceFake
	i.ResyncFunc = i.ResyncFake

	i.Watch()
	return i, make(chan struct{}, 1)
}
