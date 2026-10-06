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
//     Services in namespaces offloaded by the same consumer cluster, and shall not refer to extension filters.
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

	case isKind(parent.Group, parent.Kind, corev1.GroupName, "Service"):
		if parentNs == namespace {
			return "", nil
		}
		// The Services shall belong to a namespace offloaded by the same consumer cluster.
		same, err := v.sameConsumer(ctx, namespace, parentNs)
		if err != nil || same {
			return "", err
		}
		return fmt.Sprintf("parentRef to Service %s/%s not allowed: the namespace is not offloaded by the same cluster",
			parentNs, parent.Name), nil

	default:
		return fmt.Sprintf("parentRef to %s %s/%s not allowed: unsupported kind", ptrOr(parent.Kind, gatewayKind), parentNs, parent.Name), nil
	}
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
