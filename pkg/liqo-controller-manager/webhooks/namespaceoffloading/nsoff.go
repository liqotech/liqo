// Copyright 2019-2023 The Liqo Authors
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

	offv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
)

type nsoffwh struct {
	decoder *admission.Decoder
}

// New returns a new NamespaceOffloadingWebhook instance.
func New() *webhook.Admission {
	return &webhook.Admission{Handler: &nsoffwh{}}
}

// InjectDecoder injects the decoder - this method is used by controller runtime.
func (w *nsoffwh) InjectDecoder(decoder *admission.Decoder) error {
	w.decoder = decoder
	return nil
}

// DecodeNamespaceOffloading decodes the NamespaceOffloading from the incoming request.
func (w *nsoffwh) DecodeNamespaceOffloading(obj runtime.RawExtension) (*offv1alpha1.NamespaceOffloading, error) {
	var nsoff offv1alpha1.NamespaceOffloading
	err := w.decoder.DecodeRaw(obj, &nsoff)
	return &nsoff, err
}

// Handle implements the NamespaceOffloading validating webhook logic.
//
//nolint:gocritic // The signature of this method is imposed by controller runtime.
func (w *nsoffwh) Handle(ctx context.Context, req admission.Request) admission.Response {
	var warnings []string

	nsoff, err := w.DecodeNamespaceOffloading(req.Object)
	if err != nil {
		klog.Errorf("Failed decoding NamespaceOffloading object: %v", err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Ensure that the NamespaceOffloading has the correct name.
	if nsoff.GetName() != consts.DefaultNamespaceOffloadingName {
		return admission.Denied("NamespaceOffloading name must match " + consts.DefaultNamespaceOffloadingName)
	}

	if req.Operation != admissionv1.Update {
		return admission.Allowed("")
	}

	// In case of updates, validate the modified fields.
	old, err := w.DecodeNamespaceOffloading(req.OldObject)
	if err != nil {
		klog.Errorf("Failed decoding NamespaceOffloading object: %v", err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	if old.Spec.NamespaceMappingStrategy != nsoff.Spec.NamespaceMappingStrategy {
		return admission.Denied("The NamespaceMappingStrategy value cannot be modified after creation")
	}

	if nsoff.Spec.PodOffloadingStrategy != offv1alpha1.LocalAndRemotePodOffloadingStrategyType &&
		old.Spec.PodOffloadingStrategy != nsoff.Spec.PodOffloadingStrategy {
		const msg = "The PodOffloadingStrategy was mutated to a more restrictive setting: existing pods violating this policy might still be running"
		warnings = append(warnings, msg)
	}

	return admission.Allowed("").WithWarnings(warnings...)
}
