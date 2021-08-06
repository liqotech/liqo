package crdclient

import (
	"errors"
	"sync"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// FakeInformer is an implementation of the cache.FakeCustomStore that
// allows to trigger some callbacks when CRUD events occur.
type fakeInformer struct {
	cache.FakeCustomStore
	funcs cache.ResourceEventHandlerFuncs

	// KeyerFromObj is a function that allows to create a key given a generic runtime.Object
	// The API should implement the keyer interface
	keyer KeyerFunc

	data          map[string]runtime.Object
	lock          sync.Mutex
	cacheWatcher  *watch.RaceFreeFakeWatcher
	watcher       *watch.RaceFreeFakeWatcher
	groupResource schema.GroupResource
}

func (i *fakeInformer) AddFake(obj interface{}) error {
	if i.keyer == nil {
		return errors.New("keyer function not set")
	}
	k, err := i.keyer(obj.(runtime.Object))
	if err != nil {
		return err
	}

	i.lock.Lock()
	i.data[k] = obj.(runtime.Object)
	i.lock.Unlock()

	i.cacheWatcher.Add(obj.(runtime.Object))

	return nil
}

func (i *fakeInformer) UpdateFake(obj interface{}) error {
	if i.keyer == nil {
		return errors.New("keyer function not set")
	}
	k, err := i.keyer(obj.(runtime.Object))
	if err != nil {
		return err
	}

	i.lock.Lock()
	defer i.lock.Unlock()
	old, ok := i.data[k]
	if !ok {
		return kerrors.NewNotFound(i.groupResource, k)
	}
	i.data[k] = obj.(runtime.Object)

	i.cacheWatcher.Modify(old)

	return nil
}

func (i *fakeInformer) DeleteFake(obj interface{}) error {
	if i.keyer == nil {
		return errors.New("keyer function not set")
	}
	k, err := i.keyer(obj.(runtime.Object))
	if err != nil {
		return err
	}

	i.lock.Lock()
	delete(i.data, k)
	i.lock.Unlock()

	i.cacheWatcher.Delete(obj.(runtime.Object))

	return nil
}

func (i *fakeInformer) ListFake() []interface{} {
	i.lock.Lock()
	items := make([]interface{}, 0, len(i.data))
	for _, v := range i.data {
		items = append(items, v)
	}
	i.lock.Unlock()

	return items
}

func (i *fakeInformer) ListKeysFake() []string {
	panic("to implement")
}

func (i *fakeInformer) GetFake(obj interface{}) (item interface{}, exists bool, err error) {
	panic("to implement")
}

func (i *fakeInformer) GetByKeyFake(key string) (item interface{}, exists bool, err error) {
	i.lock.Lock()
	v, ok := i.data[key]
	i.lock.Unlock()

	if !ok {
		return nil, false, kerrors.NewNotFound(i.groupResource, key)
	}

	return v, true, nil
}

func (i *fakeInformer) ReplaceFake(list []interface{}, resourceVersion string) error {
	panic("to implement")
}

func (i *fakeInformer) ResyncFake() error {
	panic("to implement")
}

func (i *fakeInformer) Watch() {
	w := watch.NewRaceFreeFake()
	w2 := watch.NewRaceFreeFake()
	i.cacheWatcher = w
	i.watcher = w2
	go func() {
		for e := range w.ResultChan() {
			switch e.Type {
			case watch.Added:
				if i.funcs.AddFunc != nil {
					i.funcs.AddFunc(e.Object)
				}
				i.watcher.Add(e.Object)
			case watch.Deleted:
				if i.funcs.DeleteFunc != nil {
					i.funcs.DeleteFunc(e.Object)
				}
				i.watcher.Delete(e.Object)
			case watch.Modified:
				if i.keyer == nil {
					klog.Fatal("keyer function not set")
				}
				k, err := i.keyer(e.Object)
				if err != nil {
					klog.Fatal(err)
				}
				i.lock.Lock()
				newObj, ok := i.data[k]
				if !ok {
					klog.Fatal(err)
				}
				i.lock.Unlock()
				if i.funcs.UpdateFunc != nil {
					i.funcs.UpdateFunc(e.Object, newObj)
				}
				i.watcher.Modify(newObj)
			}
		}
	}()
}
