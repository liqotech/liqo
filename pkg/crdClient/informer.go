package crdClient

import (
	clientv1alpha1 "github.com/liqoTech/liqo/pkg/crdClient/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"reflect"
	"time"
)

func WatchResources(clientSet clientv1alpha1.NamespacedCRDClientInterface,
	resource, namespace string,
	resyncPeriod time.Duration,
	handlers cache.ResourceEventHandlerFuncs,
	lo metav1.ListOptions) (cache.Store, chan struct{}) {

	listFunc := func(ls metav1.ListOptions) (result runtime.Object, err error) {
		ls = lo
		return clientSet.Resource(resource).Namespace(namespace).List(ls)
	}

	watchFunc := func(ls metav1.ListOptions) (watch.Interface, error) {
		ls = lo
		return clientSet.Resource(resource).Namespace(namespace).Watch(ls)
	}

	store, controller := cache.NewInformer(
		&cache.ListWatch{
			ListFunc:  listFunc,
			WatchFunc: watchFunc,
		},
		reflect.New(clientv1alpha1.Registry[resource].SingularType).Interface().(runtime.Object),
		resyncPeriod,
		handlers,
	)

	stopChan := make(chan struct{}, 1)

	go controller.Run(stopChan)

	return store, stopChan
}
