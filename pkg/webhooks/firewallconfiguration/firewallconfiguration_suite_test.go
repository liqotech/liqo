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

package firewallconfiguration_test

import (
	"encoding/json"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	firewallapi "github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

var scheme *runtime.Scheme

var _ = BeforeSuite(func() {
	scheme = runtime.NewScheme()
	testutil.LogsToGinkgoWriter()
	Expect(networkingv1beta1.AddToScheme(scheme)).To(Succeed())
})

func TestFirewallConfigurationWebhooks(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "FirewallConfiguration Webhook Suite")
}

// forgeFirewallConfiguration returns a FirewallConfiguration with a single chain of the given type containing the given rules.
func forgeFirewallConfiguration(sets []firewallapi.Set, chain *firewallapi.Chain) *networkingv1beta1.FirewallConfiguration {
	return &networkingv1beta1.FirewallConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fwcfg",
			Namespace: "default",
			UID:       uuid.NewUUID(),
		},
		Spec: networkingv1beta1.FirewallConfigurationSpec{
			Table: firewallapi.Table{
				Name:   ptr.To("test-table"),
				Family: ptr.To(firewallapi.TableFamilyIPv4),
				Sets:   sets,
				Chains: []firewallapi.Chain{*chain},
			},
		},
	}
}

func forgeFilterChain(matches ...firewallapi.Match) *firewallapi.Chain {
	return &firewallapi.Chain{
		Name:     ptr.To("filter-chain"),
		Type:     firewallapi.ChainTypeFilter,
		Hook:     ptr.To(firewallapi.ChainHookForward),
		Priority: ptr.To(firewallapi.ChainPriorityFilter),
		Policy:   ptr.To(firewallapi.ChainPolicyAccept),
		Rules: firewallapi.RulesSet{FilterRules: []firewallapi.FilterRule{{
			Name:   ptr.To("filter-rule"),
			Match:  matches,
			Action: firewallapi.ActionDrop,
		}}},
	}
}

func forgeNatChain(matches ...firewallapi.Match) *firewallapi.Chain {
	return &firewallapi.Chain{
		Name:     ptr.To("nat-chain"),
		Type:     firewallapi.ChainTypeNAT,
		Hook:     ptr.To(firewallapi.ChainHookPostrouting),
		Priority: ptr.To(firewallapi.ChainPriorityNATSource),
		Policy:   ptr.To(firewallapi.ChainPolicyAccept),
		Rules: firewallapi.RulesSet{NatRules: []firewallapi.NatRule{{
			Name:    ptr.To("nat-rule"),
			Match:   matches,
			NatType: firewallapi.NatTypeMasquerade,
		}}},
	}
}

func forgeIPMatch(op firewallapi.MatchOperation, value string, pos firewallapi.MatchPosition) firewallapi.Match {
	return firewallapi.Match{
		Op: op,
		IP: &firewallapi.MatchIP{Value: value, Position: pos},
	}
}

func generateAdmissionRequest(obj, oldObj *networkingv1beta1.FirewallConfiguration, op admissionv1.Operation) admission.Request {
	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Object:    toRawExtension(obj),
			Operation: op,
		},
	}
	if oldObj != nil {
		req.OldObject = toRawExtension(oldObj)
	}
	return req
}

func toRawExtension(obj *networkingv1beta1.FirewallConfiguration) runtime.RawExtension {
	raw, err := json.Marshal(obj)
	Expect(err).NotTo(HaveOccurred())
	return runtime.RawExtension{Raw: raw}
}
