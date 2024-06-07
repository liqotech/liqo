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
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"

	v1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
)

// FakeVkOptionsTemplates implements VkOptionsTemplateInterface
type FakeVkOptionsTemplates struct {
	Fake *FakeVirtualkubeletV1alpha1
	ns   string
}

var vkoptionstemplatesResource = v1alpha1.SchemeGroupVersion.WithResource("vkoptionstemplates")

var vkoptionstemplatesKind = v1alpha1.SchemeGroupVersion.WithKind("VkOptionsTemplate")

// Get takes name of the vkOptionsTemplate, and returns the corresponding vkOptionsTemplate object, and an error if there is any.
func (c *FakeVkOptionsTemplates) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.VkOptionsTemplate, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(vkoptionstemplatesResource, c.ns, name), &v1alpha1.VkOptionsTemplate{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.VkOptionsTemplate), err
}

// List takes label and field selectors, and returns the list of VkOptionsTemplates that match those selectors.
func (c *FakeVkOptionsTemplates) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.VkOptionsTemplateList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(vkoptionstemplatesResource, vkoptionstemplatesKind, c.ns, opts), &v1alpha1.VkOptionsTemplateList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.VkOptionsTemplateList{ListMeta: obj.(*v1alpha1.VkOptionsTemplateList).ListMeta}
	for _, item := range obj.(*v1alpha1.VkOptionsTemplateList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested vkOptionsTemplates.
func (c *FakeVkOptionsTemplates) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(vkoptionstemplatesResource, c.ns, opts))

}

// Create takes the representation of a vkOptionsTemplate and creates it.  Returns the server's representation of the vkOptionsTemplate, and an error, if there is any.
func (c *FakeVkOptionsTemplates) Create(ctx context.Context, vkOptionsTemplate *v1alpha1.VkOptionsTemplate, opts v1.CreateOptions) (result *v1alpha1.VkOptionsTemplate, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(vkoptionstemplatesResource, c.ns, vkOptionsTemplate), &v1alpha1.VkOptionsTemplate{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.VkOptionsTemplate), err
}

// Update takes the representation of a vkOptionsTemplate and updates it. Returns the server's representation of the vkOptionsTemplate, and an error, if there is any.
func (c *FakeVkOptionsTemplates) Update(ctx context.Context, vkOptionsTemplate *v1alpha1.VkOptionsTemplate, opts v1.UpdateOptions) (result *v1alpha1.VkOptionsTemplate, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(vkoptionstemplatesResource, c.ns, vkOptionsTemplate), &v1alpha1.VkOptionsTemplate{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.VkOptionsTemplate), err
}

// Delete takes name of the vkOptionsTemplate and deletes it. Returns an error if one occurs.
func (c *FakeVkOptionsTemplates) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(vkoptionstemplatesResource, c.ns, name, opts), &v1alpha1.VkOptionsTemplate{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeVkOptionsTemplates) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(vkoptionstemplatesResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.VkOptionsTemplateList{})
	return err
}

// Patch applies the patch and returns the patched vkOptionsTemplate.
func (c *FakeVkOptionsTemplates) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.VkOptionsTemplate, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(vkoptionstemplatesResource, c.ns, name, pt, data, subresources...), &v1alpha1.VkOptionsTemplate{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.VkOptionsTemplate), err
}
