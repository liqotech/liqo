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

package resourceslice

import (
	"context"
	"fmt"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	"github.com/liqotech/liqo/internal/crdReplicator/reflection"
	"github.com/liqotech/liqo/pkg/consts"
	authetication "github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication"
	"github.com/liqotech/liqo/pkg/utils/getters"
	liqolabels "github.com/liqotech/liqo/pkg/utils/labels"
)

// cluster-role
// +kubebuilder:rbac:groups=authentication.liqo.io,resources=resourceslices,verbs=get;list;watch;

type rswh struct {
	decoder admission.Decoder
}

type rswhv struct {
	client client.Client
	rswh
}

// NewValidator returns a new ResourceSlice validating webhook.
func NewValidator(cl client.Client) *webhook.Admission {
	return &webhook.Admission{Handler: &rswhv{
		rswh: rswh{
			decoder: admission.NewDecoder(runtime.NewScheme()),
		},
		client: cl,
	}}
}

// DecodeResourceSlice decodes the ResourceSlice from the incoming request.
func (w *rswh) DecodeResourceSlice(obj runtime.RawExtension) (*authv1beta1.ResourceSlice, error) {
	var rs authv1beta1.ResourceSlice
	err := w.decoder.DecodeRaw(obj, &rs)
	return &rs, err
}

// Handle implements the ResourceSlice validating webhook logic.
//
//nolint:gocritic // The signature of this method is imposed by controller runtime.
func (w *rswhv) Handle(ctx context.Context, req admission.Request) admission.Response {
	switch req.Operation {
	case admissionv1.Create:
		return w.handleCreate(ctx, &req)
	case admissionv1.Update:
		return w.handleUpdate(ctx, &req)
	default:
		return admission.Allowed("")
	}
}

func (w *rswhv) handleCreate(ctx context.Context, req *admission.Request) admission.Response {
	rs, err := w.DecodeResourceSlice(req.Object)
	if err != nil {
		klog.Errorf("Failed decoding ResourceSlice object: %v", err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Always accept replicated ResourceSlices as a VirtualNode will not be created from those.
	if reflection.IsReplicated(rs) {
		return admission.Allowed("")
	}

	if err := checkResourceSliceDuplicate(ctx, w.client, rs.Name); err != nil {
		return admission.Denied("a resourceslice with the same name already exists in the cluster")
	}

	return admission.Allowed("")
}

func (w *rswhv) handleUpdate(_ context.Context, req *admission.Request) admission.Response {
	if !authetication.IsControlPlaneUser(req.UserInfo.Groups) {
		return admission.Allowed("")
	}

	rsnew, err := w.DecodeResourceSlice(req.Object)
	if err != nil {
		klog.Errorf("Failed decoding ResourceSlice object: %v", err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	rsold, err := w.DecodeResourceSlice(req.OldObject)
	if err != nil {
		klog.Errorf("Failed decoding ResourceSlice object: %v", err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	// can't change remoteclusterID label
	oldRemoteClusterID, oldRemoteClusterIDFound := rsold.Labels[consts.RemoteClusterID]
	newRemoteClusterID, newRemoteClusterIDFound := rsnew.Labels[consts.RemoteClusterID]

	switch {
	case oldRemoteClusterIDFound && newRemoteClusterIDFound && oldRemoteClusterID != newRemoteClusterID:
		return admission.Denied("can't change the remoteClusterID label")
	case oldRemoteClusterIDFound && !newRemoteClusterIDFound:
		return admission.Denied("can't delete the remoteClusterID label")
	case !oldRemoteClusterIDFound && newRemoteClusterIDFound:
		return admission.Denied("can't add the remoteClusterID label")
	}

	// control plane users can't change/delete/add the CordonResourceAnnotation
	oldCordonAnnotationValue, oldCordonAnnotationFound := rsold.Annotations[consts.CordonResourceAnnotation]
	newCordonAnnotationValue, newCordonAnnotationFound := rsnew.Annotations[consts.CordonResourceAnnotation]

	switch {
	case oldCordonAnnotationFound && newCordonAnnotationFound && oldCordonAnnotationValue != newCordonAnnotationValue:
		return admission.Denied(fmt.Sprintf("control plane users can't change the %s annotation", consts.CordonResourceAnnotation))
	case oldCordonAnnotationFound && !newCordonAnnotationFound:
		return admission.Denied(fmt.Sprintf("control plane users can't delete the %s annotation", consts.CordonResourceAnnotation))
	case !oldCordonAnnotationFound && newCordonAnnotationFound:
		return admission.Denied(fmt.Sprintf("control plane users can't add the %s annotation", consts.CordonResourceAnnotation))
	}

	return admission.Allowed("")
}

// checkResourceSliceDuplicate checks if the ResourceSlice already exists in the cluster.
func checkResourceSliceDuplicate(ctx context.Context, cl client.Client, name string) error {
	resSlices, err := getters.ListResourceSlicesByLabel(ctx, cl, corev1.NamespaceAll, liqolabels.LocalLabelSelector())
	if err != nil {
		return err
	}
	for i := range resSlices {
		if resSlices[i].Name == name {
			return fmt.Errorf("ResourceSlice %q already exists in the cluster", name)
		}
	}
	return nil
}
