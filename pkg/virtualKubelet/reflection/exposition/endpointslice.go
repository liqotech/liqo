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

package exposition

import (
	"context"
	"fmt"
	"sync"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	discoveryv1beta1clients "k8s.io/client-go/kubernetes/typed/discovery/v1beta1"
	discoveryv1beta1listers "k8s.io/client-go/listers/discovery/v1beta1"
	"k8s.io/klog/v2"
	"k8s.io/utils/trace"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/liqonet/ipam"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/generic"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

var _ manager.NamespacedReflector = (*NamespacedEndpointSliceReflector)(nil)

const (
	// EndpointSliceReflectorName -> The name associated with the EndpointSlice reflector.
	EndpointSliceReflectorName = "EndpointSlice"
)

// NamespacedEndpointSliceReflector manages the EndpointSlice reflection for a given pair of local and remote namespaces.
type NamespacedEndpointSliceReflector struct {
	generic.NamespacedReflector

	localEndpointSlices        discoveryv1beta1listers.EndpointSliceNamespaceLister
	remoteEndpointSlices       discoveryv1beta1listers.EndpointSliceNamespaceLister
	remoteEndpointSlicesClient discoveryv1beta1clients.EndpointSliceInterface

	ipamclient   ipam.IpamClient
	translations sync.Map
}

// NewEndpointSliceReflector returns a new EndpointSliceReflector instance.
func NewEndpointSliceReflector(ipamclient ipam.IpamClient, workers uint) manager.Reflector {
	return generic.NewReflector(EndpointSliceReflectorName, NewNamespacedEndpointSliceReflector(ipamclient), generic.WithoutFallback(), workers)
}

// NewNamespacedEndpointSliceReflector returns a function generating NamespacedEndpointSliceReflector instances.
func NewNamespacedEndpointSliceReflector(ipamclient ipam.IpamClient) func(*options.NamespacedOpts) manager.NamespacedReflector {
	return func(opts *options.NamespacedOpts) manager.NamespacedReflector {
		local := opts.LocalFactory.Discovery().V1beta1().EndpointSlices()
		remote := opts.RemoteFactory.Discovery().V1beta1().EndpointSlices()

		local.Informer().AddEventHandler(opts.HandlerFactory(generic.NamespacedKeyer(opts.LocalNamespace)))
		remote.Informer().AddEventHandler(opts.HandlerFactory(generic.NamespacedKeyer(opts.LocalNamespace)))

		return &NamespacedEndpointSliceReflector{
			NamespacedReflector:        generic.NewNamespacedReflector(opts),
			localEndpointSlices:        local.Lister().EndpointSlices(opts.LocalNamespace),
			remoteEndpointSlices:       remote.Lister().EndpointSlices(opts.RemoteNamespace),
			remoteEndpointSlicesClient: opts.RemoteClient.DiscoveryV1beta1().EndpointSlices(opts.RemoteNamespace),
			ipamclient:                 ipamclient,
		}
	}
}

// Handle reconciles endpointslice objects.
func (ner *NamespacedEndpointSliceReflector) Handle(ctx context.Context, name string) error {
	tracer := trace.FromContext(ctx)

	// Skip the kubernetes service, as meaningless to reflect.
	if ner.LocalNamespace() == corev1.NamespaceDefault && name == kubernetesServiceName {
		klog.V(4).Infof("Skipping reflection of local EndpointSlice %q as blacklisted", ner.LocalRef(name))
		return nil
	}

	// Retrieve the local and remote objects (only not found errors can occur).
	klog.V(4).Infof("Handling reflection of local EndpointSlice %q (remote: %q)", ner.LocalRef(name), ner.RemoteRef(name))
	local, lerr := ner.localEndpointSlices.Get(name)
	utilruntime.Must(client.IgnoreNotFound(lerr))
	remote, rerr := ner.remoteEndpointSlices.Get(name)
	utilruntime.Must(client.IgnoreNotFound(rerr))
	tracer.Step("Retrieved the local and remote objects")

	// Abort the reflection if the remote object is not managed by us, as we do not want to mutate others' objects.
	if rerr == nil && (!forge.IsReflected(remote) || !forge.IsEndpointSliceManagedByReflection(remote)) {
		// Prevent misleading warnings triggered by remote non-reflected endpointslices, since they inherit
		// the labels from the service. Hence, vanilla remote endpointslices do also trigger the Handle function.
		if lerr == nil {
			klog.Infof("Skipping reflection of local EndpointSlice %q as remote already exists and is not managed by us", ner.LocalRef(name))
		}
		return nil
	}
	tracer.Step("Performed the sanity checks")

	// The local endpointslice does no longer exist. Ensure it is also absent from the remote cluster.
	if kerrors.IsNotFound(lerr) {
		// Release the address translations
		if err := ner.UnmapEndpointIPs(ctx, name); err != nil {
			return err
		}

		defer tracer.Step("Ensured the absence of the remote object")
		if !kerrors.IsNotFound(rerr) {
			klog.V(4).Infof("Deleting remote EndpointSlice %q, since local %q does no longer exist", ner.RemoteRef(name), ner.LocalRef(name))
			return ner.DeleteRemote(ctx, ner.remoteEndpointSlicesClient, EndpointSliceReflectorName, name, remote.GetUID())
		}

		klog.V(4).Infof("Local EndpointSlice %q and remote EndpointSlice %q both vanished", ner.LocalRef(name), ner.RemoteRef(name))
		return nil
	}

	// Wrap the address translation logic, so that we do not have to handle errors in the forge logic.
	var terr error
	translator := func(originals []string) []string {
		// Avoid processing further addresses if one already failed.
		if terr != nil {
			return nil
		}

		var translations []string
		translations, terr = ner.MapEndpointIPs(ctx, name, originals)
		return translations
	}

	// Forge the mutation to be applied to the remote cluster.
	mutation := forge.RemoteEndpointSlice(local, ner.RemoteNamespace(), translator)
	if terr != nil {
		klog.Errorf("Reflection of local EndpointSlice %q to %q failed: %v", ner.LocalRef(name), ner.RemoteRef(name), terr)
		return terr
	}
	tracer.Step("Remote mutation created")

	// Apply the mutation.
	defer tracer.Step("Enforced the correctness of the remote object")
	if _, err := ner.remoteEndpointSlicesClient.Apply(ctx, mutation, forge.ApplyOptions()); err != nil {
		klog.Errorf("Failed to enforce remote EndpointSlice %q (local: %q): %v", ner.RemoteRef(name), ner.LocalRef(name), err)
		return err
	}

	klog.Infof("Remote EndpointSlice %q successfully enforced (local: %q)", ner.RemoteRef(name), ner.LocalRef(name))
	return nil
}

// MapEndpointIPs maps the local set of addresses to the corresponding remote ones.
func (ner *NamespacedEndpointSliceReflector) MapEndpointIPs(ctx context.Context, endpointslice string, originals []string) ([]string, error) {
	var translations []string

	// Retrieve the cache for the given endpointslice. The cache is not synchronized,
	// since we are guaranteed to be the only ones operating on this object.
	ucache, _ := ner.translations.LoadOrStore(endpointslice, map[string]string{})
	cache := ucache.(map[string]string)

	for _, original := range originals {
		// Check if we already know the translation.
		translation, found := cache[original]

		if !found {
			// Cache miss -> we need to interact with the IPAM to request the translation.
			response, err := ner.ipamclient.MapEndpointIP(ctx, &ipam.MapRequest{ClusterID: forge.RemoteClusterID, Ip: original})
			if err != nil {
				return nil, fmt.Errorf("failed to translate endpoint IP %v: %w", original, err)
			}
			translation = response.GetIp()
			cache[original] = translation
		}

		translations = append(translations, translation)
		klog.V(6).Infof("Translated local endpoint IP %v to remote %v", original, translation)
	}

	return translations, nil
}

// UnmapEndpointIPs unmaps the local set of addresses for the given endpointslice and releases the corresponding remote ones.
func (ner *NamespacedEndpointSliceReflector) UnmapEndpointIPs(ctx context.Context, endpointslice string) error {
	// Retrieve the cache for the given endpointslice. The cache is not synchronized,
	// since we are guaranteed to be the only ones operating on this object.
	ucache, found := ner.translations.Load(endpointslice)
	if !found {
		klog.V(4).Infof("Mappings from local EndpointSlice %q to remote %q already released",
			ner.LocalRef(endpointslice), ner.RemoteRef(endpointslice))
		return nil
	}

	cache := ucache.(map[string]string)
	for original, translation := range cache {
		// Interact with the IPAM to release the translation.
		_, err := ner.ipamclient.UnmapEndpointIP(ctx, &ipam.UnmapRequest{ClusterID: forge.RemoteClusterID, Ip: original})
		if err != nil {
			klog.Errorf("Failed to release endpoint IP %v of EndpointSlice %q: %w", original, ner.LocalRef(endpointslice), err)
			return fmt.Errorf("failed to release endpoint IP %v of EndpointSlice %q: %w", original, ner.LocalRef(endpointslice), err)
		}

		// Remove the object from our local cache, to avoid retrying to free it again if an error occurs with the subsequent entries.
		klog.V(6).Infof("Released mapping from local endpoint IP %v to remote %v of EndpointSlice %q", original, translation, ner.LocalRef(endpointslice))
		delete(cache, original)
	}

	// Remove the cache for this endpointslice.
	ner.translations.Delete(endpointslice)
	klog.V(4).Infof("Released mappings from local EndpointSlice %q to remote %q", ner.LocalRef(endpointslice), ner.RemoteRef(endpointslice))
	return nil
}
