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

package ip

import (
	"context"
	"fmt"
	"net"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

// cluster-role
// +kubebuilder:rbac:groups=ipam.liqo.io,resources=ips,verbs=get;list;watch
// +kubebuilder:rbac:groups=ipam.liqo.io,resources=networks,verbs=get;list;watch

type ipwh struct {
	client  client.Client
	decoder admission.Decoder
}

// NewValidator returns a new IP validating webhook.
func NewValidator(cl client.Client) *webhook.Admission {
	return &webhook.Admission{Handler: &ipwh{
		client:  cl,
		decoder: admission.NewDecoder(runtime.NewScheme()),
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
func (w *ipwh) Handle(ctx context.Context, req admission.Request) admission.Response {
	ip, err := w.DecodeIP(req.Object)
	if err != nil {
		klog.Errorf("Failed decoding IP object: %v", err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	switch req.Operation {
	case admissionv1.Create:
		return w.handleCreate(ctx, ip)
	case admissionv1.Update:
		return w.handleUpdate(ctx, ip)
	default:
		return admission.Allowed("")
	}
}

func (w *ipwh) handleCreate(ctx context.Context, ip *ipamv1alpha1.IP) admission.Response {
	// Check if the IP belongs to a pod CIDR.
	if err := w.validateIPBelongsToPodCIDR(ctx, ip); err != nil {
		klog.Errorf("IP %q validation failed: %v", ip.Name, err)
		return admission.Denied(err.Error())
	}
	return admission.Allowed("")
}

func (w *ipwh) handleUpdate(ctx context.Context, ip *ipamv1alpha1.IP) admission.Response {
	// For updates, only validate if the IP field has changed.
	// The IP field is immutable, so this is just a safety check.
	if err := w.validateIPBelongsToPodCIDR(ctx, ip); err != nil {
		klog.Errorf("IP %q validation failed: %v", ip.Name, err)
		return admission.Denied(err.Error())
	}
	return admission.Allowed("")
}

// validateIPBelongsToPodCIDR checks if the IP belongs to a pod CIDR.
// It returns an error if the IP is found within a pod CIDR (i.e., it is already allocated).
func (w *ipwh) validateIPBelongsToPodCIDR(ctx context.Context, ip *ipamv1alpha1.IP) error {
	ipStr := ip.Spec.IP.String()
	if ipStr == "" {
		return fmt.Errorf("IP field is empty")
	}

	// Parse the IP address.
	parsedIP := net.ParseIP(ipStr)
	if parsedIP == nil {
		return fmt.Errorf("invalid IP address %q", ipStr)
	}

	// Get the pod CIDR networks.
	podCIDRNetworks, err := getters.GetNetworksByLabel(ctx, w.client,
		labels.SelectorFromSet(map[string]string{
			consts.NetworkTypeLabelKey: string(consts.NetworkTypePodCIDR),
		}), "")
	if err != nil {
		return fmt.Errorf("failed to get pod CIDR networks: %w", err)
	}

	if len(podCIDRNetworks) == 0 {
		return fmt.Errorf("no pod CIDR networks found")
	}

	// Check if the IP belongs to any of the pod CIDRs.
	for i := range podCIDRNetworks {
		if podCIDRNetworks[i].Spec.CIDR == "" {
			continue
		}
		_, cidr, err := net.ParseCIDR(podCIDRNetworks[i].Spec.CIDR.String())
		if err != nil {
			klog.Warningf("Failed to parse CIDR %q: %v", podCIDRNetworks[i].Spec.CIDR, err)
			continue
		}
		if cidr.Contains(parsedIP) {
			return fmt.Errorf("%q cannot be remapped using IP CRD since it is already part of the pod CIDR %q", ipStr, podCIDRNetworks[i].Spec.CIDR)
		}
	}

	return nil
}
