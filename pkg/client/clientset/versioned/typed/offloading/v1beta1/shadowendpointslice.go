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

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	gentype "k8s.io/client-go/gentype"

	v1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	scheme "github.com/liqotech/liqo/pkg/client/clientset/versioned/scheme"
)

// ShadowEndpointSlicesGetter has a method to return a ShadowEndpointSliceInterface.
// A group's client should implement this interface.
type ShadowEndpointSlicesGetter interface {
	ShadowEndpointSlices(namespace string) ShadowEndpointSliceInterface
}

// ShadowEndpointSliceInterface has methods to work with ShadowEndpointSlice resources.
type ShadowEndpointSliceInterface interface {
	Create(ctx context.Context, shadowEndpointSlice *v1beta1.ShadowEndpointSlice, opts v1.CreateOptions) (*v1beta1.ShadowEndpointSlice, error)
	Update(ctx context.Context, shadowEndpointSlice *v1beta1.ShadowEndpointSlice, opts v1.UpdateOptions) (*v1beta1.ShadowEndpointSlice, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1beta1.ShadowEndpointSlice, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1beta1.ShadowEndpointSliceList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1beta1.ShadowEndpointSlice, err error)
	ShadowEndpointSliceExpansion
}

// shadowEndpointSlices implements ShadowEndpointSliceInterface
type shadowEndpointSlices struct {
	*gentype.ClientWithList[*v1beta1.ShadowEndpointSlice, *v1beta1.ShadowEndpointSliceList]
}

// newShadowEndpointSlices returns a ShadowEndpointSlices
func newShadowEndpointSlices(c *OffloadingV1beta1Client, namespace string) *shadowEndpointSlices {
	return &shadowEndpointSlices{
		gentype.NewClientWithList[*v1beta1.ShadowEndpointSlice, *v1beta1.ShadowEndpointSliceList](
			"shadowendpointslices",
			c.RESTClient(),
			scheme.ParameterCodec,
			namespace,
			func() *v1beta1.ShadowEndpointSlice { return &v1beta1.ShadowEndpointSlice{} },
			func() *v1beta1.ShadowEndpointSliceList { return &v1beta1.ShadowEndpointSliceList{} }),
	}
}
