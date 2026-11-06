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

package custom

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"

	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/generic"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

// ClusterScopedGVRReflector manages the reflection of a single cluster-scoped GVR,
// which is not associated with any namespace pair.
type ClusterScopedGVRReflector struct {
	gvrReflector

	localInformer  informers.GenericInformer
	remoteInformer informers.GenericInformer
}

// NewClusterGVRReflector returns a function generating ClusterScopedGVRReflector instances for
// the given GVR. statusSyncEnabled indicates whether the remote→local status sync is active
// (opt-in configuration and status subresource availability verified at provider startup):
// when false, the status sync is skipped.
func NewClusterGVRReflector(gvr schema.GroupVersionResource, statusSyncEnabled bool) func(*options.ReflectorOpts) manager.NamespacedReflector {
	return func(opts *options.ReflectorOpts) manager.NamespacedReflector {
		localInformer := opts.LocalDynamicFactory.ForResource(gvr)
		remoteInformer := opts.RemoteDynamicFactory.ForResource(gvr)

		// Cluster-scoped resources are keyed by the empty namespace, which is
		// guaranteed not to collide with any real namespace.
		_, err := localInformer.Informer().AddEventHandler(opts.HandlerFactory(generic.NamespacedKeyer("")))
		utilruntime.Must(err)
		_, err = remoteInformer.Informer().AddEventHandler(opts.HandlerFactory(generic.NamespacedKeyer("")))
		utilruntime.Must(err)

		// The shared base reflector is forged with empty namespaces, so that the object
		// refs correctly identify cluster-scoped resources. Its readiness is overridden
		// by ClusterScopedGVRReflector.Ready, as it depends on the synchronization of the informer caches.
		return &ClusterScopedGVRReflector{
			gvrReflector: gvrReflector{
				NamespacedReflector: generic.NewReflectorBase(opts, "CustomResource["+gvr.String()+"]"),
				gvr:                 gvr,
				localLister:         localInformer.Lister(),
				remoteLister:        remoteInformer.Lister(),
				localClient:         opts.LocalDynamicClient.Resource(gvr),
				remoteClient:        opts.RemoteDynamicClient.Resource(gvr),
				clusterScoped:       true,
				statusSyncEnabled:   statusSyncEnabled,
			},
			localInformer:  localInformer,
			remoteInformer: remoteInformer,
		}
	}
}

// Ready returns whether the ClusterScopedGVRReflector is completely initialized,
// i.e. both the local and remote informer caches have synchronized.
func (cr *ClusterScopedGVRReflector) Ready() bool {
	return cr.localInformer.Informer().HasSynced() && cr.remoteInformer.Informer().HasSynced()
}

// List returns the list of objects to be reflected.
func (cr *ClusterScopedGVRReflector) List() ([]any, error) {
	return cr.list("")
}
