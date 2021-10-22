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

package options

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// Keyer retrieves a NamespacedName referring to the reconciliation target from the object metadata.
type Keyer func(metadata metav1.Object) types.NamespacedName

// ReflectorOpts is a structure grouping the parameters to start a Reflector.
type ReflectorOpts struct {
	LocalClient      kubernetes.Interface
	LocalPodInformer corev1informers.PodInformer

	HandlerFactory func(Keyer) cache.ResourceEventHandler
}

// New returns a new ReflectorOpts object.
func New(client kubernetes.Interface, podInformer corev1informers.PodInformer) *ReflectorOpts {
	return &ReflectorOpts{LocalClient: client, LocalPodInformer: podInformer}
}

// WithHandlerFactory configures the handler factory of the ReflectorOpts.
func (ro *ReflectorOpts) WithHandlerFactory(handler func(Keyer) cache.ResourceEventHandler) *ReflectorOpts {
	ro.HandlerFactory = handler
	return ro
}

// NamespacedOpts is a structure grouping the parameters to start a NamespacedReflector.
type NamespacedOpts struct {
	LocalNamespace  string
	RemoteNamespace string

	LocalClient  kubernetes.Interface
	RemoteClient kubernetes.Interface

	LocalFactory  informers.SharedInformerFactory
	RemoteFactory informers.SharedInformerFactory

	Ready          func() bool
	HandlerFactory func(Keyer) cache.ResourceEventHandler
}

// NewNamespaced returns a new NamespacedOpts object.
func NewNamespaced() *NamespacedOpts {
	return &NamespacedOpts{}
}

// WithLocal configures the local parameters of the NamespacedOpts.
func (ro *NamespacedOpts) WithLocal(namespace string, client kubernetes.Interface, factory informers.SharedInformerFactory) *NamespacedOpts {
	ro.LocalNamespace = namespace
	ro.LocalClient = client
	ro.LocalFactory = factory
	return ro
}

// WithRemote configures the remote parameters of the NamespacedOpts.
func (ro *NamespacedOpts) WithRemote(namespace string, client kubernetes.Interface, factory informers.SharedInformerFactory) *NamespacedOpts {
	ro.RemoteNamespace = namespace
	ro.RemoteClient = client
	ro.RemoteFactory = factory
	return ro
}

// WithHandlerFactory configures the handler factory of the NamespacedOpts.
func (ro *NamespacedOpts) WithHandlerFactory(handler func(Keyer) cache.ResourceEventHandler) *NamespacedOpts {
	ro.HandlerFactory = handler
	return ro
}

// WithReadinessFunc configures the readiness function of the NamespacedOpts.
func (ro *NamespacedOpts) WithReadinessFunc(ready func() bool) *NamespacedOpts {
	ro.Ready = ready
	return ro
}
