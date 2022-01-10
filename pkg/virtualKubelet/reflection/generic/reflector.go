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

package generic

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"k8s.io/utils/trace"

	traceutils "github.com/liqotech/liqo/pkg/utils/trace"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

var _ manager.Reflector = (*reflector)(nil)

// NamespacedReflectorFactoryFunc represents the function type to create a new NamespacedReflector.
type NamespacedReflectorFactoryFunc func(*options.NamespacedOpts) manager.NamespacedReflector

// FallbackReflectorFactoryFunc represents the function type to create a new FallbackReflector.
type FallbackReflectorFactoryFunc func(*options.ReflectorOpts) manager.FallbackReflector

// reflector implements the logic common to all reflectors.
type reflector struct {
	sync.RWMutex

	name    string
	workers uint

	workqueue workqueue.RateLimitingInterface

	reflectors map[string]manager.NamespacedReflector
	fallback   manager.FallbackReflector

	namespacedFactory NamespacedReflectorFactoryFunc
	fallbackFactory   FallbackReflectorFactoryFunc
}

// NewReflector returns a new reflector to implement the reflection towards a remote clusters.
func NewReflector(name string, namespaced NamespacedReflectorFactoryFunc, fallback FallbackReflectorFactoryFunc, workers uint) manager.Reflector {
	return &reflector{
		name:    name,
		workers: workers,

		workqueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), name),

		reflectors: make(map[string]manager.NamespacedReflector),

		namespacedFactory: namespaced,
		fallbackFactory:   fallback,
	}
}

// Start starts the reflector.
func (gr *reflector) Start(ctx context.Context, opts *options.ReflectorOpts) {
	klog.Infof("Starting the %v reflector with %v workers", gr.name, gr.workers)
	gr.fallback = gr.fallbackFactory(opts.WithHandlerFactory(gr.handlers))

	for i := uint(0); i < gr.workers; i++ {
		go wait.Until(gr.runWorker, time.Second, ctx.Done())
	}

	// Make sure the working queue is properly stopped when the context is closed.
	go func() {
		<-ctx.Done()
		gr.workqueue.ShutDown()
	}()
}

// StartNamespace starts the reflection for the given namespace.
func (gr *reflector) StartNamespace(opts *options.NamespacedOpts) {
	gr.Lock()
	defer gr.Unlock()

	klog.Infof("Starting %v reflection between local namespace %q and remote namespace %q",
		gr.name, opts.LocalNamespace, opts.RemoteNamespace)
	if _, found := gr.reflectors[opts.LocalNamespace]; found {
		klog.Warningf("%v reflection between local namespace %q and remote namespace %q already started",
			gr.name, opts.LocalNamespace, opts.RemoteNamespace)
		return
	}

	gr.reflectors[opts.LocalNamespace] = gr.namespacedFactory(opts.WithHandlerFactory(gr.handlers))

	// In case a fallback reflector exists, re-enqueue all the elements returned for the given namespace.
	if gr.fallback != nil {
		for _, key := range gr.fallback.Keys(opts.LocalNamespace, opts.RemoteNamespace) {
			gr.workqueue.Add(key)
		}
	}

	klog.Infof("%v reflection between local namespace %q and remote namespace %q correctly started",
		gr.name, opts.LocalNamespace, opts.RemoteNamespace)
}

// StopNamespace stops the reflection for a given namespace.
func (gr *reflector) StopNamespace(local, remote string) {
	gr.Lock()
	defer gr.Unlock()

	klog.Infof("Stopping %v reflection between local namespace %q and remote namespace %q", gr.name, local, remote)
	_, found := gr.reflectors[local]
	if !found {
		klog.Warningf("%v reflection between local namespace %q and remote namespace %q already stopped", gr.name, local, remote)
		return
	}

	delete(gr.reflectors, local)

	// In case a fallback reflector exists, re-enqueue all the elements returned for the given namespace.
	if gr.fallback != nil {
		for _, key := range gr.fallback.Keys(local, remote) {
			gr.workqueue.Add(key)
		}
	}

	klog.Infof("Reflection between local namespace %q and remote namespace %q correctly stopped", local, remote)
}

// namespace returns the service reflector associated with a given namespace (if any).
func (gr *reflector) namespace(namespace string) (manager.NamespacedReflector, bool) {
	gr.Lock()
	defer gr.Unlock()

	reflector, found := gr.reflectors[namespace]
	return reflector, found
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (gr *reflector) runWorker() {
	for gr.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the handler.
func (gr *reflector) processNextWorkItem() bool {
	// Get he element to be processed.
	key, shutdown := gr.workqueue.Get()

	if shutdown {
		return false
	}

	// We call Done here so the workqueue knows we have finished
	// processing this item. We also must remember to call Forget if we
	// do not want this work item being re-queued. For example, we do
	// not call Forget if a transient error occurs, instead the item is
	// put back on the workqueue and attempted again after a back-off
	// period.
	defer gr.workqueue.Done(key)

	// Run the handler, passing it the item to be processed as parameter.
	if err := gr.handle(context.Background(), key.(types.NamespacedName)); err != nil {
		// Put the item back on the workqueue to handle any transient errors.
		gr.workqueue.AddRateLimited(key)
		return true
	}

	// Finally, if no error occurs we Forget this item so it does not
	// get queued again until another change happens.
	gr.workqueue.Forget(key)
	return true
}

// handle dispatches the items to be reconciled based on the resource type and namespace.
func (gr *reflector) handle(ctx context.Context, key types.NamespacedName) error {
	tracer := trace.New("Handle", trace.Field{Key: "Reflector", Value: gr.name},
		trace.Field{Key: "Object", Value: key.Namespace}, trace.Field{Key: "Name", Value: key.Name})
	defer tracer.LogIfLong(traceutils.LongThreshold())

	// Retrieve the reflector associated with the given namespace.
	reflector, found := gr.namespace(key.Namespace)
	if !found {
		// In case none is found and no fallback is configured, just return.
		if gr.fallback == nil {
			klog.Warningf("Failed to retrieve %v reflection information for local namespace %q", gr.name, key.Namespace)
			return nil
		}

		// The fallback may not be completely initialized in case some namespace reflectors still have to be started.
		if !gr.fallback.Ready() {
			klog.Infof("%v fallback reflection not yet completely initialized (item: %q)", gr.name, klog.KRef(key.Namespace, key.Name))
			return fmt.Errorf("%v fallback reflection not yet completely initialized (item: %q)", gr.name, klog.KRef(key.Namespace, key.Name))
		}

		// Trigger the actual handle function.
		return gr.fallback.Handle(trace.ContextWithTrace(ctx, tracer), key)
	}

	// The reflector may not be completely initialized in case only one of the two informer factories has synced.
	if !reflector.Ready() {
		klog.Infof("%v reflection not yet completely initialized for local namespace %q (item: %q)", gr.name, key.Namespace, key.Name)
		return fmt.Errorf("%v reflection not yet completely initialized for local namespace %q (item: %q)", gr.name, key.Namespace, key.Name)
	}

	// Trigger the actual handle function.
	return reflector.Handle(trace.ContextWithTrace(ctx, tracer), key.Name)
}

// handle dispatches the items to be reconciled based on the resource type and namespace.
func (gr *reflector) handlers(keyer options.Keyer) cache.ResourceEventHandler {
	eh := func(obj interface{}) {
		metadata, err := meta.Accessor(obj)
		utilruntime.Must(err)
		klog.V(5).Infof("Enqueuing %v %q for reconciliation", gr.name, klog.KRef(metadata.GetNamespace(), metadata.GetName()))
		gr.workqueue.Add(keyer(metadata))
	}

	return cache.ResourceEventHandlerFuncs{
		AddFunc:    eh,
		UpdateFunc: func(_, obj interface{}) { eh(obj) },
		DeleteFunc: eh,
	}
}

// BasicKeyer returns a keyer retrieving the name and namespace from the object metadata.
func BasicKeyer() func(metadata metav1.Object) types.NamespacedName {
	return func(metadata metav1.Object) types.NamespacedName {
		return types.NamespacedName{Namespace: metadata.GetNamespace(), Name: metadata.GetName()}
	}
}

// NamespacedKeyer returns a keyer associated with the given namespace, retrieving the
// object name from its metadata.
func NamespacedKeyer(namespace string) func(metadata metav1.Object) types.NamespacedName {
	return func(metadata metav1.Object) types.NamespacedName {
		return types.NamespacedName{Namespace: namespace, Name: metadata.GetName()}
	}
}

// WithoutFallback returns a FallbackReflectorFactoryFunc which disables the fallback functionality.
func WithoutFallback() FallbackReflectorFactoryFunc {
	return func(ro *options.ReflectorOpts) manager.FallbackReflector { return nil }
}
