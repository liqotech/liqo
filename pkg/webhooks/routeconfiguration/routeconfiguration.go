// Copyright 2019-2025 The Liqo Authors
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

package routeconfiguration

import (
	"context"
	"encoding/json"
	"net/http"

	v1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
)

// cluster-role
// +kubebuilder:rbac:groups=networking.liqo.io,resources=routeconfigurations,verbs=get;list;watch;update;patch

type webhook struct {
	decoder admission.Decoder
	cl      client.Client
}

// NewValidator returns a new validator for the routeconfiguration resource.
func NewValidator(cl client.Client) *admission.Webhook {
	return &admission.Webhook{Handler: &webhook{
		decoder: admission.NewDecoder(runtime.NewScheme()),
		cl:      cl,
	}}
}

// DecodeRouteConfiguration decodes the routeconfiguration from the incoming request.
func (w *webhook) DecodeRouteConfiguration(obj runtime.RawExtension) (*networkingv1beta1.RouteConfiguration, error) {
	var routeConfiguration networkingv1beta1.RouteConfiguration
	err := w.decoder.DecodeRaw(obj, &routeConfiguration)
	return &routeConfiguration, err
}

// CreatePatchResponse creates an admission response with the given routeconfiguration.
func (w *webhook) CreatePatchResponse(req *admission.Request, routeConfiguration *networkingv1beta1.RouteConfiguration) admission.Response {
	marshaledRouteConfiguration, err := json.Marshal(routeConfiguration)
	if err != nil {
		klog.Errorf("Failed encoding routeconfiguration in admission response: %v", err)
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledRouteConfiguration)
}

// Handle implements the routeconfiguration validate webhook logic.
//
//nolint:gocritic // The signature of this method is imposed by controller runtime.
func (w *webhook) Handle(ctx context.Context, req admission.Request) admission.Response {
	routeconfiguration, err := w.DecodeRouteConfiguration(req.Object)
	if err != nil {
		klog.Errorf("Failed decoding RouteConfiguration object: %v", err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	if req.Operation == v1.Update {
		oldrouteconfiguration, err := w.DecodeRouteConfiguration(req.OldObject)
		if err != nil {
			klog.Errorf("Failed decoding old RouteConfiguration object: %v", err)
			return admission.Errored(http.StatusBadRequest, err)
		}

		if err := checkImmutableTableName(routeconfiguration, oldrouteconfiguration); err != nil {
			return admission.Denied(err.Error())
		}
	}

	if err := checkUniqueTableName(ctx, w.cl, routeconfiguration); err != nil {
		return admission.Denied(err.Error())
	}

	if err := checkUniqueRules(routeconfiguration.Spec.Table.Rules); err != nil {
		return admission.Denied(err.Error())
	}

	for i := range routeconfiguration.Spec.Table.Rules {
		if err := checkUniqueRoutes(routeconfiguration.Spec.Table.Rules[i].Routes); err != nil {
			return admission.Denied(err.Error())
		}
	}

	return admission.Allowed("")
}
