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

package fake

import (
	"context"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"

	v1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
)

// FakeShadowEndpointSlices implements ShadowEndpointSliceInterface
type FakeShadowEndpointSlices struct {
	Fake *FakeVirtualkubeletV1alpha1
	ns   string
}

var shadowendpointslicesResource = schema.GroupVersionResource{Group: "virtualkubelet.liqo.io", Version: "v1alpha1", Resource: "shadowendpointslices"}

var shadowendpointslicesKind = schema.GroupVersionKind{Group: "virtualkubelet.liqo.io", Version: "v1alpha1", Kind: "ShadowEndpointSlice"}

// Get takes name of the shadowEndpointSlice, and returns the corresponding shadowEndpointSlice object, and an error if there is any.
func (c *FakeShadowEndpointSlices) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.ShadowEndpointSlice, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(shadowendpointslicesResource, c.ns, name), &v1alpha1.ShadowEndpointSlice{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ShadowEndpointSlice), err
}

// List takes label and field selectors, and returns the list of ShadowEndpointSlices that match those selectors.
func (c *FakeShadowEndpointSlices) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.ShadowEndpointSliceList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(shadowendpointslicesResource, shadowendpointslicesKind, c.ns, opts), &v1alpha1.ShadowEndpointSliceList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.ShadowEndpointSliceList{ListMeta: obj.(*v1alpha1.ShadowEndpointSliceList).ListMeta}
	for _, item := range obj.(*v1alpha1.ShadowEndpointSliceList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested shadowEndpointSlices.
func (c *FakeShadowEndpointSlices) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(shadowendpointslicesResource, c.ns, opts))

}

// Create takes the representation of a shadowEndpointSlice and creates it.  Returns the server's representation of the shadowEndpointSlice, and an error, if there is any.
func (c *FakeShadowEndpointSlices) Create(ctx context.Context, shadowEndpointSlice *v1alpha1.ShadowEndpointSlice, opts v1.CreateOptions) (result *v1alpha1.ShadowEndpointSlice, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(shadowendpointslicesResource, c.ns, shadowEndpointSlice), &v1alpha1.ShadowEndpointSlice{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ShadowEndpointSlice), err
}

// Update takes the representation of a shadowEndpointSlice and updates it. Returns the server's representation of the shadowEndpointSlice, and an error, if there is any.
func (c *FakeShadowEndpointSlices) Update(ctx context.Context, shadowEndpointSlice *v1alpha1.ShadowEndpointSlice, opts v1.UpdateOptions) (result *v1alpha1.ShadowEndpointSlice, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(shadowendpointslicesResource, c.ns, shadowEndpointSlice), &v1alpha1.ShadowEndpointSlice{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ShadowEndpointSlice), err
}

// Delete takes name of the shadowEndpointSlice and deletes it. Returns an error if one occurs.
func (c *FakeShadowEndpointSlices) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(shadowendpointslicesResource, c.ns, name, opts), &v1alpha1.ShadowEndpointSlice{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeShadowEndpointSlices) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(shadowendpointslicesResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.ShadowEndpointSliceList{})
	return err
}

// Patch applies the patch and returns the patched shadowEndpointSlice.
func (c *FakeShadowEndpointSlices) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.ShadowEndpointSlice, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(shadowendpointslicesResource, c.ns, name, pt, data, subresources...), &v1alpha1.ShadowEndpointSlice{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ShadowEndpointSlice), err
}
