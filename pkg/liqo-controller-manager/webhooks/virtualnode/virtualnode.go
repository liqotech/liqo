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

package virtualnode

import (
	"context"
	"encoding/json"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	virtualkubeletv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	vkforge "github.com/liqotech/liqo/pkg/vkMachinery/forge"
)

// cluster-role
// +kubebuilder:rbac:groups=virtualkubelet.liqo.io,resources=virtualnode,verbs=get;list;watch;update;patch

type vnwh struct {
	clusterIdentity       *discoveryv1alpha1.ClusterIdentity
	virtualKubeletOptions *vkforge.VirtualKubeletOpts
	client                client.Client
	decoder               *admission.Decoder
}

// New returns a new VirtualNodeWebhook instance.
func New(cl client.Client, clusterIdentity *discoveryv1alpha1.ClusterIdentity,
	virtualKubeletOptions *vkforge.VirtualKubeletOpts) *admission.Webhook {
	return &admission.Webhook{Handler: &vnwh{
		client:                cl,
		clusterIdentity:       clusterIdentity,
		virtualKubeletOptions: virtualKubeletOptions,
	}}
}

// InjectDecoder injects the decoder - this method is used by controller runtime.
func (w *vnwh) InjectDecoder(decoder *admission.Decoder) error {
	w.decoder = decoder
	return nil
}

// DecodeVirtualNode decodes the pod from the incoming request.
func (w *vnwh) DecodeVirtualNode(obj runtime.RawExtension) (*virtualkubeletv1alpha1.VirtualNode, error) {
	var virtualnode virtualkubeletv1alpha1.VirtualNode
	err := w.decoder.DecodeRaw(obj, &virtualnode)
	return &virtualnode, err
}

// CreatePatchResponse creates an admission response with the given pod.
func (w *vnwh) CreatePatchResponse(req *admission.Request, virtualnode *virtualkubeletv1alpha1.VirtualNode) admission.Response {
	marshaledPod, err := json.Marshal(virtualnode)
	if err != nil {
		klog.Errorf("Failed encoding pod in admission response: %v", err)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}

// Handle implements the virtualnode mutating webhook logic.
//
//nolint:gocritic // The signature of this method is imposed by controller runtime.
func (w *vnwh) Handle(ctx context.Context, req admission.Request) admission.Response {
	_ = ctx
	virtualnode, err := w.DecodeVirtualNode(req.Object)
	if err != nil {
		klog.Errorf("Failed decoding Pod object: %v", err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	if req.Operation == admissionv1.Create {
		customizeVKOptions(w.virtualKubeletOptions, virtualnode)
		w.initVirtualNode(virtualnode)
	}

	enforceSpecInTemplate(virtualnode)

	marshaledVn, err := json.Marshal(virtualnode)
	if err != nil {
		klog.Errorf("Failed encoding pod in admission response: %v", err)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledVn)
}
