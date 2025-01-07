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

package virtualnode

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
)

// cluster-role
// +kubebuilder:rbac:groups=offloading.liqo.io,resources=virtualnodes,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=offloading.liqo.io,resources=vkoptionstemplates,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch

type vnwh struct {
	client  client.Client
	decoder admission.Decoder

	clusterID             liqov1beta1.ClusterID
	localPodCIDR          string
	liqoNamespace         string
	vkOptsDefaultTemplate *corev1.ObjectReference
}

// New returns a new VirtualNodeWebhook instance.
func New(cl client.Client, clusterID liqov1beta1.ClusterID, localPodCIDR, liqoNamespace string,
	vkOptsDefaultTemplate *corev1.ObjectReference) *admission.Webhook {
	return &admission.Webhook{Handler: &vnwh{
		client:  cl,
		decoder: admission.NewDecoder(runtime.NewScheme()),

		clusterID:             clusterID,
		localPodCIDR:          localPodCIDR,
		liqoNamespace:         liqoNamespace,
		vkOptsDefaultTemplate: vkOptsDefaultTemplate,
	}}
}

// DecodeVirtualNode decodes the virtualnode from the incoming request.
func (w *vnwh) DecodeVirtualNode(obj runtime.RawExtension) (*offloadingv1beta1.VirtualNode, error) {
	var virtualnode offloadingv1beta1.VirtualNode
	err := w.decoder.DecodeRaw(obj, &virtualnode)
	return &virtualnode, err
}

// CreatePatchResponse creates an admission response with the given virtualnode.
func (w *vnwh) CreatePatchResponse(req *admission.Request, virtualnode *offloadingv1beta1.VirtualNode) admission.Response {
	marshaledVn, err := json.Marshal(virtualnode)
	if err != nil {
		klog.Errorf("Failed encoding virtualnode in admission response: %v", err)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledVn)
}

// Handle implements the virtualnode mutating webhook logic.
//
//nolint:gocritic // The signature of this method is imposed by controller runtime.
func (w *vnwh) Handle(ctx context.Context, req admission.Request) admission.Response {
	_ = ctx
	virtualnode, err := w.DecodeVirtualNode(req.Object)
	if err != nil {
		klog.Errorf("Failed decoding virtualnode object: %v", err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Get the VirtualKubeletOptions from the VirtualNode Spec if specified,
	// otherwise use the default template passed to the webhook.
	vkOptsRef := w.vkOptsDefaultTemplate
	if virtualnode.Spec.VkOptionsTemplateRef != nil {
		vkOptsRef = virtualnode.Spec.VkOptionsTemplateRef
	}
	nsName := types.NamespacedName{Namespace: vkOptsRef.Namespace, Name: vkOptsRef.Name}
	var vkOpts offloadingv1beta1.VkOptionsTemplate
	if err = w.client.Get(ctx, nsName, &vkOpts); err != nil {
		klog.Errorf("Failed getting VkOptionsTemplate %q: %v", nsName, err)
		return admission.Denied(err.Error())
	}

	if req.Operation == admissionv1.Create {
		// VirtualNode name and the created Node have the same name.
		// This checks if the Node already exists in the cluster to avoid duplicates.
		err := checkNodeDuplicate(ctx, w, virtualnode)
		if err != nil {
			klog.Errorf("Failed checking node duplicate: %v", err)
			return admission.Denied(err.Error())
		}

		overrideVKOptionsFromExistingVirtualNode(&vkOpts, virtualnode)
		w.initVirtualNodeDeployment(virtualnode, &vkOpts)
		mutateSpec(virtualnode, &vkOpts)
	}

	mutateSpecInTemplate(virtualnode, &vkOpts)

	return w.CreatePatchResponse(&req, virtualnode)
}

// checkNodeDuplicate checks if the node already exists in the cluster.
func checkNodeDuplicate(ctx context.Context, w *vnwh, virtualnode *offloadingv1beta1.VirtualNode) error {
	node := &corev1.Node{}
	err := w.client.Get(ctx, client.ObjectKey{Name: virtualnode.Name}, node)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return fmt.Errorf("node %s already exists", virtualnode.Name)
}
