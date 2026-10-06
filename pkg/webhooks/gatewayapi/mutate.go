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
	"encoding/json"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
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

// routeMutator replaces the placeholder of the shared Gateway in the routes reflected from the consumer clusters.
type routeMutator struct {
	client  client.Reader
	decoder admission.Decoder
}

// NewRouteMutator returns a new webhook replacing, in the routes reflected from the consumer clusters, the placeholder
// of the shared Gateway with the actual shared Gateway offered by the local cluster (i.e., the default one among the
// Gateways labeled as shared). The consumer clusters are not aware of the shared Gateways, which are hence resolved here.
func NewRouteMutator(cl client.Reader) *webhook.Admission {
	return &webhook.Admission{Handler: &routeMutator{client: cl, decoder: admission.NewDecoder(runtime.NewScheme())}}
}

// Handle implements the route mutating webhook logic.
//
//nolint:gocritic // The signature of this method is imposed by controller runtime.
func (m *routeMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	if req.Operation != admissionv1.Create && req.Operation != admissionv1.Update {
		return admission.Allowed("")
	}

	r, err := decodeRoute(m.decoder, req.Kind.Kind, req.Object)
	if err != nil {
		klog.Errorf("Failed decoding %s object: %v", req.Kind.Kind, err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	var shared *types.NamespacedName
	if hasPlaceholder(r.spec) {
		offered, err := gwutils.ListSharedGateways(ctx, m.client)
		if err != nil {
			klog.Errorf("Failed retrieving the shared Gateways: %v", err)
			return admission.Errored(http.StatusInternalServerError, err)
		}
		shared = gwutils.DefaultSharedGateway(offered)
	}

	if !MutateRoute(r.Object, r.spec, shared) {
		return admission.Allowed("")
	}

	marshaled, err := json.Marshal(r.Object)
	if err != nil {
		klog.Errorf("Failed encoding %s in admission response: %v", req.Kind.Kind, err)
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaled)
}

// hasPlaceholder returns whether the given route refers to the placeholder of the shared Gateway.
func hasPlaceholder(spec *gwv1.CommonRouteSpec) bool {
	for i := range spec.ParentRefs {
		if forge.IsSharedGatewayPlaceholder(&spec.ParentRefs[i]) {
			return true
		}
	}
	return false
}

// MutateRoute replaces the references to the placeholder of the shared Gateway with the given shared Gateway, and annotates
// the route accordingly, so that the consumer cluster can associate the status of the shared Gateway with the placeholder.
// If no Gateway is shared, the references to the placeholder are removed, as they would refer to a non-existing Gateway:
// the route is hence detached, and the consumer cluster reports it as not accepted. The annotation is removed if the route
// is no longer attached to the shared Gateway it reports. It returns whether the route has been modified.
func MutateRoute(obj metav1.Object, spec *gwv1.CommonRouteSpec, shared *types.NamespacedName) (modified bool) {
	parents := make([]gwv1.ParentReference, 0, len(spec.ParentRefs))
	added := make(map[string]bool, len(spec.ParentRefs))
	replaced := false
	for i := range spec.ParentRefs {
		parent := spec.ParentRefs[i]
		if forge.IsSharedGatewayPlaceholder(&parent) {
			modified = true
			if shared == nil {
				continue
			}
			parent = gwv1.ParentReference{Name: gwv1.ObjectName(shared.Name), Namespace: ptr.To(gwv1.Namespace(shared.Namespace))}
			replaced = true
		}

		// Multiple parents may be replaced by the same shared gateway, while parentRefs must be unique.
		if key := forge.ParentRefKey(&parent, obj.GetNamespace()); !added[key] {
			parents = append(parents, parent)
			added[key] = true
		}
	}
	spec.ParentRefs = parents

	annotations := obj.GetAnnotations()
	switch current, found := annotations[consts.SharedGatewayAnnotation]; {
	case replaced && current != shared.String():
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations[consts.SharedGatewayAnnotation] = shared.String()
		obj.SetAnnotations(annotations)
		modified = true
	case !replaced && found && !attachedTo(obj.GetNamespace(), parents, current):
		delete(annotations, consts.SharedGatewayAnnotation)
		obj.SetAnnotations(annotations)
		modified = true
	}
	return modified
}

// attachedTo returns whether the given parents include the Gateway with the given namespaced name (in the form <namespace>/<name>).
func attachedTo(routeNamespace string, parents []gwv1.ParentReference, gateway string) bool {
	for i := range parents {
		parent := &parents[i]
		if isKind(parent.Group, parent.Kind, gwv1.GroupName, gatewayKind) &&
			(types.NamespacedName{Namespace: parentNamespace(parent, routeNamespace), Name: string(parent.Name)}).String() == gateway {
			return true
		}
	}
	return false
}
