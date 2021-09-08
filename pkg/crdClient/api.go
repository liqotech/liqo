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
	"context"
	"reflect"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

type Client struct {
	Client rest.Interface

	api      string
	resource RegistryType
	ns       string

	storage cache.Store
}

func (c *Client) Namespace(namespace string) CrdClientInterface {
	c.ns = namespace
	return c
}

func (c *Client) Get(name string, opts *metav1.GetOptions) (runtime.Object, error) {
	result := reflect.New(c.resource.SingularType).Interface()
	var namespaced bool
	if c.ns != "" {
		namespaced = true
	}

	err := c.Client.
		Get().
		Resource(c.api).
		VersionedParams(opts, scheme.ParameterCodec).
		NamespaceIfScoped(c.ns, namespaced).
		Name(name).
		Do(context.TODO()).
		Into(result.(runtime.Object))

	return result.(runtime.Object), err
}

func (c *Client) List(opts *metav1.ListOptions) (runtime.Object, error) {
	result := reflect.New(c.resource.PluralType).Interface()
	var namespaced bool
	if c.ns != "" {
		namespaced = true
	}

	err := c.Client.
		Get().
		Resource(c.api).
		VersionedParams(opts, scheme.ParameterCodec).
		NamespaceIfScoped(c.ns, namespaced).
		Do(context.TODO()).
		Into(result.(runtime.Object))

	return result.(runtime.Object), err
}

func (c *Client) Watch(opts *metav1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true

	var namespaced bool
	if c.ns != "" {
		namespaced = true
	}

	return c.Client.
		Get().
		Resource(c.api).
		VersionedParams(opts, scheme.ParameterCodec).
		NamespaceIfScoped(c.ns, namespaced).
		Timeout(timeout).
		Watch(context.TODO())
}

func (c *Client) Create(obj runtime.Object, opts *metav1.CreateOptions) (runtime.Object, error) {
	result := reflect.New(c.resource.SingularType).Interface()

	var namespaced bool
	if c.ns != "" {
		namespaced = true
	}

	err := c.Client.
		Post().
		Resource(c.api).
		VersionedParams(opts, scheme.ParameterCodec).
		NamespaceIfScoped(c.ns, namespaced).
		Body(obj).
		Do(context.TODO()).
		Into(result.(runtime.Object))

	return result.(runtime.Object), err
}

func (c *Client) Delete(name string, opts *metav1.DeleteOptions) error {
	var namespaced bool
	if c.ns != "" {
		namespaced = true
	}

	return c.Client.
		Delete().
		Resource(c.api).
		VersionedParams(opts, scheme.ParameterCodec).
		NamespaceIfScoped(c.ns, namespaced).
		Name(name).
		Body(opts).
		Do(context.TODO()).
		Error()
}

func (c *Client) Update(name string, obj runtime.Object, opts *metav1.UpdateOptions) (runtime.Object, error) {
	result := reflect.New(c.resource.SingularType).Interface()

	var namespaced bool
	if c.ns != "" {
		namespaced = true
	}

	err := c.Client.Put().
		Resource(c.api).
		VersionedParams(opts, scheme.ParameterCodec).
		NamespaceIfScoped(c.ns, namespaced).
		Name(name).
		Body(obj).
		Do(context.TODO()).
		Into(result.(runtime.Object))

	return result.(runtime.Object), err
}

func (c *Client) UpdateStatus(name string, obj runtime.Object, opts *metav1.UpdateOptions) (runtime.Object, error) {
	result := reflect.New(c.resource.SingularType).Interface()

	var namespaced bool
	if c.ns != "" {
		namespaced = true
	}

	err := c.Client.Put().
		Resource(c.api).
		VersionedParams(opts, scheme.ParameterCodec).
		NamespaceIfScoped(c.ns, namespaced).
		Name(name).
		SubResource("status").
		Body(obj).
		Do(context.TODO()).
		Into(result.(runtime.Object))

	return result.(runtime.Object), err
}
