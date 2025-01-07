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

package firewallconfiguration

import (
	"context"
	"encoding/json"
	"net/http"

	v1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	firewallapi "github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
)

// cluster-role
// +kubebuilder:rbac:groups=networking.liqo.io,resources=firewallconfigurations,verbs=get;list;watch;update;patch

type webhook struct {
	decoder admission.Decoder
}

type webhookMutate struct {
	webhook
}
type webhookValidate struct {
	webhook
	cl client.Client
}

// NewValidator returns a new validator for the firewallconfiguration resource.
func NewValidator(cl client.Client) *admission.Webhook {
	return &admission.Webhook{Handler: &webhookValidate{
		webhook: webhook{
			decoder: admission.NewDecoder(runtime.NewScheme()),
		},
		cl: cl,
	}}
}

// NewMutator returns a new mutator for the firewallconfiguration resource.
func NewMutator() *admission.Webhook {
	return &admission.Webhook{Handler: &webhookMutate{
		webhook: webhook{
			decoder: admission.NewDecoder(runtime.NewScheme()),
		},
	}}
}

// DecodeFirewallConfiguration decodes the firewallconfiguration from the incoming request.
func (w *webhook) DecodeFirewallConfiguration(obj runtime.RawExtension) (*networkingv1beta1.FirewallConfiguration, error) {
	var firewallConfiguration networkingv1beta1.FirewallConfiguration
	err := w.decoder.DecodeRaw(obj, &firewallConfiguration)
	return &firewallConfiguration, err
}

// CreatePatchResponse creates an admission response with the given firewallconfiguration.
func (w *webhook) CreatePatchResponse(req *admission.Request, firewallConfiguration *networkingv1beta1.FirewallConfiguration) admission.Response {
	marshaledFirewallConfiguration, err := json.Marshal(firewallConfiguration)
	if err != nil {
		klog.Errorf("Failed encoding firewallconfiguration in admission response: %v", err)
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledFirewallConfiguration)
}

// Handle implements the firewallconfiguration mutate webhook logic.
//
//nolint:gocritic // The signature of this method is imposed by controller runtime.
func (w *webhookMutate) Handle(_ context.Context, req admission.Request) admission.Response {
	firewallConfiguration, err := w.DecodeFirewallConfiguration(req.Object)
	if err != nil {
		klog.Errorf("Failed decoding FirewallConfiguration object: %v", err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	generateRuleNames(firewallConfiguration.Spec.Table.Chains)

	return w.CreatePatchResponse(&req, firewallConfiguration)
}

// Handle implements the firewallconfiguration validate webhook logic.
//
//nolint:gocritic // The signature of this method is imposed by controller runtime.
func (w *webhookValidate) Handle(ctx context.Context, req admission.Request) admission.Response {
	var err error
	var firewallConfiguration, oldFirewallConfiguration *networkingv1beta1.FirewallConfiguration
	firewallConfiguration, err = w.DecodeFirewallConfiguration(req.Object)
	if err != nil {
		klog.Errorf("Failed decoding FirewallConfiguration object: %v", err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	family := firewallConfiguration.Spec.Table.Family
	chains := firewallConfiguration.Spec.Table.Chains

	if req.Operation == v1.Update {
		oldFirewallConfiguration, err = w.DecodeFirewallConfiguration(req.OldObject)
		if err != nil {
			klog.Errorf("Failed decoding FirewallConfiguration object: %v", err)
			return admission.Errored(http.StatusBadRequest, err)
		}
		if err := checkImmutableTableName(firewallConfiguration, oldFirewallConfiguration); err != nil {
			return admission.Denied(err.Error())
		}
	}

	if err := checkUniqueTableName(ctx, w.cl, firewallConfiguration); err != nil {
		return admission.Denied(err.Error())
	}

	if err := checkUniqueChainName(chains); err != nil {
		return admission.Denied(err.Error())
	}

	for i := range chains {
		chain := chains[i]

		if err := checkChain(*family, &chain); err != nil {
			return admission.Denied(err.Error())
		}

		if err := checkRulesInChain(&chain); err != nil {
			return admission.Denied(err.Error())
		}

		switch *chain.Type {
		case firewallapi.ChainTypeNAT:
			if err := checkNatRulesInChain(&chain); err != nil {
				return admission.Denied(err.Error())
			}
		default:
		}
	}
	return admission.Allowed("")
}
