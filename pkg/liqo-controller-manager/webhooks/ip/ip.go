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

package ipwh

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
)

type ipwh struct {
	decoder *admission.Decoder
}

type ipwhv struct {
	ipwh
}

// NewValidator returns a new IP validating webhook.
func NewValidator() *webhook.Admission {
	return &webhook.Admission{Handler: &ipwhv{
		ipwh: ipwh{
			decoder: admission.NewDecoder(runtime.NewScheme()),
		},
	}}
}

// DecodeIP decodes the IP from the incoming request.
func (w *ipwh) DecodeIP(obj runtime.RawExtension) (*ipamv1alpha1.IP, error) {
	var ip ipamv1alpha1.IP
	err := w.decoder.DecodeRaw(obj, &ip)
	return &ip, err
}

// Handle implements the IP validating webhook logic.
//
//nolint:gocritic // The signature of this method is imposed by controller runtime.
func (w *ipwhv) Handle(_ context.Context, req admission.Request) admission.Response {
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

func (w *ipwhv) HandleCreate(req *admission.Request) admission.Response {
	ip, err := w.DecodeIP(req.Object)
	if err != nil {
		klog.Errorf("Failed decoding IP object: %v", err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Check existence of the IP
	if ip.Spec.IP == "" {
		return admission.Denied("Missing IP")
	}

	// Check if the IP provided is a valid IP
	if ip := net.ParseIP(ip.Spec.IP); ip == nil {
		return admission.Denied(fmt.Sprintf("Invalid IP: %v", err))
	}

	return admission.Allowed("")
}

// HandleUpdate is the function in charge of handling Update requests.
func (w *ipwhv) HandleUpdate(req *admission.Request) admission.Response {
	ipnew, err := w.DecodeIP(req.Object)
	if err != nil {
		klog.Errorf("Failed decoding new IP object: %v", err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	ipold, err := w.DecodeIP(req.OldObject)
	if err != nil {
		klog.Errorf("Failed decoding old IP object: %v", err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Check if the IP is modified
	if ipold.Spec.IP != ipnew.Spec.IP {
		return admission.Denied("The IP cannot be modified after creation")
	}

	return admission.Allowed("")
}
