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

// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	v1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
)

// VirtualNodeLister helps list VirtualNodes.
// All objects returned here must be treated as read-only.
type VirtualNodeLister interface {
	// List lists all VirtualNodes in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.VirtualNode, err error)
	// VirtualNodes returns an object that can list and get VirtualNodes.
	VirtualNodes(namespace string) VirtualNodeNamespaceLister
	VirtualNodeListerExpansion
}

// virtualNodeLister implements the VirtualNodeLister interface.
type virtualNodeLister struct {
	indexer cache.Indexer
}

// NewVirtualNodeLister returns a new VirtualNodeLister.
func NewVirtualNodeLister(indexer cache.Indexer) VirtualNodeLister {
	return &virtualNodeLister{indexer: indexer}
}

// List lists all VirtualNodes in the indexer.
func (s *virtualNodeLister) List(selector labels.Selector) (ret []*v1alpha1.VirtualNode, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.VirtualNode))
	})
	return ret, err
}

// VirtualNodes returns an object that can list and get VirtualNodes.
func (s *virtualNodeLister) VirtualNodes(namespace string) VirtualNodeNamespaceLister {
	return virtualNodeNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// VirtualNodeNamespaceLister helps list and get VirtualNodes.
// All objects returned here must be treated as read-only.
type VirtualNodeNamespaceLister interface {
	// List lists all VirtualNodes in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.VirtualNode, err error)
	// Get retrieves the VirtualNode from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha1.VirtualNode, error)
	VirtualNodeNamespaceListerExpansion
}

// virtualNodeNamespaceLister implements the VirtualNodeNamespaceLister
// interface.
type virtualNodeNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all VirtualNodes in the indexer for a given namespace.
func (s virtualNodeNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.VirtualNode, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.VirtualNode))
	})
	return ret, err
}

// Get retrieves the VirtualNode from the indexer for a given namespace and name.
func (s virtualNodeNamespaceLister) Get(name string) (*v1alpha1.VirtualNode, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("virtualnode"), name)
	}
	return obj.(*v1alpha1.VirtualNode), nil
}
