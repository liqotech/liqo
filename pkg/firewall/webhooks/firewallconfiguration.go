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

package webhooks

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	networkingv1alpha1fw "github.com/liqotech/liqo/apis/networking/v1alpha1/firewall"
)

// cluster-role
// +kubebuilder:rbac:groups=networking.liqo.io,resources=firewallconfigrations,verbs=get;list;watch;update;patch

type fwcfgwh struct {
	decoder *admission.Decoder
}

// NewValidator returns a new validator for the firewallconfiguration resource.
func NewValidator() *admission.Webhook {
	return &admission.Webhook{Handler: &fwcfgwh{
		decoder: admission.NewDecoder(runtime.NewScheme()),
	}}
}

// DecodeFirewallConfiguration decodes the firewallconfiguration from the incoming request.
func (w *fwcfgwh) DecodeFirewallConfiguration(obj runtime.RawExtension) (*networkingv1alpha1.FirewallConfiguration, error) {
	var firewallConfiguration networkingv1alpha1.FirewallConfiguration
	err := w.decoder.DecodeRaw(obj, &firewallConfiguration)
	return &firewallConfiguration, err
}

// CreatePatchResponse creates an admission response with the given firewallconfiguration.
func (w *fwcfgwh) CreatePatchResponse(req *admission.Request, firewallConfiguration *networkingv1alpha1.FirewallConfiguration) admission.Response {
	marshaledFirewallConfiguration, err := json.Marshal(firewallConfiguration)
	if err != nil {
		klog.Errorf("Failed encoding firewallconfiguration in admission response: %v", err)
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledFirewallConfiguration)
}

// Handle implements the firewallconfiguration mutating webhook logic.
//
//nolint:gocritic // The signature of this method is imposed by controller runtime.
func (w *fwcfgwh) Handle(_ context.Context, req admission.Request) admission.Response {
	firewallConfiguration, err := w.DecodeFirewallConfiguration(req.Object)
	if err != nil {
		klog.Errorf("Failed decoding FirewallConfiguration object: %v", err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	chains := firewallConfiguration.Spec.Table.Chains

	for i := range chains {
		chaintype := chains[i].Type
		rules := chains[i].Rules

		total := totalDefinedRulesSets(rules)

		if total > 1 {
			return admission.Denied(
				fmt.Sprintf("In chain %s, have been defined %d ruleset. ", *chains[i].Name, total) +
					"You must define only one between filterRules, natRoules and routeRules.",
			)
		}

		switch *chaintype {
		case networkingv1alpha1fw.ChainTypeNAT:
			if rules.NatRules == nil {
				return admission.Denied("NAT rules must be defined when using NAT chain. Please fulfill the rules.natRules field.")
			}
		case networkingv1alpha1fw.ChainTypeFilter:
			if rules.FilterRules == nil {
				return admission.Denied("Filter rules must be defined when using Filter chain. Please fulfill the rules.filterRules field.")
			}
		case networkingv1alpha1fw.ChainTypeRoute:
			if rules.RouteRules == nil {
				return admission.Denied("Route rules must be defined when using Route chain. Please fulfill the rules.routeRules field.")
			}
		default:
			return admission.Denied("Chain type not supported")
		}
	}

	return admission.Allowed("")
}

func totalDefinedRulesSets(rules networkingv1alpha1fw.RulesSet) int {
	total := 0
	if rules.NatRules != nil {
		total++
	}
	if rules.FilterRules != nil {
		total++
	}
	if rules.RouteRules != nil {
		total++
	}
	return total
}
