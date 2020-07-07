package crdClient

import (
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
)

type FakeClient struct {
	Client rest.Interface

	api      string
	resource RegistryType
	ns       string

	storage *fakeInformer
}

func (c *FakeClient) Namespace(namespace string) CrdClientInterface {
	c.ns = namespace
	return c
}

func (c *FakeClient) Get(name string, _ metav1.GetOptions) (runtime.Object, error) {

	result, found, err := c.storage.GetByKey(name)
	if !found {
		return nil, kerrors.NewNotFound(c.storage.groupResource, name)
	}

	return result.(runtime.Object), err
}

func (c *FakeClient) List(_ metav1.ListOptions) (runtime.Object, error) {
	panic("to implement")
}

func (c *FakeClient) Watch(_ metav1.ListOptions) (watch.Interface, error) {
	return c.storage.watcher, nil
}

func (c *FakeClient) Create(obj runtime.Object, _ metav1.CreateOptions) (runtime.Object, error) {
	err := c.storage.Add(obj)
	if err != nil {
		return nil, err
	}

	return obj, nil
}

func (c *FakeClient) Delete(name string, opts metav1.DeleteOptions) error {
	panic("to implement")
}

func (c *FakeClient) Update(name string, obj runtime.Object, _ metav1.UpdateOptions) (runtime.Object, error) {
	err := c.storage.Update(obj)
	if err != nil {
		return nil, err
	}

	result, found, err := c.storage.GetByKey(name)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, kerrors.NewNotFound(c.storage.groupResource, name)
	}

	return result.(runtime.Object), nil
}

func (c *FakeClient) UpdateStatus(name string, obj runtime.Object, opts metav1.UpdateOptions) (runtime.Object, error) {
	panic("to implement")
}
