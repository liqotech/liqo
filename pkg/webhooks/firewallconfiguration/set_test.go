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
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	firewallapi "github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
	fwcfgwh "github.com/liqotech/liqo/pkg/webhooks/firewallconfiguration"
)

var _ = Describe("FirewallConfiguration validating webhook with named sets", func() {
	var validator *admission.Webhook

	ipSet := func(name string, keyType firewallapi.SetDataType, keys ...string) firewallapi.Set {
		set := firewallapi.Set{Name: name, KeyType: keyType}
		for _, k := range keys {
			set.Elements = append(set.Elements, firewallapi.SetElement{Key: k})
		}
		return set
	}

	validate := func(fwcfg *networkingv1beta1.FirewallConfiguration) admission.Response {
		return validator.Handle(context.TODO(), generateAdmissionRequest(fwcfg, nil, admissionv1.Create))
	}

	BeforeEach(func() {
		validator = fwcfgwh.NewValidator(fake.NewClientBuilder().WithScheme(scheme).Build())
	})

	Context("sets definition", func() {
		It("should allow a configuration without sets", func() {
			fwcfg := forgeFirewallConfiguration(nil,
				forgeFilterChain(forgeIPMatch(firewallapi.MatchOperationEq, "10.0.0.1", firewallapi.MatchPositionSrc)))
			Expect(validate(fwcfg).Allowed).To(BeTrue())
		})

		It("should allow sets of all supported key types", func() {
			fwcfg := forgeFirewallConfiguration([]firewallapi.Set{
				ipSet("addrs", firewallapi.SetDataTypeIPAddr, "10.0.0.1", "10.0.0.2"),
				ipSet("cidrs", firewallapi.SetDataTypeIPCIDR, "10.0.0.0/24", "192.168.1.1/32"),
				ipSet("ints", firewallapi.SetDataTypeInteger, "1", "65535"),
				ipSet("empty", firewallapi.SetDataTypeIPAddr),
			}, forgeFilterChain(forgeIPMatch(firewallapi.MatchOperationEq, "@addrs", firewallapi.MatchPositionSrc)))
			resp := validate(fwcfg)
			Expect(resp.Allowed).To(BeTrue(), resp.Result.Message)
		})

		It("should allow a map with ipv4_addr data", func() {
			fwcfg := forgeFirewallConfiguration([]firewallapi.Set{{
				Name:     "map",
				KeyType:  firewallapi.SetDataTypeIPAddr,
				DataType: ptr.To(firewallapi.SetDataTypeIPAddr),
				Elements: []firewallapi.SetElement{{Key: "10.0.0.1", Data: ptr.To("192.168.0.1")}},
			}}, forgeFilterChain(forgeIPMatch(firewallapi.MatchOperationEq, "10.0.0.1", firewallapi.MatchPositionSrc)))
			resp := validate(fwcfg)
			Expect(resp.Allowed).To(BeTrue(), resp.Result.Message)
		})

		DescribeTable("should deny invalid sets",
			func(sets []firewallapi.Set, errSubstring string) {
				fwcfg := forgeFirewallConfiguration(sets,
					forgeFilterChain(forgeIPMatch(firewallapi.MatchOperationEq, "10.0.0.1", firewallapi.MatchPositionSrc)))
				resp := validate(fwcfg)
				Expect(resp.Allowed).To(BeFalse())
				Expect(resp.Result.Message).To(ContainSubstring(errSubstring))
			},
			Entry("duplicated set name",
				[]firewallapi.Set{ipSet("dup", firewallapi.SetDataTypeIPAddr), ipSet("dup", firewallapi.SetDataTypeIPCIDR)},
				"duplicated"),
			Entry("empty set name",
				[]firewallapi.Set{ipSet("", firewallapi.SetDataTypeIPAddr)},
				"empty"),
			Entry("invalid ipv4_addr element",
				[]firewallapi.Set{ipSet("s", firewallapi.SetDataTypeIPAddr, "10.0.0.300")},
				"invalid"),
			Entry("CIDR element in ipv4_addr set",
				[]firewallapi.Set{ipSet("s", firewallapi.SetDataTypeIPAddr, "10.0.0.0/24")},
				"invalid"),
			Entry("IPv6 element in ipv4_addr set",
				[]firewallapi.Set{ipSet("s", firewallapi.SetDataTypeIPAddr, "fd00::1")},
				"invalid"),
			Entry("invalid ipv4_cidr element",
				[]firewallapi.Set{ipSet("s", firewallapi.SetDataTypeIPCIDR, "10.0.0.0/33")},
				"invalid"),
			Entry("plain IP in ipv4_cidr set",
				[]firewallapi.Set{ipSet("s", firewallapi.SetDataTypeIPCIDR, "10.0.0.1")},
				"invalid"),
			Entry("invalid integer element",
				[]firewallapi.Set{ipSet("s", firewallapi.SetDataTypeInteger, "abc")},
				"invalid"),
			Entry("element with data in a set without data type",
				[]firewallapi.Set{{Name: "s", KeyType: firewallapi.SetDataTypeIPAddr,
					Elements: []firewallapi.SetElement{{Key: "10.0.0.1", Data: ptr.To("10.0.0.2")}}}},
				"should not have data"),
			Entry("element without data in a map",
				[]firewallapi.Set{{Name: "s", KeyType: firewallapi.SetDataTypeIPAddr, DataType: ptr.To(firewallapi.SetDataTypeIPAddr),
					Elements: []firewallapi.SetElement{{Key: "10.0.0.1"}}}},
				"nil data"),
			Entry("invalid data in a map",
				[]firewallapi.Set{{Name: "s", KeyType: firewallapi.SetDataTypeIPAddr, DataType: ptr.To(firewallapi.SetDataTypeIPAddr),
					Elements: []firewallapi.SetElement{{Key: "10.0.0.1", Data: ptr.To("not-an-ip")}}}},
				"invalid"),
		)
	})

	Context("rules referencing named sets", func() {
		sets := func() []firewallapi.Set {
			return []firewallapi.Set{
				ipSet("addrs", firewallapi.SetDataTypeIPAddr, "10.0.0.1"),
				ipSet("cidrs", firewallapi.SetDataTypeIPCIDR, "10.0.0.0/24"),
				ipSet("ints", firewallapi.SetDataTypeInteger, "42"),
			}
		}

		DescribeTable("should allow filter rules matching IP sets",
			func(op firewallapi.MatchOperation, value string, pos firewallapi.MatchPosition) {
				resp := validate(forgeFirewallConfiguration(sets(), forgeFilterChain(forgeIPMatch(op, value, pos))))
				Expect(resp.Allowed).To(BeTrue(), resp.Result.Message)
			},
			Entry("ipv4_addr set as source", firewallapi.MatchOperationEq, "@addrs", firewallapi.MatchPositionSrc),
			Entry("ipv4_addr set as destination", firewallapi.MatchOperationEq, "@addrs", firewallapi.MatchPositionDst),
			Entry("ipv4_cidr set as source", firewallapi.MatchOperationEq, "@cidrs", firewallapi.MatchPositionSrc),
			Entry("ipv4_cidr set with neq", firewallapi.MatchOperationNeq, "@cidrs", firewallapi.MatchPositionDst),
		)

		It("should allow a filter rule matching multiple sets", func() {
			resp := validate(forgeFirewallConfiguration(sets(), forgeFilterChain(
				forgeIPMatch(firewallapi.MatchOperationEq, "@addrs", firewallapi.MatchPositionSrc),
				forgeIPMatch(firewallapi.MatchOperationNeq, "@cidrs", firewallapi.MatchPositionDst),
			)))
			Expect(resp.Allowed).To(BeTrue(), resp.Result.Message)
		})

		It("should allow a nat rule matching an IP set", func() {
			resp := validate(forgeFirewallConfiguration(sets(), forgeNatChain(
				forgeIPMatch(firewallapi.MatchOperationEq, "@cidrs", firewallapi.MatchPositionSrc),
			)))
			Expect(resp.Allowed).To(BeTrue(), resp.Result.Message)
		})

		DescribeTable("should deny rules referencing invalid sets",
			func(chainFn func(...firewallapi.Match) *firewallapi.Chain, value, errSubstring string) {
				resp := validate(forgeFirewallConfiguration(sets(),
					chainFn(forgeIPMatch(firewallapi.MatchOperationEq, value, firewallapi.MatchPositionSrc))))
				Expect(resp.Allowed).To(BeFalse())
				Expect(resp.Result.Message).To(ContainSubstring(errSubstring))
			},
			Entry("filter rule with missing set", forgeFilterChain, "@missing", "does not exist"),
			Entry("filter rule with integer set", forgeFilterChain, "@ints", "is not of type IP"),
			Entry("filter rule with empty set name", forgeFilterChain, "@", "invalid"),
			Entry("nat rule with missing set", forgeNatChain, "@missing", "does not exist"),
			Entry("nat rule with integer set", forgeNatChain, "@ints", "is not of type IP"),
		)

		It("should deny a rule referencing a set when no sets are defined", func() {
			resp := validate(forgeFirewallConfiguration(nil,
				forgeFilterChain(forgeIPMatch(firewallapi.MatchOperationEq, "@addrs", firewallapi.MatchPositionSrc))))
			Expect(resp.Allowed).To(BeFalse())
			Expect(resp.Result.Message).To(ContainSubstring("does not exist"))
		})

		It("should deny an update removing a set still referenced by a rule", func() {
			oldFwcfg := forgeFirewallConfiguration(sets(),
				forgeFilterChain(forgeIPMatch(firewallapi.MatchOperationEq, "@addrs", firewallapi.MatchPositionSrc)))
			newFwcfg := oldFwcfg.DeepCopy()
			newFwcfg.Spec.Table.Sets = newFwcfg.Spec.Table.Sets[1:]

			resp := validator.Handle(context.TODO(), generateAdmissionRequest(newFwcfg, oldFwcfg, admissionv1.Update))
			Expect(resp.Allowed).To(BeFalse())
			Expect(resp.Result.Message).To(ContainSubstring("does not exist"))
		})
	})
})
