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

package options

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	liqoclient "github.com/liqotech/liqo/pkg/client/clientset/versioned"
	liqoinformers "github.com/liqotech/liqo/pkg/client/informers/externalversions"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

// Keyer retrieves a set of NamespacedNames referring to the reconciliation targets from the object metadata.
type Keyer func(metadata metav1.Object) []types.NamespacedName

// EventFilter filters out the events matching a given type.
type EventFilter func(watch.EventType) bool

// ReflectorOpts is a structure grouping the parameters to start a Reflector.
type ReflectorOpts struct {
	LocalClient      kubernetes.Interface
	LocalPodInformer corev1informers.PodInformer
	EventBroadcaster record.EventBroadcaster

	HandlerFactory func(Keyer, ...EventFilter) cache.ResourceEventHandler

	Ready func() bool
}

// New returns a new ReflectorOpts object.
func New(client kubernetes.Interface, podInformer corev1informers.PodInformer) *ReflectorOpts {
	return &ReflectorOpts{LocalClient: client, LocalPodInformer: podInformer}
}

// WithHandlerFactory configures the handler factory of the ReflectorOpts.
func (ro *ReflectorOpts) WithHandlerFactory(handler func(Keyer, ...EventFilter) cache.ResourceEventHandler) *ReflectorOpts {
	ro.HandlerFactory = handler
	return ro
}

// WithReadinessFunc configures the readiness function of the ReflectorOpts.
func (ro *ReflectorOpts) WithReadinessFunc(ready func() bool) *ReflectorOpts {
	ro.Ready = ready
	return ro
}

// WithEventBroadcaster configures the event broadcaster of the NamespacedOpts.
func (ro *ReflectorOpts) WithEventBroadcaster(broadcaster record.EventBroadcaster) *ReflectorOpts {
	ro.EventBroadcaster = broadcaster
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
	HandlerFactory func(Keyer, ...EventFilter) cache.ResourceEventHandler

	ForgingOpts    *forge.ForgingOpts
	ReflectionType offloadingv1beta1.ReflectionType
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
func (ro *NamespacedOpts) WithHandlerFactory(handler func(Keyer, ...EventFilter) cache.ResourceEventHandler) *NamespacedOpts {
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

// WithForgingOpts configures the reflection options of the NamespacedOpts.
func (ro *NamespacedOpts) WithForgingOpts(opts *forge.ForgingOpts) *NamespacedOpts {
	ro.ForgingOpts = opts
	return ro
}

// WithReflectionType configures the reflection type of the NamespacedOpts.
func (ro *NamespacedOpts) WithReflectionType(reflectionType offloadingv1beta1.ReflectionType) *NamespacedOpts {
	ro.ReflectionType = reflectionType
	return ro
}

// EventFilterCreate ignores events of type create.
func EventFilterCreate(et watch.EventType) bool { return et == watch.Added }

// EventFilterUpdate ignores events of type update.
func EventFilterUpdate(et watch.EventType) bool { return et == watch.Modified }

// EventFilterDelete ignores events of type delete.
func EventFilterDelete(et watch.EventType) bool { return et == watch.Deleted }
