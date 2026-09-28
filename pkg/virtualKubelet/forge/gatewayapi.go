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
	"encoding/json"
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"
	gwv1apply "sigs.k8s.io/gateway-api/applyconfiguration/apis/v1"

	"github.com/liqotech/liqo/pkg/consts"
)

// NamespaceMapper returns the remote namespace mapped to the given local one, and whether such a mapping exists.
type NamespaceMapper func(local string) (remote string, found bool)

// coreGroupAlias is an alias of the core API group, which is accepted by the Gateway API in object references.
const coreGroupAlias = "core"

// GatewayAPIForgingOpts groups the parameters to forge the reflected Gateway API resources.
type GatewayAPIForgingOpts struct {
	// Mapper maps the local namespaces to the remote ones, to translate cross-namespace references.
	Mapper NamespaceMapper
	// SharedGateway is the Gateway offered by the remote cluster the routes are attached to, if any.
	SharedGateway *types.NamespacedName
	// IsReflectedGateway returns whether the given local Gateway is reflected to the remote cluster
	// (i.e., it belongs to the virtual GatewayClass), hence routes can be attached to the remote copy.
	IsReflectedGateway func(namespace, name string) bool
	// VirtualGatewayClass is the name of the local GatewayClass whose Gateways are reflected.
	VirtualGatewayClass string
	// RemoteGatewayClass is the name of the GatewayClass offered by the remote cluster, if any.
	RemoteGatewayClass string
}

// ErrNotReflectable is returned when an object cannot be reflected without altering its semantic
// (e.g., removing an authentication filter), or it would be meaningless (e.g., a route not attached to any parent).
type ErrNotReflectable struct {
	Reason string
}

func (e *ErrNotReflectable) Error() string { return e.Reason }

// ErrNotManaged is returned when an object is not managed by the reflection (e.g., a Gateway of a different class),
// hence it shall not be reflected, and without generating any event.
var ErrNotManaged = errors.New("object not managed by the reflection")

// RemoteHTTPRoute forges the apply patch for the reflected HTTPRoute, given the local one.
// It returns the list of references which have been dropped since they could not be translated, as warnings,
// or an ErrNotReflectable in case the route shall not be reflected at all.
func RemoteHTTPRoute(local *gwv1.HTTPRoute, targetNamespace string, opts *GatewayAPIForgingOpts,
	forgingOpts *ForgingOpts) (*gwv1apply.HTTPRouteApplyConfiguration, []string, error) {
	spec, warnings, err := RemoteHTTPRouteSpec(local.GetNamespace(), local.Spec.DeepCopy(), opts)
	if err != nil {
		return nil, warnings, err
	}

	var specApply gwv1apply.HTTPRouteSpecApplyConfiguration
	if err := convert(spec, &specApply); err != nil {
		return nil, warnings, err
	}

	return gwv1apply.HTTPRoute(local.GetName(), targetNamespace).
		WithLabels(FilterNotReflected(local.GetLabels(), forgingOpts.LabelsNotReflected)).WithLabels(ReflectionLabels()).
		WithAnnotations(FilterNotReflected(local.GetAnnotations(), forgingOpts.AnnotationsNotReflected)).
		WithSpec(&specApply), warnings, nil
}

// RemoteHTTPRouteSpec translates the spec of the local HTTPRoute (modified in place) into the one of the reflected route.
func RemoteHTTPRouteSpec(localNamespace string, spec *gwv1.HTTPRouteSpec, opts *GatewayAPIForgingOpts) (*gwv1.HTTPRouteSpec, []string, error) {
	var warnings []string

	parents, parentWarnings, err := RemoteRouteParentRefs(localNamespace, &spec.CommonRouteSpec, opts)
	warnings = append(warnings, parentWarnings...)
	if err != nil {
		return nil, warnings, err
	}
	spec.ParentRefs = parents

	for i := range spec.Rules {
		rule := &spec.Rules[i]

		filters, filterWarnings, err := remoteHTTPRouteFilters(localNamespace, rule.Filters, opts.Mapper)
		warnings = append(warnings, filterWarnings...)
		if err != nil {
			return nil, warnings, err
		}
		rule.Filters = filters

		backends := make([]gwv1.HTTPBackendRef, 0, len(rule.BackendRefs))
		for j := range rule.BackendRefs {
			backend := &rule.BackendRefs[j]
			if warning := RemoteBackendObjectReference(localNamespace, &backend.BackendObjectReference, opts.Mapper); warning != "" {
				warnings = append(warnings, warning)
				continue
			}

			backend.Filters, filterWarnings, err = remoteHTTPRouteFilters(localNamespace, backend.Filters, opts.Mapper)
			warnings = append(warnings, filterWarnings...)
			if err != nil {
				return nil, warnings, err
			}
			backends = append(backends, *backend)
		}
		rule.BackendRefs = backends
	}

	return spec, warnings, nil
}

func remoteHTTPRouteFilters(localNamespace string, filters []gwv1.HTTPRouteFilter, mapper NamespaceMapper) ([]gwv1.HTTPRouteFilter, []string, error) {
	var warnings []string
	remote := make([]gwv1.HTTPRouteFilter, 0, len(filters))
	for i := range filters {
		filter := &filters[i]
		switch filter.Type {
		case gwv1.HTTPRouteFilterExtensionRef:
			// Extension filters refer to implementation-specific resources, which might enforce security policies.
			return nil, warnings, &ErrNotReflectable{Reason: fmt.Sprintf("unsupported %s filter %q", filter.Type, filter.ExtensionRef.Name)}
		case gwv1.HTTPRouteFilterExternalAuth:
			// The authentication filter cannot be dropped, as the route would be exposed without authentication.
			if warning := RemoteBackendObjectReference(localNamespace, &filter.ExternalAuth.BackendRef, mapper); warning != "" {
				return nil, warnings, &ErrNotReflectable{Reason: fmt.Sprintf("cannot translate the %s filter: %s", filter.Type, warning)}
			}
		case gwv1.HTTPRouteFilterRequestMirror:
			// Mirroring does not alter the handling of the requests, hence the filter can be safely dropped.
			if warning := RemoteBackendObjectReference(localNamespace, &filter.RequestMirror.BackendRef, mapper); warning != "" {
				warnings = append(warnings, fmt.Sprintf("%s filter dropped: %s", filter.Type, warning))
				continue
			}
		case gwv1.HTTPRouteFilterRequestHeaderModifier, gwv1.HTTPRouteFilterResponseHeaderModifier,
			gwv1.HTTPRouteFilterRequestRedirect, gwv1.HTTPRouteFilterURLRewrite, gwv1.HTTPRouteFilterCORS:
			// These filters do not refer to other objects, hence they are reflected as is.
		}
		remote = append(remote, *filter)
	}
	return remote, warnings, nil
}

// RemoteGRPCRoute forges the apply patch for the reflected GRPCRoute, given the local one.
// It returns the list of references which have been dropped since they could not be translated, as warnings,
// or an ErrNotReflectable in case the route shall not be reflected at all.
func RemoteGRPCRoute(local *gwv1.GRPCRoute, targetNamespace string, opts *GatewayAPIForgingOpts,
	forgingOpts *ForgingOpts) (*gwv1apply.GRPCRouteApplyConfiguration, []string, error) {
	spec, warnings, err := RemoteGRPCRouteSpec(local.GetNamespace(), local.Spec.DeepCopy(), opts)
	if err != nil {
		return nil, warnings, err
	}

	var specApply gwv1apply.GRPCRouteSpecApplyConfiguration
	if err := convert(spec, &specApply); err != nil {
		return nil, warnings, err
	}

	return gwv1apply.GRPCRoute(local.GetName(), targetNamespace).
		WithLabels(FilterNotReflected(local.GetLabels(), forgingOpts.LabelsNotReflected)).WithLabels(ReflectionLabels()).
		WithAnnotations(FilterNotReflected(local.GetAnnotations(), forgingOpts.AnnotationsNotReflected)).
		WithSpec(&specApply), warnings, nil
}

// RemoteGRPCRouteSpec translates the spec of the local GRPCRoute (modified in place) into the one of the reflected route.
func RemoteGRPCRouteSpec(localNamespace string, spec *gwv1.GRPCRouteSpec, opts *GatewayAPIForgingOpts) (*gwv1.GRPCRouteSpec, []string, error) {
	var warnings []string

	parents, parentWarnings, err := RemoteRouteParentRefs(localNamespace, &spec.CommonRouteSpec, opts)
	warnings = append(warnings, parentWarnings...)
	if err != nil {
		return nil, warnings, err
	}
	spec.ParentRefs = parents

	for i := range spec.Rules {
		rule := &spec.Rules[i]

		filters, filterWarnings, err := remoteGRPCRouteFilters(localNamespace, rule.Filters, opts.Mapper)
		warnings = append(warnings, filterWarnings...)
		if err != nil {
			return nil, warnings, err
		}
		rule.Filters = filters

		backends := make([]gwv1.GRPCBackendRef, 0, len(rule.BackendRefs))
		for j := range rule.BackendRefs {
			backend := &rule.BackendRefs[j]
			if warning := RemoteBackendObjectReference(localNamespace, &backend.BackendObjectReference, opts.Mapper); warning != "" {
				warnings = append(warnings, warning)
				continue
			}

			backend.Filters, filterWarnings, err = remoteGRPCRouteFilters(localNamespace, backend.Filters, opts.Mapper)
			warnings = append(warnings, filterWarnings...)
			if err != nil {
				return nil, warnings, err
			}
			backends = append(backends, *backend)
		}
		rule.BackendRefs = backends
	}

	return spec, warnings, nil
}

func remoteGRPCRouteFilters(localNamespace string, filters []gwv1.GRPCRouteFilter, mapper NamespaceMapper) ([]gwv1.GRPCRouteFilter, []string, error) {
	var warnings []string
	remote := make([]gwv1.GRPCRouteFilter, 0, len(filters))
	for i := range filters {
		filter := &filters[i]
		switch filter.Type {
		case gwv1.GRPCRouteFilterExtensionRef:
			// Extension filters refer to implementation-specific resources, which might enforce security policies.
			return nil, warnings, &ErrNotReflectable{Reason: fmt.Sprintf("unsupported %s filter %q", filter.Type, filter.ExtensionRef.Name)}
		case gwv1.GRPCRouteFilterRequestMirror:
			// Mirroring does not alter the handling of the requests, hence the filter can be safely dropped.
			if warning := RemoteBackendObjectReference(localNamespace, &filter.RequestMirror.BackendRef, mapper); warning != "" {
				warnings = append(warnings, fmt.Sprintf("%s filter dropped: %s", filter.Type, warning))
				continue
			}
		case gwv1.GRPCRouteFilterRequestHeaderModifier, gwv1.GRPCRouteFilterResponseHeaderModifier:
			// These filters do not refer to other objects, hence they are reflected as is.
		}
		remote = append(remote, *filter)
	}
	return remote, warnings, nil
}

// RemoteRouteParentRefs translates the parent references of a route into the ones of the reflected route.
// References to reflected Gateways are translated to their remote copies, while the ones to other Gateways are replaced
// by the shared Gateway offered by the remote cluster (if any). References to Services (i.e., mesh routes) are translated
// to the corresponding remote namespace. An ErrNotReflectable is returned if the resulting route would not be attached to any parent.
func RemoteRouteParentRefs(localNamespace string, spec *gwv1.CommonRouteSpec, opts *GatewayAPIForgingOpts) ([]gwv1.ParentReference, []string, error) {
	var warnings []string
	remote := make([]gwv1.ParentReference, 0, len(spec.ParentRefs))
	added := make(map[string]bool, len(spec.ParentRefs))

	for i := range spec.ParentRefs {
		parent, warning := RemoteParentRef(localNamespace, spec.ParentRefs[i], opts)
		if warning != "" {
			warnings = append(warnings, warning)
			continue
		}

		// Multiple parents may be replaced by the same shared gateway, while parentRefs must be unique.
		if key := parentRefKey(&parent, ""); !added[key] {
			remote = append(remote, parent)
			added[key] = true
		}
	}

	if len(remote) == 0 && spec.UseDefaultGateways == "" {
		return nil, warnings, &ErrNotReflectable{Reason: "no parentRef can be translated for the remote cluster"}
	}
	return remote, warnings, nil
}

// RemoteParentRef translates a single parent reference of a route into the one of the reflected route.
// It returns a non-empty string, describing the reason, if the reference cannot be translated, and shall be dropped.
func RemoteParentRef(localNamespace string, parent gwv1.ParentReference, opts *GatewayAPIForgingOpts) (remote gwv1.ParentReference, warning string) {
	switch {
	case isGatewayReference(parent.Group, parent.Kind):
		// Gateways of the virtual class are reflected, hence the route can be attached to the remote copy.
		if remoteParent, ok := remoteReflectedGateway(localNamespace, parent, opts); ok {
			return remoteParent, ""
		}

		if opts.SharedGateway == nil {
			warning = fmt.Sprintf("parentRef %s dropped: no shared Gateway offered by the remote cluster", parentRefString(localNamespace, &parent))
			return parent, warning
		}
		return gwv1.ParentReference{
			Name:      gwv1.ObjectName(opts.SharedGateway.Name),
			Namespace: ptr.To(gwv1.Namespace(opts.SharedGateway.Namespace)),
		}, ""

	case isServiceReference(parent.Group, parent.Kind):
		if parent.Namespace != nil {
			remoteNamespace, found := opts.Mapper(string(*parent.Namespace))
			if !found {
				warning = fmt.Sprintf("parentRef %s dropped: namespace %q not offloaded to the remote cluster",
					parentRefString(localNamespace, &parent), *parent.Namespace)
				return parent, warning
			}
			parent.Namespace = ptr.To(gwv1.Namespace(remoteNamespace))
		}
		return parent, ""

	default:
		warning = fmt.Sprintf("parentRef %s dropped: unsupported parent kind", parentRefString(localNamespace, &parent))
		return parent, warning
	}
}

// LocalRouteParentStatuses translates the statuses of a reflected route with respect to its parents into the ones
// of the local route, associating each remote parent with the local parents it has been translated from.
// Remote parents not derived from any local parent (e.g., added by other entities) are ignored.
func LocalRouteParentStatuses(localNamespace, remoteNamespace string, localParents []gwv1.ParentReference,
	remote []gwv1.RouteParentStatus, opts *GatewayAPIForgingOpts) []gwv1.RouteParentStatus {
	// Multiple local parents may be translated to the same remote one (i.e., the shared gateway).
	origins := make(map[string][]gwv1.ParentReference, len(localParents))
	for i := range localParents {
		parent, warning := RemoteParentRef(localNamespace, localParents[i], opts)
		if warning != "" {
			continue
		}
		key := parentRefKey(&parent, remoteNamespace)
		origins[key] = append(origins[key], localParents[i])
	}

	var local []gwv1.RouteParentStatus
	for i := range remote {
		for _, parent := range origins[parentRefKey(&remote[i].ParentRef, remoteNamespace)] {
			status := remote[i].DeepCopy()
			status.ParentRef = *parent.DeepCopy()
			local = append(local, *status)
		}
	}
	return local
}

// parentRefKey returns a key identifying the given parent reference, normalizing the default values.
func parentRefKey(ref *gwv1.ParentReference, routeNamespace string) string {
	group, kind, namespace := gwv1.Group(gwv1.GroupName), gwv1.Kind("Gateway"), gwv1.Namespace(routeNamespace)
	if ref.Group != nil {
		group = *ref.Group
	}
	if ref.Kind != nil {
		kind = *ref.Kind
	}
	if ref.Namespace != nil {
		namespace = *ref.Namespace
	}
	return fmt.Sprintf("%s/%s/%s/%s/%s/%d", group, kind, namespace, ref.Name,
		ptr.Deref(ref.SectionName, ""), ptr.Deref(ref.Port, 0))
}

// remoteReflectedGateway translates the parent reference to a reflected Gateway, if the parent is reflected to the remote cluster.
func remoteReflectedGateway(localNamespace string, parent gwv1.ParentReference, opts *GatewayAPIForgingOpts) (gwv1.ParentReference, bool) {
	namespace := localNamespace
	if parent.Namespace != nil {
		namespace = string(*parent.Namespace)
	}

	if opts.IsReflectedGateway == nil || !opts.IsReflectedGateway(namespace, string(parent.Name)) {
		return parent, false
	}

	remoteNamespace, found := opts.Mapper(namespace)
	if !found {
		return parent, false
	}

	// References without namespace refer to the namespace of the route, which is implicitly translated.
	if parent.Namespace != nil {
		parent.Namespace = ptr.To(gwv1.Namespace(remoteNamespace))
	}
	return parent, true
}

// RemoteGateway forges the apply patch for the reflected Gateway, given the local one.
// It returns ErrNotManaged if the Gateway does not belong to the virtual GatewayClass, and ErrNotReflectable
// if it cannot be reflected without altering its semantic. The references which have been dropped are returned as warnings.
func RemoteGateway(local *gwv1.Gateway, targetNamespace string, opts *GatewayAPIForgingOpts,
	forgingOpts *ForgingOpts) (*gwv1apply.GatewayApplyConfiguration, []string, error) {
	if string(local.Spec.GatewayClassName) != opts.VirtualGatewayClass {
		return nil, nil, ErrNotManaged
	}

	spec, warnings, err := RemoteGatewaySpec(local.GetNamespace(), local.Spec.DeepCopy(), opts)
	if err != nil {
		return nil, warnings, err
	}

	var specApply gwv1apply.GatewaySpecApplyConfiguration
	if err := convert(spec, &specApply); err != nil {
		return nil, warnings, err
	}

	return gwv1apply.Gateway(local.GetName(), targetNamespace).
		WithLabels(FilterNotReflected(local.GetLabels(), forgingOpts.LabelsNotReflected)).WithLabels(ReflectionLabels()).
		WithAnnotations(FilterNotReflected(local.GetAnnotations(), forgingOpts.AnnotationsNotReflected)).
		WithSpec(&specApply), warnings, nil
}

// RemoteGatewaySpec translates the spec of the local Gateway (modified in place) into the one of the reflected Gateway.
func RemoteGatewaySpec(localNamespace string, spec *gwv1.GatewaySpec, opts *GatewayAPIForgingOpts) (*gwv1.GatewaySpec, []string, error) {
	var warnings []string

	if opts.RemoteGatewayClass == "" {
		return nil, nil, &ErrNotReflectable{Reason: "no GatewayClass offered by the remote cluster"}
	}
	spec.GatewayClassName = gwv1.ObjectName(opts.RemoteGatewayClass)

	// The TLS configuration may enforce the authentication of clients and backends, hence it cannot be dropped.
	if spec.TLS != nil {
		return nil, nil, &ErrNotReflectable{Reason: "the Gateway-level TLS configuration is not supported"}
	}

	// The following fields refer to resources of the local cluster, or would affect other tenants of the remote cluster.
	if len(spec.Addresses) > 0 {
		warnings = append(warnings, "addresses dropped: they refer to the local cluster")
		spec.Addresses = nil
	}
	if spec.Infrastructure != nil && spec.Infrastructure.ParametersRef != nil {
		warnings = append(warnings, "infrastructure parametersRef dropped: it refers to the local cluster")
		spec.Infrastructure.ParametersRef = nil
	}
	if spec.AllowedListeners != nil {
		warnings = append(warnings, "allowedListeners dropped: ListenerSets are not reflected")
		spec.AllowedListeners = nil
	}
	if spec.DefaultScope != "" {
		warnings = append(warnings, "defaultScope dropped: the Gateway would be the default one for the routes of the whole remote cluster")
		spec.DefaultScope = ""
	}

	listeners := make([]gwv1.Listener, 0, len(spec.Listeners))
	for i := range spec.Listeners {
		listener := &spec.Listeners[i]
		if warning := remoteListenerTLS(localNamespace, listener.TLS, opts.Mapper); warning != "" {
			warnings = append(warnings, fmt.Sprintf("listener %q dropped: %s", listener.Name, warning))
			continue
		}
		warnings = append(warnings, remoteListenerAllowedRoutes(listener)...)
		listeners = append(listeners, *listener)
	}

	if len(listeners) == 0 {
		return nil, warnings, &ErrNotReflectable{Reason: "no listener can be translated for the remote cluster"}
	}
	spec.Listeners = listeners

	return spec, warnings, nil
}

// remoteListenerTLS translates the certificate references of a listener (modified in place). It returns a non-empty string,
// describing the reason, if any reference cannot be translated, as the listener would be invalid in the remote cluster.
func remoteListenerTLS(localNamespace string, tls *gwv1.ListenerTLSConfig, mapper NamespaceMapper) string {
	if tls == nil {
		return ""
	}

	for i := range tls.CertificateRefs {
		ref := &tls.CertificateRefs[i]
		if !isSecretReference(ref.Group, ref.Kind) {
			return fmt.Sprintf("certificateRef %s has unsupported kind", secretRefString(localNamespace, ref))
		}
		if ref.Namespace == nil {
			continue
		}

		remoteNamespace, found := mapper(string(*ref.Namespace))
		if !found {
			return fmt.Sprintf("certificateRef %s refers to namespace %q not offloaded to the remote cluster",
				secretRefString(localNamespace, ref), *ref.Namespace)
		}
		ref.Namespace = ptr.To(gwv1.Namespace(remoteNamespace))
	}
	return ""
}

// remoteListenerAllowedRoutes translates the namespaces the routes can be attached from (modified in place).
// Label selectors refer to the local namespaces, hence they are replaced with the restriction to the same namespace.
func remoteListenerAllowedRoutes(listener *gwv1.Listener) []string {
	if listener.AllowedRoutes == nil || listener.AllowedRoutes.Namespaces == nil || listener.AllowedRoutes.Namespaces.From == nil {
		return nil
	}

	namespaces := listener.AllowedRoutes.Namespaces
	switch *namespaces.From {
	case gwv1.NamespacesFromAll:
		// Restrict the attachment to the remote namespaces associated with the local cluster, which are equivalent to
		// all the local namespaces, since the ones not offloaded do not have a remote counterpart.
		namespaces.From = ptr.To(gwv1.NamespacesFromSelector)
		namespaces.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{consts.RemoteClusterID: string(LocalCluster)}}
	case gwv1.NamespacesFromSelector:
		namespaces.From = ptr.To(gwv1.NamespacesFromSame)
		namespaces.Selector = nil
		return []string{fmt.Sprintf("listener %q restricted to routes in the same namespace: label selectors cannot be translated", listener.Name)}
	case gwv1.NamespacesFromSame, gwv1.NamespacesFromNone:
		// The attachment does not depend on other namespaces, hence it is reflected as is.
	}
	return nil
}

// RemoteReferenceGrant forges the apply patch for the reflected ReferenceGrant, given the local one.
// It returns ErrNotManaged if none of the namespaces the references are granted from is offloaded to the remote cluster.
func RemoteReferenceGrant(local *gwv1.ReferenceGrant, targetNamespace string, opts *GatewayAPIForgingOpts,
	forgingOpts *ForgingOpts) (*gwv1apply.ReferenceGrantApplyConfiguration, []string, error) {
	spec := local.Spec.DeepCopy()

	from := make([]gwv1.ReferenceGrantFrom, 0, len(spec.From))
	for i := range spec.From {
		// Namespaces not offloaded to the remote cluster do not have a remote counterpart, hence they are silently dropped.
		if remoteNamespace, found := opts.Mapper(string(spec.From[i].Namespace)); found {
			spec.From[i].Namespace = gwv1.Namespace(remoteNamespace)
			from = append(from, spec.From[i])
		}
	}

	if len(from) == 0 {
		return nil, nil, ErrNotManaged
	}
	spec.From = from

	var specApply gwv1apply.ReferenceGrantSpecApplyConfiguration
	if err := convert(spec, &specApply); err != nil {
		return nil, nil, err
	}

	return gwv1apply.ReferenceGrant(local.GetName(), targetNamespace).
		WithLabels(FilterNotReflected(local.GetLabels(), forgingOpts.LabelsNotReflected)).WithLabels(ReflectionLabels()).
		WithAnnotations(FilterNotReflected(local.GetAnnotations(), forgingOpts.AnnotationsNotReflected)).
		WithSpec(&specApply), nil, nil
}

// RemoteBackendObjectReference translates the given backend reference (modified in place) into the one of the reflected route.
// It returns a non-empty string, describing the reason, if the reference cannot be translated, and shall be dropped.
func RemoteBackendObjectReference(localNamespace string, ref *gwv1.BackendObjectReference, mapper NamespaceMapper) string {
	if !isServiceReference(ref.Group, ref.Kind) {
		return fmt.Sprintf("backendRef %s dropped: unsupported backend kind", backendRefString(localNamespace, ref))
	}

	// References without namespace refer to the namespace of the route, which is implicitly translated.
	if ref.Namespace == nil {
		return ""
	}

	remoteNamespace, found := mapper(string(*ref.Namespace))
	if !found {
		return fmt.Sprintf("backendRef %s dropped: namespace %q not offloaded to the remote cluster", backendRefString(localNamespace, ref), *ref.Namespace)
	}
	ref.Namespace = ptr.To(gwv1.Namespace(remoteNamespace))
	return ""
}

func isGatewayReference(group *gwv1.Group, kind *gwv1.Kind) bool {
	// The group and kind of parentRefs default to gateway.networking.k8s.io and Gateway, respectively.
	return (group == nil || *group == gwv1.GroupName) && (kind == nil || *kind == "Gateway")
}

func isServiceReference(group *gwv1.Group, kind *gwv1.Kind) bool {
	// The group and kind of backendRefs default to the core group and Service, respectively.
	return (group == nil || *group == "" || *group == coreGroupAlias) && (kind == nil || *kind == "Service")
}

func isSecretReference(group *gwv1.Group, kind *gwv1.Kind) bool {
	// The group and kind of certificateRefs default to the core group and Secret, respectively.
	return (group == nil || *group == "" || *group == coreGroupAlias) && (kind == nil || *kind == "Secret")
}

func secretRefString(localNamespace string, ref *gwv1.SecretObjectReference) string {
	return refString(localNamespace, ref.Group, ref.Kind, ref.Namespace, ref.Name, "", "Secret")
}

func parentRefString(localNamespace string, ref *gwv1.ParentReference) string {
	return refString(localNamespace, ref.Group, ref.Kind, ref.Namespace, ref.Name, gwv1.GroupName, "Gateway")
}

func backendRefString(localNamespace string, ref *gwv1.BackendObjectReference) string {
	return refString(localNamespace, ref.Group, ref.Kind, ref.Namespace, ref.Name, "", "Service")
}

func refString(localNamespace string, group *gwv1.Group, kind *gwv1.Kind, namespace *gwv1.Namespace, name gwv1.ObjectName,
	defGroup gwv1.Group, defKind gwv1.Kind) string {
	g, k, ns := defGroup, defKind, gwv1.Namespace(localNamespace)
	if group != nil {
		g = *group
	}
	if kind != nil {
		k = *kind
	}
	if namespace != nil {
		ns = *namespace
	}

	if g == "" {
		return fmt.Sprintf("%s %s/%s", k, ns, name)
	}
	return fmt.Sprintf("%s.%s %s/%s", k, g, ns, name)
}

// convert converts a typed object into the corresponding apply configuration, leveraging the fact that they share the same JSON schema.
func convert(from, to interface{}) error {
	data, err := json.Marshal(from)
	if err != nil {
		return fmt.Errorf("failed to marshal the object: %w", err)
	}
	if err := json.Unmarshal(data, to); err != nil {
		return fmt.Errorf("failed to unmarshal the apply configuration: %w", err)
	}
	return nil
}
