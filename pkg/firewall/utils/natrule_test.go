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

//go:build linux

package utils

import (
	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	firewallv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
)

var _ = Describe("NatRuleWrapper", func() {
	var (
		table *nftables.Table
		chain *nftables.Chain
	)

	BeforeEach(func() {
		table = &nftables.Table{
			Name:   "nat",
			Family: nftables.TableFamilyIPv4,
		}
		chain = &nftables.Chain{
			Name:  "POSTROUTING",
			Table: table,
		}
	})

	It("Equal should return true for same rule (Masquerade)", func() {
		nr := &firewallv1beta1.NatRule{
			Name: ptr.To("test-nat-rule"),
			Match: []firewallv1beta1.Match{
				{
					Op: firewallv1beta1.MatchOperationEq,
					Proto: &firewallv1beta1.MatchProto{
						Value: firewallv1beta1.L4ProtoUDP,
					},
				},
			},
			NatType: firewallv1beta1.NatTypeMasquerade,
			To:      ptr.To(""), // Void IP type for Masquerade
		}
		wrapper := &NatRuleWrapper{NatRule: nr}

		expectedRule, err := forgeNatRule(nr, chain)
		Expect(err).NotTo(HaveOccurred())
		expectedRule.Table = table

		Expect(wrapper.Equal(expectedRule)).To(BeTrue())
	})

	It("Equal should return true for DNAT rule", func() {
		nr := &firewallv1beta1.NatRule{
			Name: ptr.To("dnat-rule"),
			Match: []firewallv1beta1.Match{
				{
					Op: firewallv1beta1.MatchOperationEq,
					Proto: &firewallv1beta1.MatchProto{
						Value: firewallv1beta1.L4ProtoTCP,
					},
				},
			},
			NatType: firewallv1beta1.NatTypeDestination,
			To:      ptr.To("10.0.0.1"),
		}
		wrapper := &NatRuleWrapper{NatRule: nr}

		expectedRule, err := forgeNatRule(nr, chain)
		Expect(err).NotTo(HaveOccurred())
		expectedRule.Table = table

		Expect(wrapper.Equal(expectedRule)).To(BeTrue())
	})

	It("Equal should return true for SNAT rule", func() {
		nr := &firewallv1beta1.NatRule{
			Name: ptr.To("snat-rule"),
			Match: []firewallv1beta1.Match{
				{
					Op: firewallv1beta1.MatchOperationEq,
					Proto: &firewallv1beta1.MatchProto{
						Value: firewallv1beta1.L4ProtoTCP,
					},
				},
			},
			NatType: firewallv1beta1.NatTypeSource,
			To:      ptr.To("10.0.0.2"),
		}
		wrapper := &NatRuleWrapper{NatRule: nr}

		expectedRule, err := forgeNatRule(nr, chain)
		Expect(err).NotTo(HaveOccurred())
		expectedRule.Table = table

		Expect(wrapper.Equal(expectedRule)).To(BeTrue())
	})

	It("Equal should return true for complex DNAT rule", func() {
		nr := &firewallv1beta1.NatRule{
			Name: ptr.To("complex-dnat-rule"),
			Match: []firewallv1beta1.Match{
				{
					Op: firewallv1beta1.MatchOperationEq,
					Proto: &firewallv1beta1.MatchProto{
						Value: firewallv1beta1.L4ProtoTCP,
					},
				},
				{
					Op: firewallv1beta1.MatchOperationEq,
					Port: &firewallv1beta1.MatchPort{
						Value:    "8080",
						Position: firewallv1beta1.MatchPositionDst,
					},
				},
				{
					Op: firewallv1beta1.MatchOperationEq,
					IP: &firewallv1beta1.MatchIP{
						Value:    "192.168.1.0/24",
						Position: firewallv1beta1.MatchPositionSrc,
					},
				},
			},
			NatType: firewallv1beta1.NatTypeDestination,
			To:      ptr.To("10.10.10.10"),
		}
		wrapper := &NatRuleWrapper{NatRule: nr}

		expectedRule, err := forgeNatRule(nr, chain)
		Expect(err).NotTo(HaveOccurred())
		expectedRule.Table = table

		Expect(wrapper.Equal(expectedRule)).To(BeTrue())
	})

	It("Equal should return true for complex SNAT rule", func() {
		nr := &firewallv1beta1.NatRule{
			Name: ptr.To("complex-snat-rule"),
			Match: []firewallv1beta1.Match{
				{
					Op: firewallv1beta1.MatchOperationEq,
					Proto: &firewallv1beta1.MatchProto{
						Value: firewallv1beta1.L4ProtoUDP,
					},
				},
				{
					Op: firewallv1beta1.MatchOperationEq,
					IP: &firewallv1beta1.MatchIP{
						Value:    "10.0.0.10-10.0.0.20",
						Position: firewallv1beta1.MatchPositionDst,
					},
				},
				{
					Op: firewallv1beta1.MatchOperationEq,
					Port: &firewallv1beta1.MatchPort{
						Value:    "3000-4000",
						Position: firewallv1beta1.MatchPositionSrc,
					},
				},
			},
			NatType: firewallv1beta1.NatTypeSource,
			To:      ptr.To("192.168.1.100"),
		}
		wrapper := &NatRuleWrapper{NatRule: nr}

		expectedRule, err := forgeNatRule(nr, chain)
		Expect(err).NotTo(HaveOccurred())
		expectedRule.Table = table

		Expect(wrapper.Equal(expectedRule)).To(BeTrue())
	})

	It("Equal should return false for different rule", func() {
		nr := &firewallv1beta1.NatRule{
			Name:    ptr.To("test-nat-rule"),
			NatType: firewallv1beta1.NatTypeMasquerade,
			To:      ptr.To(""),
		}
		wrapper := &NatRuleWrapper{NatRule: nr}

		rule, err := forgeNatRule(nr, chain)
		Expect(err).NotTo(HaveOccurred())
		rule.Table = table

		// Modify expressions
		rule.Exprs = append(rule.Exprs, &expr.Meta{Key: expr.MetaKeyIIFNAME, Register: 1})

		Expect(wrapper.Equal(rule)).To(BeFalse())
	})
})
