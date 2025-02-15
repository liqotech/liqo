// Copyright 2019-2025 The Liqo Authors
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
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/listers"
	"k8s.io/client-go/tools/cache"

	v1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
)

// IPLister helps list IPs.
// All objects returned here must be treated as read-only.
type IPLister interface {
	// List lists all IPs in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.IP, err error)
	// IPs returns an object that can list and get IPs.
	IPs(namespace string) IPNamespaceLister
	IPListerExpansion
}

// iPLister implements the IPLister interface.
type iPLister struct {
	listers.ResourceIndexer[*v1alpha1.IP]
}

// NewIPLister returns a new IPLister.
func NewIPLister(indexer cache.Indexer) IPLister {
	return &iPLister{listers.New[*v1alpha1.IP](indexer, v1alpha1.Resource("ip"))}
}

// IPs returns an object that can list and get IPs.
func (s *iPLister) IPs(namespace string) IPNamespaceLister {
	return iPNamespaceLister{listers.NewNamespaced[*v1alpha1.IP](s.ResourceIndexer, namespace)}
}

// IPNamespaceLister helps list and get IPs.
// All objects returned here must be treated as read-only.
type IPNamespaceLister interface {
	// List lists all IPs in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.IP, err error)
	// Get retrieves the IP from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha1.IP, error)
	IPNamespaceListerExpansion
}

// iPNamespaceLister implements the IPNamespaceLister
// interface.
type iPNamespaceLister struct {
	listers.ResourceIndexer[*v1alpha1.IP]
}
