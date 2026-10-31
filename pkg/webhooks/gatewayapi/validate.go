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
	"net/http"
	"slices"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/liqotech/liqo/pkg/consts"
	gwutils "github.com/liqotech/liqo/pkg/utils/gatewayapi"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

// cluster-role
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch

// validator validates the Gateway API resources reflected from the consumer clusters.
type validator struct {
	client         client.Reader
	decoder        admission.Decoder
	gatewayClasses []string
}

// NewValidator returns a new webhook validating the Gateway API resources reflected from the consumer clusters (i.e., created in
// the namespaces hosting offloaded workloads), which shall not refer to resources not belonging to the consumer cluster:
//   - the Gateways shall belong to one of the given GatewayClasses, offered to the consumer clusters;
//   - the routes shall be attached only to Gateways in the same namespace (i.e., reflected), or to shared Gateways, and to
//     Services and ListenerSets in namespaces offloaded by the same consumer cluster, and shall not refer to extension filters;
//   - the ListenerSets shall be attached only to Gateways in the same namespace, or to shared Gateways, using only the ports
//     of the listeners of the shared Gateways, and they shall refer only to certificates of the same consumer cluster.
//
// The parents of the routes are validated only when added, so that the routes already attached to a Gateway no longer
// shared can still be updated (e.g., by the controllers of the local cluster), while they cannot be attached to new ones.
func NewValidator(cl client.Reader, gatewayClasses []string) *webhook.Admission {
	return &webhook.Admission{Handler: &validator{
		client: cl, decoder: admission.NewDecoder(runtime.NewScheme()), gatewayClasses: gatewayClasses,
	}}
}

// Handle implements the validating webhook logic.
//
//nolint:gocritic // The signature of this method is imposed by controller runtime.
func (v *validator) Handle(ctx context.Context, req admission.Request) admission.Response {
	if req.Operation != admissionv1.Create && req.Operation != admissionv1.Update {
		return admission.Allowed("")
	}

	switch req.Kind.Kind {
	case gatewayKind:
		return v.handleGateway(&req)
	case httpRouteKind, grpcRouteKind:
		return v.handleRoute(ctx, &req)
	case listenerSetKind:
		return v.handleListenerSet(ctx, &req)
	default:
		return admission.Allowed("")
	}
}

func (v *validator) handleGateway(req *admission.Request) admission.Response {
	var gateway gwv1.Gateway
	if err := v.decoder.DecodeRaw(req.Object, &gateway); err != nil {
		klog.Errorf("Failed decoding Gateway object: %v", err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	if req.Operation == admissionv1.Update {
		var old gwv1.Gateway
		if err := v.decoder.DecodeRaw(req.OldObject, &old); err != nil {
			klog.Errorf("Failed decoding Gateway object: %v", err)
			return admission.Errored(http.StatusBadRequest, err)
		}
		// The GatewayClass is validated only when changed, so that the Gateways already existing can still be updated.
		if old.Spec.GatewayClassName == gateway.Spec.GatewayClassName {
			return admission.Allowed("")
		}
	}

	if !slices.Contains(v.gatewayClasses, string(gateway.Spec.GatewayClassName)) {
		return admission.Denied(fmt.Sprintf("GatewayClass %q not offered to the consumer clusters (offered: [%s])",
			gateway.Spec.GatewayClassName, strings.Join(v.gatewayClasses, ", ")))
	}
	return admission.Allowed("")
}

func (v *validator) handleRoute(ctx context.Context, req *admission.Request) admission.Response {
	r, err := decodeRoute(v.decoder, req.Kind.Kind, req.Object)
	if err != nil {
		klog.Errorf("Failed decoding %s object: %v", req.Kind.Kind, err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	if len(r.extensionRefs) > 0 {
		return admission.Denied(fmt.Sprintf("extension filters are not allowed (referenced: [%s])", strings.Join(r.extensionRefs, ", ")))
	}

	existing := make(map[string]bool)
	if req.Operation == admissionv1.Update {
		old, err := decodeRoute(v.decoder, req.Kind.Kind, req.OldObject)
		if err != nil {
			klog.Errorf("Failed decoding %s object: %v", req.Kind.Kind, err)
			return admission.Errored(http.StatusBadRequest, err)
		}
		for i := range old.spec.ParentRefs {
			existing[forge.ParentRefKey(&old.spec.ParentRefs[i], req.Namespace)] = true
		}
	}

	for i := range r.spec.ParentRefs {
		parent := &r.spec.ParentRefs[i]
		if existing[forge.ParentRefKey(parent, req.Namespace)] {
			continue
		}

		reason, err := v.validateParent(ctx, req.Namespace, parent)
		if err != nil {
			klog.Errorf("Failed validating the parentRefs of %s %s/%s: %v", req.Kind.Kind, req.Namespace, req.Name, err)
			return admission.Errored(http.StatusInternalServerError, err)
		}
		if reason != "" {
			return admission.Denied(reason)
		}
	}
	return admission.Allowed("")
}

// validateParent returns a non-empty reason if the given parent of a route in the given namespace is not allowed.
func (v *validator) validateParent(ctx context.Context, namespace string, parent *gwv1.ParentReference) (string, error) {
	parentNs := parentNamespace(parent, namespace)

	switch {
	case isKind(parent.Group, parent.Kind, gwv1.GroupName, gatewayKind):
		if parentNs == namespace {
			// The Gateways in the same namespace are the ones reflected by the consumer cluster.
			return "", nil
		}

		var gateway gwv1.Gateway
		key := types.NamespacedName{Namespace: parentNs, Name: string(parent.Name)}
		err := v.client.Get(ctx, key, &gateway)
		switch {
		case kerrors.IsNotFound(err):
			return fmt.Sprintf("parentRef to Gateway %q not allowed: the Gateway does not exist", key), nil
		case err != nil:
			return "", fmt.Errorf("failed to retrieve Gateway %q: %w", key, err)
		case !gwutils.IsSharedGateway(&gateway):
			return fmt.Sprintf("parentRef to Gateway %q not allowed: only the Gateways in the same namespace, "+
				"or the ones labeled with %q, can be referenced", key, consts.SharedGatewayLabel), nil
		}
		return "", nil

	case isKind(parent.Group, parent.Kind, corev1.GroupName, "Service"), isKind(parent.Group, parent.Kind, gwv1.GroupName, listenerSetKind):
		if parentNs == namespace {
			return "", nil
		}
		// The Services and ListenerSets shall belong to a namespace offloaded by the same consumer cluster.
		same, err := v.sameConsumer(ctx, namespace, parentNs)
		if err != nil || same {
			return "", err
		}
		return fmt.Sprintf("parentRef to %s %s/%s not allowed: the namespace is not offloaded by the same cluster",
			ptrOr(parent.Kind, gatewayKind), parentNs, parent.Name), nil

	default:
		return fmt.Sprintf("parentRef to %s %s/%s not allowed: unsupported kind", ptrOr(parent.Kind, gatewayKind), parentNs, parent.Name), nil
	}
}

func (v *validator) handleListenerSet(ctx context.Context, req *admission.Request) admission.Response {
	var listenerSet gwv1.ListenerSet
	if err := v.decoder.DecodeRaw(req.Object, &listenerSet); err != nil {
		klog.Errorf("Failed decoding ListenerSet object: %v", err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	// The parent and the ports are validated only when changed, so that the ListenerSets already attached to a Gateway
	// can still be updated if the Gateway is no longer shared (or its listeners change), as for the parents of the routes.
	var old *gwv1.ListenerSet
	if req.Operation == admissionv1.Update {
		old = &gwv1.ListenerSet{}
		if err := v.decoder.DecodeRaw(req.OldObject, old); err != nil {
			klog.Errorf("Failed decoding ListenerSet object: %v", err)
			return admission.Errored(http.StatusBadRequest, err)
		}
	}

	reason, err := v.validateListenerSet(ctx, req.Namespace, &listenerSet, old)
	if err != nil {
		klog.Errorf("Failed validating ListenerSet %s/%s: %v", req.Namespace, req.Name, err)
		return admission.Errored(http.StatusInternalServerError, err)
	}
	if reason != "" {
		return admission.Denied(reason)
	}
	return admission.Allowed("")
}

// validateListenerSet returns a non-empty reason if the given ListenerSet (whose previous version is given, if updated) is not allowed.
func (v *validator) validateListenerSet(ctx context.Context, namespace string, listenerSet, old *gwv1.ListenerSet) (string, error) {
	parent := &listenerSet.Spec.ParentRef
	if !isKind(parent.Group, parent.Kind, gwv1.GroupName, gatewayKind) {
		return fmt.Sprintf("parentRef to %s %s not allowed: unsupported kind", ptrOr(parent.Kind, gatewayKind), parent.Name), nil
	}
	gateway := types.NamespacedName{Namespace: namespace, Name: string(parent.Name)}
	if parent.Namespace != nil {
		gateway.Namespace = string(*parent.Namespace)
	}

	parentChanged := old == nil || !equality.Semantic.DeepEqual(old.Spec.ParentRef, listenerSet.Spec.ParentRef)
	if parentChanged {
		reason, err := v.validateParent(ctx, namespace, &gwv1.ParentReference{
			Group: parent.Group, Kind: parent.Kind, Namespace: parent.Namespace, Name: parent.Name})
		if err != nil || reason != "" {
			return reason, err
		}
	}

	// The certificates shall belong to a namespace offloaded by the same consumer cluster.
	for i := range listenerSet.Spec.Listeners {
		listener := &listenerSet.Spec.Listeners[i]
		if listener.TLS == nil {
			continue
		}
		for j := range listener.TLS.CertificateRefs {
			ref := &listener.TLS.CertificateRefs[j]
			if ref.Namespace == nil || string(*ref.Namespace) == namespace {
				continue
			}
			same, err := v.sameConsumer(ctx, namespace, string(*ref.Namespace))
			if err != nil {
				return "", err
			}
			if !same {
				return fmt.Sprintf("certificateRef %s/%s of listener %q not allowed: the namespace is not offloaded by the same cluster",
					*ref.Namespace, ref.Name, listener.Name), nil
			}
		}
	}

	// The ListenerSets attached to the Gateways in the same namespace (i.e., reflected) do not affect the other tenants.
	if gateway.Namespace == namespace {
		return "", nil
	}
	return v.validateListenerSetPorts(ctx, gateway, listenerSet, old, parentChanged)
}

// validateListenerSetPorts returns a non-empty reason if any listener of the given ListenerSet, attached to the given shared Gateway,
// uses a port not used by the listeners of the shared Gateway, which would be otherwise exposed on behalf of a consumer cluster.
// Only the listeners added or whose port changed are validated, unless the parent changed.
func (v *validator) validateListenerSetPorts(ctx context.Context, gateway types.NamespacedName, listenerSet, old *gwv1.ListenerSet,
	parentChanged bool) (string, error) {
	existing := make(map[gwv1.SectionName]gwv1.PortNumber)
	if old != nil && !parentChanged {
		for i := range old.Spec.Listeners {
			existing[old.Spec.Listeners[i].Name] = old.Spec.Listeners[i].Port
		}
	}

	var shared *gwv1.Gateway
	for i := range listenerSet.Spec.Listeners {
		listener := &listenerSet.Spec.Listeners[i]
		if port, found := existing[listener.Name]; found && port == listener.Port {
			continue
		}

		if shared == nil {
			shared = &gwv1.Gateway{}
			if err := v.client.Get(ctx, gateway, shared); err != nil {
				if kerrors.IsNotFound(err) {
					return fmt.Sprintf("parentRef to Gateway %q not allowed: the Gateway does not exist", gateway), nil
				}
				return "", fmt.Errorf("failed to retrieve Gateway %q: %w", gateway, err)
			}
		}

		if !slices.ContainsFunc(shared.Spec.Listeners, func(l gwv1.Listener) bool { return l.Port == listener.Port }) {
			ports := make([]string, 0, len(shared.Spec.Listeners))
			for j := range shared.Spec.Listeners {
				if port := fmt.Sprint(shared.Spec.Listeners[j].Port); !slices.Contains(ports, port) {
					ports = append(ports, port)
				}
			}
			return fmt.Sprintf("port %d of listener %q not allowed: only the ports of the listeners of shared Gateway %q can be used "+
				"(allowed: [%s])", listener.Port, listener.Name, gateway, strings.Join(ports, ", ")), nil
		}
	}
	return "", nil
}

// sameConsumer returns whether the given namespaces host the workloads offloaded by the same consumer cluster.
func (v *validator) sameConsumer(ctx context.Context, namespace, other string) (bool, error) {
	clusterIDs := make([]string, 0, 2)
	for _, name := range []string{namespace, other} {
		var ns corev1.Namespace
		if err := v.client.Get(ctx, types.NamespacedName{Name: name}, &ns); err != nil {
			if kerrors.IsNotFound(err) {
				return false, nil
			}
			return false, fmt.Errorf("failed to retrieve namespace %q: %w", name, err)
		}
		clusterID, found := ns.Labels[consts.RemoteClusterID]
		if !found || clusterID == "" {
			return false, nil
		}
		clusterIDs = append(clusterIDs, clusterID)
	}
	return clusterIDs[0] == clusterIDs[1], nil
}

func ptrOr[T ~string](value *T, fallback T) T {
	if value == nil {
		return fallback
	}
	return *value
}
