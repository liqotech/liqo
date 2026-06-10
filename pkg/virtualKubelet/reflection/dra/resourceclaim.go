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

package dra

import (
	"context"
	"fmt"

	resourcev1 "k8s.io/api/resource/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	resourcev1clients "k8s.io/client-go/kubernetes/typed/resource/v1"
	resourcev1listers "k8s.io/client-go/listers/resource/v1"
	"k8s.io/klog/v2"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/utils/virtualkubelet"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/generic"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

const (
	// ResourceClaimReflectorName is the name associated with the ResourceClaim reflector.
	ResourceClaimReflectorName = "ResourceClaim"
)

// ResourceClaimReflector mirrors ResourceClaim objects from the local cluster to the
// remote cluster. The lifecycle of a remote claim is anchored to the local claim: a
// remote claim is deleted only when its local counterpart is deleted.
type ResourceClaimReflector struct {
	manager.Reflector
}

// NamespacedResourceClaimReflector handles ResourceClaim reflection for a single
// (local, remote) namespace pair.
type NamespacedResourceClaimReflector struct {
	generic.NamespacedReflector

	localClaims  resourcev1listers.ResourceClaimNamespaceLister
	remoteClaims resourcev1listers.ResourceClaimNamespaceLister

	remoteClaimsClient        resourcev1clients.ResourceClaimInterface
	localDeviceClassesClient  resourcev1clients.DeviceClassInterface
	remoteDeviceClassesClient resourcev1clients.DeviceClassInterface
}

var _ manager.NamespacedReflector = (*NamespacedResourceClaimReflector)(nil)

// NewResourceClaimReflector returns a new ResourceClaimReflector.
func NewResourceClaimReflector(reflectorConfig *offloadingv1beta1.ReflectorConfig) manager.Reflector {
	r := &ResourceClaimReflector{}
	r.Reflector = generic.NewReflector(ResourceClaimReflectorName,
		r.NewNamespaced, generic.WithoutFallback(),
		reflectorConfig.NumWorkers, offloadingv1beta1.CustomLiqo, generic.ConcurrencyModeAll)
	return r
}

// NewNamespaced returns a new NamespacedResourceClaimReflector instance.
func (rcr *ResourceClaimReflector) NewNamespaced(opts *options.NamespacedOpts) manager.NamespacedReflector {
	localClaims := opts.LocalFactory.Resource().V1().ResourceClaims()
	remoteClaims := opts.RemoteFactory.Resource().V1().ResourceClaims()

	// Local claim events enqueue the claim name directly.
	_, err := localClaims.Informer().AddEventHandler(opts.HandlerFactory(generic.NamespacedKeyer(opts.LocalNamespace)))
	utilruntime.Must(err)
	// Remote claim events enqueue by name (preserved during reflection).
	_, err = remoteClaims.Informer().AddEventHandler(opts.HandlerFactory(
		func(obj metav1.Object) []types.NamespacedName {
			if !forge.IsReflected(obj) {
				return nil
			}
			return []types.NamespacedName{{Namespace: opts.LocalNamespace, Name: obj.GetName()}}
		},
	))
	utilruntime.Must(err)

	return &NamespacedResourceClaimReflector{
		NamespacedReflector: generic.NewNamespacedReflector(opts, ResourceClaimReflectorName),

		localClaims:  localClaims.Lister().ResourceClaims(opts.LocalNamespace),
		remoteClaims: remoteClaims.Lister().ResourceClaims(opts.RemoteNamespace),

		remoteClaimsClient:        opts.RemoteClient.ResourceV1().ResourceClaims(opts.RemoteNamespace),
		localDeviceClassesClient:  opts.LocalClient.ResourceV1().DeviceClasses(),
		remoteDeviceClassesClient: opts.RemoteClient.ResourceV1().DeviceClasses(),
	}
}

// Handle reconciles a single ResourceClaim identified by claimName.
// The remote claim lifecycle is anchored to the local claim: the remote is
// deleted only when the local claim no longer exists.
func (n *NamespacedResourceClaimReflector) Handle(ctx context.Context, claimName string) error {
	klog.V(4).Infof("Handling ResourceClaim reflection for local claim %q", n.LocalRef(claimName))

	local, lerr := n.localClaims.Get(claimName)
	if lerr != nil && !kerrors.IsNotFound(lerr) {
		return lerr
	}

	remote, rerr := n.remoteClaims.Get(claimName)
	if rerr != nil && !kerrors.IsNotFound(rerr) {
		return rerr
	}

	if remote == nil && local == nil {
		// No local and no remote: nothing to do.
		return nil
	}

	// Refuse to mutate a remote claim we don't own.
	if rerr == nil && !forge.IsReflected(remote) {
		klog.Infof("Skipping reflection of ResourceClaim %q: remote exists and is not managed by Liqo", n.RemoteRef(claimName))
		return nil
	}

	// Local claim gone: clean up the remote if we own it.
	if kerrors.IsNotFound(lerr) && remote != nil {
		klog.Infof("Deleting remote ResourceClaim %q: local no longer exists", n.RemoteRef(claimName))
		return n.DeleteRemote(ctx, n.remoteClaimsClient, ResourceClaimReflectorName, claimName, remote.GetUID())
	}

	// Ensure every DeviceClass referenced by the claim is present on the remote.
	for _, dc := range forge.ReferencedDeviceClasses(local) {
		err := ensureRemoteDeviceClass(ctx, dc,
			n.localDeviceClassesClient, n.remoteDeviceClassesClient,
			n.ForgingOpts.LabelsNotReflected, n.ForgingOpts.AnnotationsNotReflected)
		switch {
		case kerrors.IsNotFound(err):
			// Do not reflect the claim if any referenced class is missing locally.
			klog.Warningf("Skipping reflection of ResourceClaim %q as it references missing local DeviceClass %q", n.LocalRef(claimName), dc)
			return nil
		case err != nil:
			return fmt.Errorf("ensuring ResourceClaim %q DeviceClass: %w", claimName, err)
		}
	}

	desired := forge.RemoteResourceClaim(local, n.RemoteNamespace(),
		n.ForgingOpts.LabelsNotReflected, n.ForgingOpts.AnnotationsNotReflected)

	if kerrors.IsNotFound(rerr) {
		if _, err := n.remoteClaimsClient.Create(ctx, desired, metav1.CreateOptions{}); err != nil && !kerrors.IsAlreadyExists(err) {
			klog.Errorf("Failed to create remote ResourceClaim %q: %v", n.RemoteRef(claimName), err)
			return fmt.Errorf("creating ResourceClaim %q: %w", n.RemoteRef(claimName), err)
		}
		klog.Infof("Remote ResourceClaim %q successfully created", n.RemoteRef(claimName))
		return nil
	}

	// ResourceClaim.Spec is immutable; only labels/annotations can be updated.
	if apiequality.Semantic.DeepEqual(remote.Labels, desired.Labels) &&
		apiequality.Semantic.DeepEqual(remote.Annotations, desired.Annotations) {
		klog.V(4).Infof("Remote ResourceClaim %q is already up-to-date", n.RemoteRef(claimName))
		return nil
	}
	updated := remote.DeepCopy()
	updated.Labels = desired.Labels
	updated.Annotations = desired.Annotations
	if _, err := n.remoteClaimsClient.Update(ctx, updated, metav1.UpdateOptions{}); err != nil {
		klog.Errorf("Failed to update remote ResourceClaim %q metadata: %v", n.RemoteRef(claimName), err)
		return fmt.Errorf("updating ResourceClaim %q: %w", n.RemoteRef(claimName), err)
	}
	return nil
}

// List returns the union of local and remote reflected claim names so that resync
// catches orphaned remote claims whose local was deleted while the VK was down.
func (n *NamespacedResourceClaimReflector) List() ([]any, error) {
	local, lerr := virtualkubelet.List[virtualkubelet.Lister[*resourcev1.ResourceClaim]](
		n.localClaims,
	)

	if lerr != nil {
		return nil, fmt.Errorf("listing local ResourceClaims: %w", lerr)
	}

	remote, rerr := virtualkubelet.List[virtualkubelet.Lister[*resourcev1.ResourceClaim]](
		n.remoteClaims,
	)
	if rerr != nil {
		return nil, fmt.Errorf("listing remote ResourceClaims: %w", rerr)
	}

	return append(local, remote...), nil
}
