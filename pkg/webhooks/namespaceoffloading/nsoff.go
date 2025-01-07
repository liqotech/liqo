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

package nsoffwh

import (
	"context"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
)

type nsoffwh struct {
	decoder admission.Decoder
}

// New returns a new NamespaceOffloadingWebhook instance.
func New() *webhook.Admission {
	return &webhook.Admission{Handler: &nsoffwh{
		decoder: admission.NewDecoder(runtime.NewScheme()),
	}}
}

// DecodeNamespaceOffloading decodes the NamespaceOffloading from the incoming request.
func (w *nsoffwh) DecodeNamespaceOffloading(obj runtime.RawExtension) (*offloadingv1beta1.NamespaceOffloading, error) {
	var nsoff offloadingv1beta1.NamespaceOffloading
	err := w.decoder.DecodeRaw(obj, &nsoff)
	return &nsoff, err
}

// Handle implements the NamespaceOffloading validating webhook logic.
//
//nolint:gocritic // The signature of this method is imposed by controller runtime.
func (w *nsoffwh) Handle(ctx context.Context, req admission.Request) admission.Response {
	nsoff, err := w.DecodeNamespaceOffloading(req.Object)
	if err != nil {
		klog.Errorf("Failed decoding NamespaceOffloading object: %v", err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Ensure that the NamespaceOffloading has the correct name.
	if nsoff.GetName() != consts.DefaultNamespaceOffloadingName {
		return admission.Denied("NamespaceOffloading name must match " + consts.DefaultNamespaceOffloadingName)
	}

	switch req.Operation {
	case admissionv1.Create:
		return w.handleCreate(ctx, &req, nsoff)
	case admissionv1.Update:
		return w.handleUpdate(ctx, &req, nsoff)
	default:
		return admission.Allowed("")
	}
}

func (w *nsoffwh) handleCreate(_ context.Context, _ *admission.Request, nsoff *offloadingv1beta1.NamespaceOffloading) admission.Response {
	if nsoff.Spec.NamespaceMappingStrategy == offloadingv1beta1.SelectedNameMappingStrategyType &&
		nsoff.Spec.RemoteNamespaceName == "" {
		return admission.Denied("The RemoteNamespaceName value cannot be empty when using the SelectedName NamespaceMappingStrategy")
	}

	return admission.Allowed("")
}

func (w *nsoffwh) handleUpdate(_ context.Context, req *admission.Request, nsoff *offloadingv1beta1.NamespaceOffloading) admission.Response {
	var warnings []string

	// In case of updates, validate the modified fields.
	old, err := w.DecodeNamespaceOffloading(req.OldObject)
	if err != nil {
		klog.Errorf("Failed decoding NamespaceOffloading object: %v", err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	if old.Spec.NamespaceMappingStrategy != nsoff.Spec.NamespaceMappingStrategy {
		return admission.Denied("The NamespaceMappingStrategy value cannot be modified after creation")
	}

	if old.Spec.RemoteNamespaceName != nsoff.Spec.RemoteNamespaceName {
		return admission.Denied("The RemoteNamespaceName value cannot be modified after creation")
	}

	if nsoff.Spec.PodOffloadingStrategy != offloadingv1beta1.LocalAndRemotePodOffloadingStrategyType &&
		old.Spec.PodOffloadingStrategy != nsoff.Spec.PodOffloadingStrategy {
		const msg = "The PodOffloadingStrategy was mutated to a more restrictive setting: existing pods violating this policy might still be running"
		warnings = append(warnings, msg)
	}

	return admission.Allowed("").WithWarnings(warnings...)
}
