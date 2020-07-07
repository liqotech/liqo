package crdClient

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

// NewFakeCustomInformer creates a new FakeCustomInformer, registers the callbacks
// and start the watching routine that implements the caching functionality
// and the callbak notifications
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
