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
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	httpRouteKind = "HTTPRoute"
	grpcRouteKind = "GRPCRoute"
	gatewayKind   = "Gateway"
)

// route groups the fields of a route (either HTTPRoute or GRPCRoute) handled by the webhooks.
type route struct {
	client.Object
	// spec is the common spec of the route, including the parent references.
	spec *gwv1.CommonRouteSpec
	// extensionRefs are the names of the extension filters referenced by the route.
	extensionRefs []string
}

// decodeRoute decodes the route of the given kind from the given raw object.
func decodeRoute(decoder admission.Decoder, kind string, raw runtime.RawExtension) (*route, error) {
	switch kind {
	case httpRouteKind:
		var httpRoute gwv1.HTTPRoute
		if err := decoder.DecodeRaw(raw, &httpRoute); err != nil {
			return nil, err
		}

		var refs []string
		for i := range httpRoute.Spec.Rules {
			rule := &httpRoute.Spec.Rules[i]
			refs = append(refs, httpExtensionRefs(rule.Filters)...)
			for j := range rule.BackendRefs {
				refs = append(refs, httpExtensionRefs(rule.BackendRefs[j].Filters)...)
			}
		}
		return &route{Object: &httpRoute, spec: &httpRoute.Spec.CommonRouteSpec, extensionRefs: refs}, nil

	case grpcRouteKind:
		var grpcRoute gwv1.GRPCRoute
		if err := decoder.DecodeRaw(raw, &grpcRoute); err != nil {
			return nil, err
		}

		var refs []string
		for i := range grpcRoute.Spec.Rules {
			rule := &grpcRoute.Spec.Rules[i]
			refs = append(refs, grpcExtensionRefs(rule.Filters)...)
			for j := range rule.BackendRefs {
				refs = append(refs, grpcExtensionRefs(rule.BackendRefs[j].Filters)...)
			}
		}
		return &route{Object: &grpcRoute, spec: &grpcRoute.Spec.CommonRouteSpec, extensionRefs: refs}, nil

	default:
		return nil, fmt.Errorf("unsupported kind %q", kind)
	}
}

func httpExtensionRefs(filters []gwv1.HTTPRouteFilter) []string {
	var refs []string
	for i := range filters {
		if filters[i].Type == gwv1.HTTPRouteFilterExtensionRef && filters[i].ExtensionRef != nil {
			refs = append(refs, string(filters[i].ExtensionRef.Name))
		}
	}
	return refs
}

func grpcExtensionRefs(filters []gwv1.GRPCRouteFilter) []string {
	var refs []string
	for i := range filters {
		if filters[i].Type == gwv1.GRPCRouteFilterExtensionRef && filters[i].ExtensionRef != nil {
			refs = append(refs, string(filters[i].ExtensionRef.Name))
		}
	}
	return refs
}

// isKind returns whether the given (possibly defaulted) group and kind of a reference match the given ones.
func isKind(group *gwv1.Group, kind *gwv1.Kind, expectedGroup gwv1.Group, expectedKind gwv1.Kind) bool {
	actualGroup, actualKind := gwv1.Group(gwv1.GroupName), gwv1.Kind(gatewayKind)
	if group != nil {
		actualGroup = *group
	}
	if kind != nil {
		actualKind = *kind
	}
	return actualGroup == expectedGroup && actualKind == expectedKind
}

// parentNamespace returns the namespace of the given parent reference, defaulting to the one of the route.
func parentNamespace(ref *gwv1.ParentReference, routeNamespace string) string {
	if ref.Namespace != nil {
		return string(*ref.Namespace)
	}
	return routeNamespace
}
