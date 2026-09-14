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

package virtualnode

import (
	"context"
	"fmt"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
)

// cluster-role
// +kubebuilder:rbac:groups=offloading.liqo.io,resources=virtualnodes,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch

type vnwhv struct {
	client  client.Client
	decoder admission.Decoder
}

// NewValidator returns a new VirtualNode validating webhook.
func NewValidator(cl client.Client) *webhook.Admission {
	return &webhook.Admission{Handler: &vnwhv{
		client:  cl,
		decoder: admission.NewDecoder(runtime.NewScheme()),
	}}
}

// Handle implements the VirtualNode validating webhook logic.
//
//nolint:gocritic // The signature of this method is imposed by controller runtime.
func (w *vnwhv) Handle(ctx context.Context, req admission.Request) admission.Response {
	switch req.Operation {
	case admissionv1.Create:
		return w.handleCreate(ctx, &req)
	default:
		return admission.Allowed("")
	}
}

func (w *vnwhv) handleCreate(ctx context.Context, req *admission.Request) admission.Response {
	virtualnode, err := w.decodeVirtualNode(req.Object)
	if err != nil {
		klog.Errorf("Failed decoding virtualnode object: %v", err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	// The VirtualNode and the created Node share the same name: deny the creation if a
	// Node with the same name already exists, to prevent the virtual-kubelet from
	// clashing with a pre-existing node.
	if err := w.checkNodeDuplicate(ctx, virtualnode); err != nil {
		klog.Errorf("Failed checking node duplicate: %v", err)
		return admission.Denied(err.Error())
	}

	return admission.Allowed("")
}

// decodeVirtualNode decodes the virtualnode from the incoming request.
func (w *vnwhv) decodeVirtualNode(obj runtime.RawExtension) (*offloadingv1beta1.VirtualNode, error) {
	var virtualnode offloadingv1beta1.VirtualNode
	if err := w.decoder.DecodeRaw(obj, &virtualnode); err != nil {
		return nil, err
	}
	return &virtualnode, nil
}

// checkNodeDuplicate checks whether a Node with the same name already exists in the cluster.
func (w *vnwhv) checkNodeDuplicate(ctx context.Context, virtualnode *offloadingv1beta1.VirtualNode) error {
	node := &corev1.Node{}
	err := w.client.Get(ctx, client.ObjectKey{Name: virtualnode.Name}, node)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("node %s already exists", virtualnode.Name)
}
