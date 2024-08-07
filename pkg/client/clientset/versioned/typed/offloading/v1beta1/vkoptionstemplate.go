// Copyright 2019-2024 The Liqo Authors
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

// Code generated by client-gen. DO NOT EDIT.

package v1beta1

import (
	"context"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"

	v1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	scheme "github.com/liqotech/liqo/pkg/client/clientset/versioned/scheme"
)

// VkOptionsTemplatesGetter has a method to return a VkOptionsTemplateInterface.
// A group's client should implement this interface.
type VkOptionsTemplatesGetter interface {
	VkOptionsTemplates(namespace string) VkOptionsTemplateInterface
}

// VkOptionsTemplateInterface has methods to work with VkOptionsTemplate resources.
type VkOptionsTemplateInterface interface {
	Create(ctx context.Context, vkOptionsTemplate *v1beta1.VkOptionsTemplate, opts v1.CreateOptions) (*v1beta1.VkOptionsTemplate, error)
	Update(ctx context.Context, vkOptionsTemplate *v1beta1.VkOptionsTemplate, opts v1.UpdateOptions) (*v1beta1.VkOptionsTemplate, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1beta1.VkOptionsTemplate, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1beta1.VkOptionsTemplateList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1beta1.VkOptionsTemplate, err error)
	VkOptionsTemplateExpansion
}

// vkOptionsTemplates implements VkOptionsTemplateInterface
type vkOptionsTemplates struct {
	client rest.Interface
	ns     string
}

// newVkOptionsTemplates returns a VkOptionsTemplates
func newVkOptionsTemplates(c *OffloadingV1beta1Client, namespace string) *vkOptionsTemplates {
	return &vkOptionsTemplates{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the vkOptionsTemplate, and returns the corresponding vkOptionsTemplate object, and an error if there is any.
func (c *vkOptionsTemplates) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1beta1.VkOptionsTemplate, err error) {
	result = &v1beta1.VkOptionsTemplate{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("vkoptionstemplates").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of VkOptionsTemplates that match those selectors.
func (c *vkOptionsTemplates) List(ctx context.Context, opts v1.ListOptions) (result *v1beta1.VkOptionsTemplateList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1beta1.VkOptionsTemplateList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("vkoptionstemplates").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested vkOptionsTemplates.
func (c *vkOptionsTemplates) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("vkoptionstemplates").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a vkOptionsTemplate and creates it.  Returns the server's representation of the vkOptionsTemplate, and an error, if there is any.
func (c *vkOptionsTemplates) Create(ctx context.Context, vkOptionsTemplate *v1beta1.VkOptionsTemplate, opts v1.CreateOptions) (result *v1beta1.VkOptionsTemplate, err error) {
	result = &v1beta1.VkOptionsTemplate{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("vkoptionstemplates").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(vkOptionsTemplate).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a vkOptionsTemplate and updates it. Returns the server's representation of the vkOptionsTemplate, and an error, if there is any.
func (c *vkOptionsTemplates) Update(ctx context.Context, vkOptionsTemplate *v1beta1.VkOptionsTemplate, opts v1.UpdateOptions) (result *v1beta1.VkOptionsTemplate, err error) {
	result = &v1beta1.VkOptionsTemplate{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("vkoptionstemplates").
		Name(vkOptionsTemplate.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(vkOptionsTemplate).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the vkOptionsTemplate and deletes it. Returns an error if one occurs.
func (c *vkOptionsTemplates) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("vkoptionstemplates").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *vkOptionsTemplates) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("vkoptionstemplates").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched vkOptionsTemplate.
func (c *vkOptionsTemplates) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1beta1.VkOptionsTemplate, err error) {
	result = &v1beta1.VkOptionsTemplate{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("vkoptionstemplates").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
