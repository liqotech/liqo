// Copyright 2019-2022 The Liqo Authors
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
	"k8s.io/client-go/tools/record"

	liqoclient "github.com/liqotech/liqo/pkg/client/clientset/versioned"
	liqoinformers "github.com/liqotech/liqo/pkg/client/informers/externalversions"
)

// Keyer retrieves a NamespacedName referring to the reconciliation target from the object metadata.
type Keyer func(metadata metav1.Object) types.NamespacedName

// ReflectorOpts is a structure grouping the parameters to start a Reflector.
type ReflectorOpts struct {
	LocalClient      kubernetes.Interface
	LocalPodInformer corev1informers.PodInformer

	HandlerFactory func(Keyer) cache.ResourceEventHandler

	Ready func() bool
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

// WithReadinessFunc configures the readiness function of the ReflectorOpts.
func (ro *ReflectorOpts) WithReadinessFunc(ready func() bool) *ReflectorOpts {
	ro.Ready = ready
	return ro
}

// NamespacedOpts is a structure grouping the parameters to start a NamespacedReflector.
type NamespacedOpts struct {
	LocalNamespace  string
	RemoteNamespace string

	LocalClient      kubernetes.Interface
	RemoteClient     kubernetes.Interface
	LocalLiqoClient  liqoclient.Interface
	RemoteLiqoClient liqoclient.Interface

	LocalFactory      informers.SharedInformerFactory
	RemoteFactory     informers.SharedInformerFactory
	LocalLiqoFactory  liqoinformers.SharedInformerFactory
	RemoteLiqoFactory liqoinformers.SharedInformerFactory

	EventBroadcaster record.EventBroadcaster

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

// WithLiqoLocal configures the local liqo client and informer factory parameters of the NamespacedOpts.
func (ro *NamespacedOpts) WithLiqoLocal(client liqoclient.Interface, factory liqoinformers.SharedInformerFactory) *NamespacedOpts {
	ro.LocalLiqoClient = client
	ro.LocalLiqoFactory = factory
	return ro
}

// WithRemote configures the remote parameters of the NamespacedOpts.
func (ro *NamespacedOpts) WithRemote(namespace string, client kubernetes.Interface, factory informers.SharedInformerFactory) *NamespacedOpts {
	ro.RemoteNamespace = namespace
	ro.RemoteClient = client
	ro.RemoteFactory = factory
	return ro
}

// WithLiqoRemote configures the remote liqo client and informer factory parameters of the NamespacedOpts.
func (ro *NamespacedOpts) WithLiqoRemote(client liqoclient.Interface, factory liqoinformers.SharedInformerFactory) *NamespacedOpts {
	ro.RemoteLiqoClient = client
	ro.RemoteLiqoFactory = factory
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

// WithEventBroadcaster configures the event broadcaster of the NamespacedOpts.
func (ro *NamespacedOpts) WithEventBroadcaster(broadcaster record.EventBroadcaster) *NamespacedOpts {
	ro.EventBroadcaster = broadcaster
	return ro
}
