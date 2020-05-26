package crdClient

import (
	clientv1alpha1 "github.com/netgroup-polito/dronev2/pkg/crdClient/v1alpha1"
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
	handlers cache.ResourceEventHandlerFuncs) (cache.Store, chan struct{}) {
	store, controller := cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(lo metav1.ListOptions) (result runtime.Object, err error) {
				return clientSet.NamespacedCRDClient(namespace).List(resource, lo)
			},
			WatchFunc: func(lo metav1.ListOptions) (watch.Interface, error) {
				return clientSet.NamespacedCRDClient(namespace).Watch(resource, lo)
			},
		},
		reflect.New(clientv1alpha1.Registry[resource]).Interface().(runtime.Object),
		resyncPeriod,
		handlers,
	)

	stopChan := make(chan struct{}, 1)

	go controller.Run(stopChan)

	return store, stopChan
}
