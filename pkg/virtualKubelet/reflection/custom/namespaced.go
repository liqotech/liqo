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

	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/generic"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

// NamespacedGVRReflector manages the reflection of a single namespaced GVR between paired namespaces.
type NamespacedGVRReflector struct {
	gvrReflector
}

// NewNamespacedGVRReflector returns a function generating NamespacedGVRReflector instances for the given GVR.
// statusSyncEnabled indicates whether the remote→local status sync is active (opt-in
// configuration and status subresource availability verified at provider startup):
// when false, the status sync is skipped.
func NewNamespacedGVRReflector(gvr schema.GroupVersionResource, statusSyncEnabled bool) func(*options.NamespacedOpts) manager.NamespacedReflector {
	return func(opts *options.NamespacedOpts) manager.NamespacedReflector {
		localInformer := opts.LocalDynamicFactory.ForResource(gvr)
		remoteInformer := opts.RemoteDynamicFactory.ForResource(gvr)

		_, err := localInformer.Informer().AddEventHandler(opts.HandlerFactory(generic.NamespacedKeyer(opts.LocalNamespace)))
		utilruntime.Must(err)
		_, err = remoteInformer.Informer().AddEventHandler(opts.HandlerFactory(generic.NamespacedKeyer(opts.LocalNamespace)))
		utilruntime.Must(err)

		return &NamespacedGVRReflector{
			gvrReflector: gvrReflector{
				NamespacedReflector: generic.NewNamespacedReflector(opts, "CustomResource["+gvr.String()+"]"),
				gvr:                 gvr,
				localLister:         localInformer.Lister().ByNamespace(opts.LocalNamespace),
				remoteLister:        remoteInformer.Lister().ByNamespace(opts.RemoteNamespace),
				localClient:         opts.LocalDynamicClient.Resource(gvr).Namespace(opts.LocalNamespace),
				remoteClient:        opts.RemoteDynamicClient.Resource(gvr).Namespace(opts.RemoteNamespace),
				statusSyncEnabled:   statusSyncEnabled,
			},
		}
	}
}

// List returns the list of objects to be reflected.
func (ncr *NamespacedGVRReflector) List() ([]any, error) {
	return ncr.list(ncr.LocalNamespace())
}
