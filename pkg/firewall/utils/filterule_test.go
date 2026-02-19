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

var _ = Describe("FilterRuleWrapper", func() {
	var (
		table *nftables.Table
		chain *nftables.Chain
	)

	BeforeEach(func() {
		table = &nftables.Table{
			Name:   "filter",
			Family: nftables.TableFamilyIPv4,
		}
		chain = &nftables.Chain{
			Name:  "INPUT",
			Table: table,
		}
	})

	It("Equal should return true for same rule", func() {
		fr := &firewallv1beta1.FilterRule{
			Name: ptr.To("test-filter-rule"),
			Match: []firewallv1beta1.Match{
				{
					Op: firewallv1beta1.MatchOperationEq,
					Proto: &firewallv1beta1.MatchProto{
						Value: firewallv1beta1.L4ProtoTCP,
					},
				},
			},
			Action: firewallv1beta1.ActionAccept,
		}
		wrapper := &FilterRuleWrapper{FilterRule: fr}

		expectedRule, err := forgeFilterRule(fr, chain)
		Expect(err).NotTo(HaveOccurred())
		expectedRule.Table = table

		Expect(wrapper.Equal(expectedRule)).To(BeTrue())
	})

	It("Equal should return false for different rule", func() {
		fr := &firewallv1beta1.FilterRule{
			Name: ptr.To("test-filter-rule"),
			Match: []firewallv1beta1.Match{
				{
					Op: firewallv1beta1.MatchOperationEq,
					Proto: &firewallv1beta1.MatchProto{
						Value: firewallv1beta1.L4ProtoTCP,
					},
				},
			},
			Action: firewallv1beta1.ActionAccept,
		}
		wrapper := &FilterRuleWrapper{FilterRule: fr}

		rule, err := forgeFilterRule(fr, chain)
		Expect(err).NotTo(HaveOccurred())
		rule.Table = table

		// Modify the rule (e.g., add a random expression)
		rule.Exprs = append(rule.Exprs, &expr.Meta{Key: expr.MetaKeyIIFNAME, Register: 1})

		Expect(wrapper.Equal(rule)).To(BeFalse())
	})

	It("Equal should return true for complex match", func() {
		fr := &firewallv1beta1.FilterRule{
			Name: ptr.To("complex-rule"),
			Match: []firewallv1beta1.Match{
				{
					Op: firewallv1beta1.MatchOperationEq,
					IP: &firewallv1beta1.MatchIP{
						Value:    "192.168.1.0/24",
						Position: firewallv1beta1.MatchPositionSrc,
					},
				},
				{
					Op: firewallv1beta1.MatchOperationEq,
					Port: &firewallv1beta1.MatchPort{
						Value:    "8000-9000",
						Position: firewallv1beta1.MatchPositionDst,
					},
				},
				{
					Op: firewallv1beta1.MatchOperationEq,
					Dev: &firewallv1beta1.MatchDev{
						Value:    "eth0",
						Position: firewallv1beta1.MatchDevPositionIn,
					},
				},
			},
			Action: firewallv1beta1.ActionAccept,
		}
		wrapper := &FilterRuleWrapper{FilterRule: fr}

		expectedRule, err := forgeFilterRule(fr, chain)
		Expect(err).NotTo(HaveOccurred())
		expectedRule.Table = table

		Expect(wrapper.Equal(expectedRule)).To(BeTrue())
	})

	It("Equal should return true for complex match (different order)", func() {
		fr := &firewallv1beta1.FilterRule{
			Name: ptr.To("very-complex-rule"),
			Match: []firewallv1beta1.Match{
				{
					Op: firewallv1beta1.MatchOperationEq,
					Proto: &firewallv1beta1.MatchProto{
						Value: firewallv1beta1.L4ProtoUDP,
					},
				},
				{
					Op: firewallv1beta1.MatchOperationEq,
					Dev: &firewallv1beta1.MatchDev{
						Value:    "eth0",
						Position: firewallv1beta1.MatchDevPositionIn,
					},
				},
				{
					Op: firewallv1beta1.MatchOperationEq,
					IP: &firewallv1beta1.MatchIP{
						Value:    "192.168.1.10-192.168.1.20",
						Position: firewallv1beta1.MatchPositionSrc,
					},
				},
				{
					Op: firewallv1beta1.MatchOperationEq,
					IP: &firewallv1beta1.MatchIP{
						Value:    "10.0.0.0/24",
						Position: firewallv1beta1.MatchPositionDst,
					},
				},
				{
					Op: firewallv1beta1.MatchOperationEq,
					Port: &firewallv1beta1.MatchPort{
						Value:    "5000-6000",
						Position: firewallv1beta1.MatchPositionDst,
					},
				},
			},
			Action: firewallv1beta1.ActionAccept,
		}
		wrapper := &FilterRuleWrapper{FilterRule: fr}

		expectedRule, err := forgeFilterRule(fr, chain)
		Expect(err).NotTo(HaveOccurred())
		expectedRule.Table = table

		Expect(wrapper.Equal(expectedRule)).To(BeTrue())
	})

	It("Equal should return true for complex exclusion rule", func() {
		fr := &firewallv1beta1.FilterRule{
			Name: ptr.To("exclusion-rule"),
			Match: []firewallv1beta1.Match{
				{
					Op: firewallv1beta1.MatchOperationNeq,
					Proto: &firewallv1beta1.MatchProto{
						Value: firewallv1beta1.L4ProtoTCP,
					},
				},
				{
					Op: firewallv1beta1.MatchOperationNeq,
					Dev: &firewallv1beta1.MatchDev{
						Value:    "eth0",
						Position: firewallv1beta1.MatchDevPositionOut,
					},
				},
				{
					Op: firewallv1beta1.MatchOperationEq,
					Port: &firewallv1beta1.MatchPort{
						Value:    "8080",
						Position: firewallv1beta1.MatchPositionSrc,
					},
				},
			},
			Action: firewallv1beta1.ActionDrop,
		}
		wrapper := &FilterRuleWrapper{FilterRule: fr}

		expectedRule, err := forgeFilterRule(fr, chain)
		Expect(err).NotTo(HaveOccurred())
		expectedRule.Table = table

		Expect(wrapper.Equal(expectedRule)).To(BeTrue())
	})

	It("Equal should return true for ActionCtMark", func() {
		fr := &firewallv1beta1.FilterRule{
			Name:   ptr.To("ctmark-rule"),
			Match:  []firewallv1beta1.Match{},
			Action: firewallv1beta1.ActionCtMark,
			Value:  ptr.To("100"),
		}
		wrapper := &FilterRuleWrapper{FilterRule: fr}

		expectedRule, err := forgeFilterRule(fr, chain)
		Expect(err).NotTo(HaveOccurred())
		expectedRule.Table = table

		Expect(wrapper.Equal(expectedRule)).To(BeTrue())
	})

	It("Equal should return true for ActionTCPMssClamp", func() {
		fr := &firewallv1beta1.FilterRule{
			Name: ptr.To("mss-clamp-rule"),
			Match: []firewallv1beta1.Match{
				{
					Op: firewallv1beta1.MatchOperationEq,
					Proto: &firewallv1beta1.MatchProto{
						Value: firewallv1beta1.L4ProtoTCP,
					},
				},
			},
			Action: firewallv1beta1.ActionTCPMssClamp,
			Value:  ptr.To("1400"),
		}
		wrapper := &FilterRuleWrapper{FilterRule: fr}

		expectedRule, err := forgeFilterRule(fr, chain)
		Expect(err).NotTo(HaveOccurred())
		expectedRule.Table = table

		Expect(wrapper.Equal(expectedRule)).To(BeTrue())
	})
})
