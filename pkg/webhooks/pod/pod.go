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

package pod

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

// cluster-role
// +kubebuilder:rbac:groups=offloading.liqo.io,resources=namespaceoffloadings,verbs=get;list;watch

type podwh struct {
	client  client.Client
	decoder admission.Decoder

	runtimeClassName string
}

// New returns a new PodWebhook instance.
func New(cl client.Client, liqoRuntimeClassName string) *webhook.Admission {
	return &webhook.Admission{Handler: &podwh{
		client:  cl,
		decoder: admission.NewDecoder(runtime.NewScheme()),

		runtimeClassName: liqoRuntimeClassName,
	}}
}

// DecodePod decodes the pod from the incoming request.
func (w *podwh) DecodePod(obj runtime.RawExtension) (*corev1.Pod, error) {
	var pod corev1.Pod
	err := w.decoder.DecodeRaw(obj, &pod)
	return &pod, err
}

// CreatePatchResponse creates an admission response with the given pod.
func (w *podwh) CreatePatchResponse(req *admission.Request, pod *corev1.Pod) admission.Response {
	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		klog.Errorf("Failed encoding pod in admission response: %v", err)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}

// Handle implements the pod mutating webhook logic.
//
//nolint:gocritic // The signature of this method is imposed by controller runtime.
func (w *podwh) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod, err := w.DecodePod(req.Object)
	if err != nil {
		klog.Errorf("Failed decoding Pod object: %v", err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Get the NamespaceOffloading associated with the pod Namespace. If there is no NamespaceOffloading for that
	// Namespace, it is an error, since the liqo.io/scheduling label should not be present on this namespace.
	nsoff := &offloadingv1beta1.NamespaceOffloading{}
	if err = w.client.Get(ctx, types.NamespacedName{
		Namespace: req.Namespace, // Using req.Namespace, as pod.Namespace appears to be unset.
		Name:      liqoconst.DefaultNamespaceOffloadingName,
	}, nsoff); err != nil {
		klog.Errorf("Failed retrieving NamespaceOffloading for namespace %q: %v", req.Namespace, err)
		return admission.Errored(http.StatusInternalServerError, errors.New("failed retrieving NamespaceOffloading"))
	}

	if err = mutatePod(nsoff, pod, w.runtimeClassName); err != nil {
		return admission.Errored(http.StatusInternalServerError, errors.New("failed constructing pod mutation"))
	}

	return w.CreatePatchResponse(&req, pod)
}
