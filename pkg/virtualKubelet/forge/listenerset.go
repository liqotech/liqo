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

package forge

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"
	gwv1apply "sigs.k8s.io/gateway-api/applyconfiguration/apis/v1"

	"github.com/liqotech/liqo/pkg/consts"
)

// ListenerSetParent returns the Gateway the given ListenerSet is attached to, and false if the parent is not a Gateway.
func ListenerSetParent(listenerSet *gwv1.ListenerSet) (types.NamespacedName, bool) {
	parent := &listenerSet.Spec.ParentRef
	if !isGatewayReference(parent.Group, parent.Kind) {
		return types.NamespacedName{}, false
	}

	gateway := types.NamespacedName{Namespace: listenerSet.GetNamespace(), Name: string(parent.Name)}
	if parent.Namespace != nil {
		gateway.Namespace = string(*parent.Namespace)
	}
	return gateway, true
}

// ListenerSetAllowed returns whether the given Gateway allows the attachment of the ListenerSets in the given namespace,
// according to its allowedListeners field (which defaults to none). Label selectors are conservatively restricted to the
// namespace of the Gateway, as the labels of the other namespaces are not known when reflecting the ListenerSets,
// consistently with the translation of the namespaces the routes can be attached from.
func ListenerSetAllowed(gateway *gwv1.Gateway, namespace string) bool {
	allowed := gateway.Spec.AllowedListeners
	if allowed == nil || allowed.Namespaces == nil || allowed.Namespaces.From == nil {
		return false
	}

	switch *allowed.Namespaces.From {
	case gwv1.NamespacesFromAll:
		return true
	case gwv1.NamespacesFromSame, gwv1.NamespacesFromSelector:
		return namespace == gateway.GetNamespace()
	default:
		return false
	}
}

// MapListenerSet returns how the given local ListenerSet is mapped to the remote cluster, that is, how its parent Gateway is
// mapped, provided that it belongs to the virtual class and it allows the attachment of the ListenerSet. Otherwise, the
// ListenerSet is either served by the local cluster (along with its Gateway) or not accepted, and it is not reflected.
func MapListenerSet(listenerSet *gwv1.ListenerSet, opts *GatewayAPIForgingOpts) GatewayMapping {
	gateway := virtualParentGateway(listenerSet, opts)
	if gateway == nil {
		return GatewayNotReflected
	}
	return MapGateway(gateway, opts)
}

// virtualParentGateway returns the local Gateway the given ListenerSet is attached to, if it belongs to the virtual class and
// it allows the attachment of the ListenerSet, and nil otherwise.
func virtualParentGateway(listenerSet *gwv1.ListenerSet, opts *GatewayAPIForgingOpts) *gwv1.Gateway {
	parent, ok := ListenerSetParent(listenerSet)
	if !ok || opts.LocalGateway == nil {
		return nil
	}

	gateway := opts.LocalGateway(parent.Namespace, parent.Name)
	if gateway == nil || string(gateway.Spec.GatewayClassName) != opts.VirtualGatewayClass ||
		!ListenerSetAllowed(gateway, listenerSet.GetNamespace()) {
		return nil
	}
	return gateway
}

// SharedGatewayPlaceholderParentRef returns the reference to the placeholder of the shared Gateway offered by the remote cluster,
// in the form of the parent of a ListenerSet, which is replaced by the remote cluster with the actual shared Gateway.
func SharedGatewayPlaceholderParentRef() gwv1.ParentGatewayReference {
	return gwv1.ParentGatewayReference{Name: consts.SharedGatewayPlaceholder}
}

// IsSharedGatewayPlaceholderParent returns whether the given parent of a ListenerSet refers to the placeholder of the shared Gateway.
// Differently from the parents of the routes, the namespace is not considered: the parent of a ListenerSet is a structure
// rather than an atomic list, hence the server-side apply of the placeholder by the consumer cluster (which does not set
// the namespace) preserves the namespace set by the remote cluster when the placeholder was previously replaced.
func IsSharedGatewayPlaceholderParent(ref *gwv1.ParentGatewayReference) bool {
	return isGatewayReference(ref.Group, ref.Kind) && ref.Name == consts.SharedGatewayPlaceholder
}

// RemoteListenerSet forges the apply patch for the reflected ListenerSet, given the local one.
// It returns ErrNotManaged if the ListenerSet is not attached to a Gateway of the virtual class allowing its attachment, and
// ErrNotReflectable if it cannot be reflected without altering its semantic. The references which have been dropped are returned as warnings.
func RemoteListenerSet(local *gwv1.ListenerSet, targetNamespace string, opts *GatewayAPIForgingOpts,
	forgingOpts *ForgingOpts) (*gwv1apply.ListenerSetApplyConfiguration, []string, error) {
	spec, warnings, err := RemoteListenerSetSpec(local, opts)
	if err != nil {
		return nil, warnings, err
	}

	var specApply gwv1apply.ListenerSetSpecApplyConfiguration
	if err := convert(spec, &specApply); err != nil {
		return nil, warnings, err
	}

	return gwv1apply.ListenerSet(local.GetName(), targetNamespace).
		WithLabels(FilterNotReflected(local.GetLabels(), forgingOpts.LabelsNotReflected)).WithLabels(ReflectionLabels()).
		WithAnnotations(routeAnnotations(local, forgingOpts)).
		WithSpec(&specApply), warnings, nil
}

// RemoteListenerSetSpec translates the spec of the local ListenerSet into the one of the reflected ListenerSet.
func RemoteListenerSetSpec(local *gwv1.ListenerSet, opts *GatewayAPIForgingOpts) (*gwv1.ListenerSetSpec, []string, error) {
	spec := local.Spec.DeepCopy()
	var warnings []string

	gateway := virtualParentGateway(local, opts)
	if gateway == nil {
		return nil, nil, ErrNotManaged
	}

	switch MapGateway(gateway, opts) {
	case GatewayNotReflected:
		return nil, nil, &ErrNotReflectable{Reason: "the parent Gateway is neither reflected nor mapped to the shared Gateway of the remote cluster"}

	case GatewayShared:
		// The remote cluster replaces the placeholder with the actual shared Gateway, which is not known by the local cluster.
		spec.ParentRef = SharedGatewayPlaceholderParentRef()

	case GatewayReflected:
		// References without namespace refer to the namespace of the ListenerSet, which is implicitly translated.
		if spec.ParentRef.Namespace != nil {
			remoteNamespace, found := opts.Mapper(string(*spec.ParentRef.Namespace))
			if !found {
				return nil, nil, &ErrNotReflectable{Reason: fmt.Sprintf("the namespace %q of the parent Gateway is not offloaded to the remote cluster",
					*spec.ParentRef.Namespace)}
			}
			spec.ParentRef.Namespace = ptr.To(gwv1.Namespace(remoteNamespace))
		}
	}

	listeners := make([]gwv1.ListenerEntry, 0, len(spec.Listeners))
	for i := range spec.Listeners {
		listener := &spec.Listeners[i]
		if warning := remoteListenerTLS(local.GetNamespace(), listener.TLS, opts.Mapper); warning != "" {
			warnings = append(warnings, fmt.Sprintf("listener %q dropped: %s", listener.Name, warning))
			continue
		}
		warnings = append(warnings, remoteAllowedRoutes(listener.Name, listener.AllowedRoutes)...)
		listeners = append(listeners, *listener)
	}

	if len(listeners) == 0 {
		return nil, warnings, &ErrNotReflectable{Reason: "no listener can be translated for the remote cluster"}
	}
	spec.Listeners = listeners

	return spec, warnings, nil
}
