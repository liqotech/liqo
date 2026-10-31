// Copyright 2019-2026 The Liqo Authors
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

package gatewayapi

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/metadata/metadatainformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	gwutils "github.com/liqotech/liqo/pkg/utils/gatewayapi"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/generic"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

// unsupportedRouteReflector generates a warning event for each local route of a kind not reflected by Liqo (e.g., TCPRoutes),
// which would be otherwise silently ignored. Only the metadata of the routes are retrieved, as they are sufficient to
// generate the events, and they do not depend on the version of the Gateway API installed in the local cluster.
type unsupportedRouteReflector struct {
	manager.Reflector
	factory metadatainformer.SharedInformerFactory
}

// NewUnsupportedRouteReflectors returns the reflectors generating a warning event for each local route of the given kinds.
// Routes which the virtual kubelet is not allowed to list and watch are skipped, since the informers would never sync,
// blocking the start of the whole reflection.
func NewUnsupportedRouteReflectors(kubeClient kubernetes.Interface, metadataClient metadata.Interface,
	routes []gwutils.UnsupportedRoute, reflectorConfig *offloadingv1beta1.ReflectorConfig) []manager.Reflector {
	if reflectorConfig.NumWorkers == 0 {
		return nil
	}

	var reflectors []manager.Reflector
	factory := metadatainformer.NewSharedInformerFactory(metadataClient, 0)
	for _, route := range routes {
		if !canListAndWatch(kubeClient, route.GVR.GroupResource(), "") {
			klog.Warningf("Not allowed to list and watch %s in the local cluster: no events will be generated for the local objects, "+
				"which are not reflected to cluster %q", route.GVR.GroupResource(), forge.RemoteCluster)
			continue
		}
		reflectors = append(reflectors, NewUnsupportedRouteReflector(route, factory, reflectorConfig))
	}
	return reflectors
}

// NewUnsupportedRouteReflector returns a reflector generating a warning event for each local route of the given kind.
func NewUnsupportedRouteReflector(route gwutils.UnsupportedRoute, factory metadatainformer.SharedInformerFactory,
	reflectorConfig *offloadingv1beta1.ReflectorConfig) manager.Reflector {
	informer := factory.ForResource(route.GVR)
	return &unsupportedRouteReflector{
		Reflector: generic.NewReflector(route.Kind, newNamespacedUnsupportedRouteReflector(route, informer),
			newUnsupportedRouteFallback(informer), reflectorConfig.NumWorkers, reflectorConfig.Type, generic.ConcurrencyModeLeader),
		factory: factory,
	}
}

// Start starts the reflector, and the cluster-wide metadata informers, once the event handlers have been registered.
func (ur *unsupportedRouteReflector) Start(ctx context.Context, opts *options.ReflectorOpts) {
	ur.Reflector.Start(ctx, opts)
	ur.factory.Start(ctx.Done())
	ur.factory.WaitForCacheSync(ctx.Done())
}

// unsupportedRouteNamespacedReflector generates the warning events for the local routes of a given namespace.
type unsupportedRouteNamespacedReflector struct {
	generic.NamespacedReflector

	route gwutils.UnsupportedRoute
	local cache.GenericNamespaceLister
}

// NewNamespacedUnsupportedRouteReflector returns a function generating NamespacedReflector instances for the given kind of routes.
// It is exposed for testing purposes.
func NewNamespacedUnsupportedRouteReflector(route gwutils.UnsupportedRoute,
	factory metadatainformer.SharedInformerFactory) func(*options.NamespacedOpts) manager.NamespacedReflector {
	return newNamespacedUnsupportedRouteReflector(route, factory.ForResource(route.GVR))
}

func newNamespacedUnsupportedRouteReflector(route gwutils.UnsupportedRoute,
	informer informers.GenericInformer) func(*options.NamespacedOpts) manager.NamespacedReflector {
	return func(opts *options.NamespacedOpts) manager.NamespacedReflector {
		reflector := &unsupportedRouteNamespacedReflector{
			NamespacedReflector: generic.NewNamespacedReflector(opts, route.Kind),
			route:               route,
			local:               informer.Lister().ByNamespace(opts.LocalNamespace),
		}
		reflector.EventRecorder = opts.EventBroadcaster.NewRecorder(Scheme, corev1.EventSource{
			Component: "liqo-" + strings.ToLower(route.Kind) + "-reflection", Host: forge.LiqoNodeName})
		return reflector
	}
}

// Handle generates a warning event for the given local route, notifying that it is not reflected.
func (ur *unsupportedRouteNamespacedReflector) Handle(_ context.Context, name string) error {
	obj, err := ur.local.Get(name)
	if kerrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to retrieve local %s %q: %w", ur.route.Kind, ur.LocalRef(name), err)
	}

	accessor, err := meta.Accessor(obj)
	if err != nil {
		return fmt.Errorf("failed to access the metadata of local %s %q: %w", ur.route.Kind, ur.LocalRef(name), err)
	}

	// Routes excluded from the reflection are silently ignored, as they would not be reflected anyway.
	skipReflection, err := ur.ShouldSkipReflection(accessor)
	if err != nil {
		klog.Errorf("Failed to check whether local %s %q should be reflected: %v", ur.route.Kind, ur.LocalRef(name), err)
		return err
	}
	if skipReflection {
		return nil
	}

	reason := fmt.Sprintf("%s resources are not supported by the Liqo reflection", ur.route.Kind)
	klog.Warningf("Local %s %q cannot be reflected: %s", ur.route.Kind, ur.LocalRef(name), reason)

	// The event refers to the route through an explicit reference, since its type is not registered in the scheme.
	ur.Event(&corev1.ObjectReference{
		APIVersion: ur.route.GVR.GroupVersion().String(), Kind: ur.route.Kind,
		Namespace: accessor.GetNamespace(), Name: accessor.GetName(),
		UID: accessor.GetUID(), ResourceVersion: accessor.GetResourceVersion(),
	}, corev1.EventTypeWarning, forge.EventFailedReflection, forge.EventReflectionNotPossibleMsg(reason))
	return nil
}

// List returns the local routes of the namespace.
func (ur *unsupportedRouteNamespacedReflector) List() ([]interface{}, error) {
	objects, err := ur.local.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	keys := namespacedNames(objects)
	result := make([]interface{}, 0, len(keys))
	for _, key := range keys {
		result = append(result, key)
	}
	return result, nil
}

// Cleanup does nothing, as no resource is created.
func (ur *unsupportedRouteNamespacedReflector) Cleanup(_ context.Context, _, _ string) error {
	return nil
}

// unsupportedRouteFallback registers the event handlers on the cluster-wide metadata informer, enqueuing
// the routes in the namespaces currently reflected, while the ones in other namespaces are ignored.
type unsupportedRouteFallback struct {
	informer informers.GenericInformer
}

func newUnsupportedRouteFallback(informer informers.GenericInformer) generic.FallbackReflectorFactoryFunc {
	return func(opts *options.ReflectorOpts) manager.FallbackReflector {
		_, err := informer.Informer().AddEventHandler(opts.HandlerFactory(mappedKeyer(opts.NamespaceMapper)))
		utilruntime.Must(err)
		return &unsupportedRouteFallback{informer: informer}
	}
}

// Handle ignores the objects in the namespaces not reflected.
func (uf *unsupportedRouteFallback) Handle(_ context.Context, _ types.NamespacedName) error {
	return nil
}

// Keys returns the keys of the local routes in the given namespace, which are enqueued when the reflection starts.
func (uf *unsupportedRouteFallback) Keys(local, _ string) []types.NamespacedName {
	objects, err := uf.informer.Lister().ByNamespace(local).List(labels.Everything())
	utilruntime.Must(err)

	return namespacedNames(objects)
}

// Ready returns whether the fallback is ready, which is always true, as the informer is synced before starting.
func (uf *unsupportedRouteFallback) Ready() bool { return true }

// List returns no objects, as the ones in the namespaces reflected are listed by the namespaced reflectors.
func (uf *unsupportedRouteFallback) List() ([]interface{}, error) { return nil, nil }

// namespacedNames returns the keys of the given objects.
func namespacedNames(objects []runtime.Object) []types.NamespacedName {
	keys := make([]types.NamespacedName, 0, len(objects))
	for _, obj := range objects {
		if accessor, err := meta.Accessor(obj); err == nil {
			keys = append(keys, types.NamespacedName{Namespace: accessor.GetNamespace(), Name: accessor.GetName()})
		}
	}
	return keys
}
