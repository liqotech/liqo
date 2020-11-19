package utils

import (
	"fmt"
	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
)

func Keyer(namespace, name string) string {
	return fmt.Sprintf("%v/%v", namespace, name)
}

func GetObject(informer cache.SharedIndexInformer, key string, backoff wait.Backoff) (interface{}, error) {
	if informer == nil {
		return nil, errors.New("informer not instantiated")
	}

	var object interface{}

	fn := func() error {
		obj, exists, err := informer.GetIndexer().GetByKey(key)
		if err != nil {
			return errors.Wrap(err, "error while getting by key object from foreign cache")
		}
		if !exists {
			return kerrors.NewNotFound(schema.GroupResource{}, key)
		}
		object = obj
		return nil
	}

	retriable := func(err error) bool {
		return kerrors.IsNotFound(err)
	}

	if err := retry.OnError(backoff, retriable, fn); err != nil {
		return nil, err
	}

	return object, nil
}

func ListIndexedObjects(informer cache.SharedIndexInformer, api, key string) ([]interface{}, error) {
	if informer == nil {
		return nil, errors.New("informer not instantiated")
	}

	objects, err := informer.GetIndexer().ByIndex(api, key)
	if err != nil {
		return nil, err
	}

	return objects, nil
}

func ListObjects(informer cache.SharedIndexInformer) ([]interface{}, error) {
	if informer == nil {
		return nil, errors.New("informer not instantiated")
	}

	return informer.GetIndexer().List(), nil
}

func ResyncListObjects(informer cache.SharedIndexInformer) ([]interface{}, error) {
	if informer == nil {
		return nil, errors.New("informer not instantiated")
	}

	// resync for ensuring to be remotely aligned with the foreign cluster state
	err := informer.GetIndexer().Resync()
	if err != nil {
		return nil, err
	}

	return informer.GetIndexer().List(), nil
}
