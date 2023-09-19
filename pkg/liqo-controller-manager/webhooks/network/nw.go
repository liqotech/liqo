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

package nwwh

import (
	"context"
	"fmt"
	"net"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
)

type nwwh struct {
	decoder *admission.Decoder
}

type nwwhv struct {
	nwwh
}

// NewValidator returns a new Network validating webhook.
func NewValidator() *webhook.Admission {
	return &webhook.Admission{Handler: &nwwhv{
		nwwh: nwwh{
			decoder: admission.NewDecoder(runtime.NewScheme()),
		},
	}}
}

// DecodeNetwork decodes the Network from the incoming request.
func (w *nwwh) DecodeNetwork(obj runtime.RawExtension) (*ipamv1alpha1.Network, error) {
	var nw ipamv1alpha1.Network
	err := w.decoder.DecodeRaw(obj, &nw)
	return &nw, err
}

// Handle implements the Network validating webhook logic.
//
//nolint:gocritic // The signature of this method is imposed by controller runtime.
func (w *nwwhv) Handle(_ context.Context, req admission.Request) admission.Response {
	klog.V(4).Infof("Operation: %s", req.Operation)

	switch req.Operation {
	case admissionv1.Create:
		return w.HandleCreate(&req)
	case admissionv1.Update:
		return w.HandleUpdate(&req)
	default:
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unsupported operation %s", req.Operation))
	}
}

func (w *nwwhv) HandleCreate(req *admission.Request) admission.Response {
	nw, err := w.DecodeNetwork(req.Object)
	if err != nil {
		klog.Errorf("Failed decoding Network object: %v", err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Check existence of the remote clusterID label
	_, found := nw.Labels[consts.RemoteClusterID]
	if !found {
		return admission.Denied(fmt.Sprintf("Missing remote clusterID label (%q)", consts.RemoteClusterID))
	}

	// Check existence of the network CIDR
	if nw.Spec.CIDR == "" {
		return admission.Denied("Missing CIDR")
	}

	// Check if the CIDR is a valid network
	if _, _, err := net.ParseCIDR(nw.Spec.CIDR.String()); err != nil {
		return admission.Denied(fmt.Sprintf("Invalid CIDR: %v", err))
	}

	return admission.Allowed("")
}

// HandleUpdate is the function in charge of handling Update requests.
func (w *nwwhv) HandleUpdate(req *admission.Request) admission.Response {
	nwnew, err := w.DecodeNetwork(req.Object)
	if err != nil {
		klog.Errorf("Failed decoding new Network object: %v", err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	nwold, err := w.DecodeNetwork(req.OldObject)
	if err != nil {
		klog.Errorf("Failed decoding old Network object: %v", err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Check if the remote clusterID label is modified
	// We do not check the existence of the label as it should always be present
	// thank to the webhook validation of creation requests.
	if nwold.Labels[consts.RemoteClusterID] != nwnew.Labels[consts.RemoteClusterID] {
		return admission.Denied("The remote clusterID label cannot be modified after creation")
	}

	// Check if the CIDR is modified
	if nwold.Spec.CIDR != nwnew.Spec.CIDR {
		return admission.Denied("The CIDR cannot be modified after creation")
	}

	return admission.Allowed("")
}
