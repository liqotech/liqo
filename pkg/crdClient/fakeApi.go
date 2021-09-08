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

func (c *FakeClient) Get(name string, _ *metav1.GetOptions) (runtime.Object, error) {
	result, found, err := c.storage.GetByKey(name)
	if !found {
		return nil, kerrors.NewNotFound(c.storage.groupResource, name)
	}

	return result.(runtime.Object), err
}

func (c *FakeClient) List(_ *metav1.ListOptions) (runtime.Object, error) {
	panic("to implement")
}

func (c *FakeClient) Watch(_ *metav1.ListOptions) (watch.Interface, error) {
	return c.storage.watcher, nil
}

func (c *FakeClient) Create(obj runtime.Object, _ *metav1.CreateOptions) (runtime.Object, error) {
	err := c.storage.Add(obj)
	if err != nil {
		return nil, err
	}

	return obj, nil
}

func (c *FakeClient) Delete(name string, opts *metav1.DeleteOptions) error {
	obj, err := c.Get(name, &metav1.GetOptions{})
	if err != nil {
		return err
	}
	err = c.storage.Delete(obj)
	if err != nil {
		return err
	}
	return nil
}

func (c *FakeClient) Update(name string, obj runtime.Object, _ *metav1.UpdateOptions) (runtime.Object, error) {
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

func (c *FakeClient) UpdateStatus(name string, obj runtime.Object, opts *metav1.UpdateOptions) (runtime.Object, error) {
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
