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
	"net"

	"github.com/google/nftables"
	"github.com/google/nftables/binaryutil"
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

	It("GetName should return the rule name", func() {
		fr := &firewallv1beta1.FilterRule{
			Name: ptr.To("test-rule-name"),
		}
		wrapper := &FilterRuleWrapper{FilterRule: fr}
		Expect(wrapper.GetName()).To(Equal(ptr.To("test-rule-name")))
	})

	It("SetName should set the rule name", func() {
		fr := &firewallv1beta1.FilterRule{
			Name: ptr.To("old-name"),
		}
		wrapper := &FilterRuleWrapper{FilterRule: fr}
		wrapper.SetName("new-name")
		Expect(wrapper.GetName()).To(Equal(ptr.To("new-name")))
	})

	It("Add should add rule to chain", func() {
		fr := &firewallv1beta1.FilterRule{
			Name:   ptr.To("test-add-rule"),
			Action: firewallv1beta1.ActionAccept,
		}
		wrapper := &FilterRuleWrapper{FilterRule: fr}

		conn, err := nftables.New(nftables.AsLasting())
		Expect(err).NotTo(HaveOccurred())

		err = wrapper.Add(conn, chain)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Equal should return true for ActionReject", func() {
		fr := &firewallv1beta1.FilterRule{
			Name:   ptr.To("reject-rule"),
			Action: firewallv1beta1.ActionReject,
		}
		wrapper := &FilterRuleWrapper{FilterRule: fr}

		expectedRule, err := forgeFilterRule(fr, chain)
		Expect(err).NotTo(HaveOccurred())
		expectedRule.Table = table

		Expect(wrapper.Equal(expectedRule)).To(BeTrue())
	})

	It("Equal should return true for rule with Counter", func() {
		fr := &firewallv1beta1.FilterRule{
			Name:    ptr.To("counter-rule"),
			Action:  firewallv1beta1.ActionAccept,
			Counter: true,
		}
		wrapper := &FilterRuleWrapper{FilterRule: fr}

		expectedRule, err := forgeFilterRule(fr, chain)
		Expect(err).NotTo(HaveOccurred())
		expectedRule.Table = table

		Expect(wrapper.Equal(expectedRule)).To(BeTrue())
	})

	It("Equal should return true for ActionSetMetaMarkFromCtMark", func() {
		fr := &firewallv1beta1.FilterRule{
			Name:   ptr.To("meta-mark-rule"),
			Action: firewallv1beta1.ActionSetMetaMarkFromCtMark,
		}
		wrapper := &FilterRuleWrapper{FilterRule: fr}

		expectedRule, err := forgeFilterRule(fr, chain)
		Expect(err).NotTo(HaveOccurred())
		expectedRule.Table = table

		Expect(wrapper.Equal(expectedRule)).To(BeTrue())
	})

	It("Equal should return true for complex rule with single IP match", func() {
		fr := &firewallv1beta1.FilterRule{
			Name: ptr.To("single-ip-rule"),
			Match: []firewallv1beta1.Match{
				{
					Op: firewallv1beta1.MatchOperationEq,
					IP: &firewallv1beta1.MatchIP{
						Value:    "192.168.1.1",
						Position: firewallv1beta1.MatchPositionSrc,
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

	It("Equal should return true for ActionNotrack", func() {
		fr := &firewallv1beta1.FilterRule{
			Name:   ptr.To("notrack-rule"),
			Action: firewallv1beta1.ActionNotrack,
		}
		wrapper := &FilterRuleWrapper{FilterRule: fr}

		expectedRule, err := forgeFilterRule(fr, chain)
		Expect(err).NotTo(HaveOccurred())
		expectedRule.Table = table

		Expect(wrapper.Equal(expectedRule)).To(BeTrue())
	})

	It("Equal should return true for ActionNotrack with UDP port match", func() {
		fr := &firewallv1beta1.FilterRule{
			Name: ptr.To("notrack-geneve-dport"),
			Match: []firewallv1beta1.Match{
				{
					Op: firewallv1beta1.MatchOperationEq,
					Proto: &firewallv1beta1.MatchProto{
						Value: firewallv1beta1.L4ProtoUDP,
					},
				},
				{
					Op: firewallv1beta1.MatchOperationEq,
					Port: &firewallv1beta1.MatchPort{
						Value:    "6081",
						Position: firewallv1beta1.MatchPositionDst,
					},
				},
			},
			Action:  firewallv1beta1.ActionNotrack,
			Counter: true,
		}
		wrapper := &FilterRuleWrapper{FilterRule: fr}

		expectedRule, err := forgeFilterRule(fr, chain)
		Expect(err).NotTo(HaveOccurred())
		expectedRule.Table = table

		Expect(wrapper.Equal(expectedRule)).To(BeTrue())
	})

	It("Equal should return true for ActionNotrack with Counter", func() {
		fr := &firewallv1beta1.FilterRule{
			Name:    ptr.To("notrack-counter-rule"),
			Action:  firewallv1beta1.ActionNotrack,
			Counter: true,
		}
		wrapper := &FilterRuleWrapper{FilterRule: fr}

		expectedRule, err := forgeFilterRule(fr, chain)
		Expect(err).NotTo(HaveOccurred())
		expectedRule.Table = table

		Expect(wrapper.Equal(expectedRule)).To(BeTrue())
	})

	Context("Error handling", func() {
		It("should handle invalid CtMark value", func() {
			fr := &firewallv1beta1.FilterRule{
				Name:   ptr.To("invalid-ctmark"),
				Action: firewallv1beta1.ActionCtMark,
				Value:  ptr.To("not-a-number"),
			}
			_, err := forgeFilterRule(fr, chain)
			Expect(err).To(HaveOccurred())
		})

		It("should handle invalid TCPMssClamp value", func() {
			fr := &firewallv1beta1.FilterRule{
				Name:   ptr.To("invalid-mss"),
				Action: firewallv1beta1.ActionTCPMssClamp,
				Value:  ptr.To("not-a-number"),
			}
			_, err := forgeFilterRule(fr, chain)
			Expect(err).To(HaveOccurred())
		})

		It("should handle invalid match in forgeFilterRule", func() {
			fr := &firewallv1beta1.FilterRule{
				Name: ptr.To("invalid-match"),
				Match: []firewallv1beta1.Match{
					{
						Op: "invalid-operation",
						IP: &firewallv1beta1.MatchIP{
							Value:    "192.168.1.1",
							Position: firewallv1beta1.MatchPositionSrc,
						},
					},
				},
				Action: firewallv1beta1.ActionAccept,
			}
			_, err := forgeFilterRule(fr, chain)
			Expect(err).To(HaveOccurred())
		})

		It("Equal should return false when forgeFilterRule fails", func() {
			fr := &firewallv1beta1.FilterRule{
				Name: ptr.To("invalid-rule"),
				Match: []firewallv1beta1.Match{
					{
						Op: "invalid-operation",
						IP: &firewallv1beta1.MatchIP{
							Value:    "192.168.1.1",
							Position: firewallv1beta1.MatchPositionSrc,
						},
					},
				},
				Action: firewallv1beta1.ActionAccept,
			}
			wrapper := &FilterRuleWrapper{FilterRule: fr}

			rule := &nftables.Rule{
				Table: table,
				Chain: chain,
			}

			Expect(wrapper.Equal(rule)).To(BeFalse())
		})

		It("should handle TCPMssClamp with nil value (auto-detect)", func() {
			fr := &firewallv1beta1.FilterRule{
				Name:   ptr.To("mss-auto"),
				Action: firewallv1beta1.ActionTCPMssClamp,
				Value:  nil,
			}
			_, err := forgeFilterRule(fr, chain)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should handle TCPMssClamp with zero value (auto-detect)", func() {
			fr := &firewallv1beta1.FilterRule{
				Name:   ptr.To("mss-zero"),
				Action: firewallv1beta1.ActionTCPMssClamp,
				Value:  ptr.To("0"),
			}
			_, err := forgeFilterRule(fr, chain)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should error on nil CtMark value", func() {
			fr := &firewallv1beta1.FilterRule{
				Name:   ptr.To("nil-ctmark"),
				Action: firewallv1beta1.ActionCtMark,
				Value:  nil,
			}
			_, err := forgeFilterRule(fr, chain)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("value is required for ctmark action"))
		})

		It("should error on nil SetMetaMark value", func() {
			fr := &firewallv1beta1.FilterRule{
				Name:   ptr.To("nil-setmetamark"),
				Action: firewallv1beta1.ActionSetMetaMark,
				Value:  nil,
			}
			_, err := forgeFilterRule(fr, chain)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).
				To(ContainSubstring("value is required for setmetamark action"))
		})

		It("should error on invalid SetMetaMark value", func() {
			fr := &firewallv1beta1.FilterRule{
				Name:   ptr.To("invalid-setmetamark"),
				Action: firewallv1beta1.ActionSetMetaMark,
				Value:  ptr.To("not-a-number"),
			}
			_, err := forgeFilterRule(fr, chain)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).
				To(ContainSubstring("cannot apply setmetamark action"))
		})

		It("should error on out-of-range CtMark value", func() {
			fr := &firewallv1beta1.FilterRule{
				Name:   ptr.To("overflow-ctmark"),
				Action: firewallv1beta1.ActionCtMark,
				Value:  ptr.To("4294967296"),
			}
			_, err := forgeFilterRule(fr, chain)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("out of range"))
		})

		It("should error on negative SetMetaMark value", func() {
			fr := &firewallv1beta1.FilterRule{
				Name:   ptr.To("negative-setmetamark"),
				Action: firewallv1beta1.ActionSetMetaMark,
				Value:  ptr.To("-1"),
			}
			_, err := forgeFilterRule(fr, chain)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("out of range"))
		})
	})

	Context("Mark actions", func() {
		It("should forge a valid setmetamark rule", func() {
			fr := &firewallv1beta1.FilterRule{
				Name:   ptr.To("setmetamark-rule"),
				Action: firewallv1beta1.ActionSetMetaMark,
				Value:  ptr.To("65280"),
			}
			rule, err := forgeFilterRule(fr, chain)
			Expect(err).NotTo(HaveOccurred())
			Expect(rule.Exprs).To(HaveLen(2))

			imm, ok := rule.Exprs[0].(*expr.Immediate)
			Expect(ok).To(BeTrue())
			Expect(imm.Register).To(Equal(uint32(1)))
			Expect(imm.Data).To(Equal(binaryutil.NativeEndian.PutUint32(65280)))

			meta, ok := rule.Exprs[1].(*expr.Meta)
			Expect(ok).To(BeTrue())
			Expect(meta.Key).To(Equal(expr.MetaKeyMARK))
			Expect(meta.SourceRegister).To(BeTrue())
			Expect(meta.Register).To(Equal(uint32(1)))
		})

		It("should forge a valid ctmark rule", func() {
			fr := &firewallv1beta1.FilterRule{
				Name:   ptr.To("ctmark-rule"),
				Action: firewallv1beta1.ActionCtMark,
				Value:  ptr.To("100"),
			}
			rule, err := forgeFilterRule(fr, chain)
			Expect(err).NotTo(HaveOccurred())
			Expect(rule.Exprs).To(HaveLen(2))

			imm, ok := rule.Exprs[0].(*expr.Immediate)
			Expect(ok).To(BeTrue())
			Expect(imm.Register).To(Equal(uint32(1)))
			Expect(imm.Data).To(Equal(binaryutil.NativeEndian.PutUint32(100)))

			ct, ok := rule.Exprs[1].(*expr.Ct)
			Expect(ok).To(BeTrue())
			Expect(ct.Key).To(Equal(expr.CtKeyMARK))
			Expect(ct.SourceRegister).To(BeTrue())
			Expect(ct.Register).To(Equal(uint32(1)))
		})

		It("should accept boundary mark values", func() {
			for _, v := range []string{"0", "4294967295"} {
				fr := &firewallv1beta1.FilterRule{
					Name:   ptr.To("boundary-mark"),
					Action: firewallv1beta1.ActionSetMetaMark,
					Value:  ptr.To(v),
				}
				_, err := forgeFilterRule(fr, chain)
				Expect(err).NotTo(HaveOccurred())
			}
		})
	})

	Context("Set match", func() {
		DescribeTable("should forge a rule with a set match",
			func(op firewallv1beta1.MatchOperation,
				pos firewallv1beta1.MatchDevPosition, values []string,
				expectedKey expr.MetaKey, expectedSetName string, expectedInvert bool) {
				fr := &firewallv1beta1.FilterRule{
					Name: ptr.To("set-rule"),
					Match: []firewallv1beta1.Match{
						{
							Op: op,
							Set: &firewallv1beta1.MatchSet{
								Values:   values,
								Position: pos,
							},
						},
					},
					Action: firewallv1beta1.ActionAccept,
				}
				rule, err := forgeFilterRule(fr, chain)
				Expect(err).NotTo(HaveOccurred())
				Expect(rule.Exprs).To(HaveLen(3))

				meta, ok := rule.Exprs[0].(*expr.Meta)
				Expect(ok).To(BeTrue())
				Expect(meta.Key).To(Equal(expectedKey))
				Expect(meta.Register).To(Equal(uint32(1)))

				lookup, ok := rule.Exprs[1].(*expr.Lookup)
				Expect(ok).To(BeTrue())
				Expect(lookup.SourceRegister).To(Equal(uint32(1)))
				Expect(lookup.SetName).To(Equal(expectedSetName))
				Expect(lookup.Invert).To(Equal(expectedInvert))

				verdict, ok := rule.Exprs[2].(*expr.Verdict)
				Expect(ok).To(BeTrue())
				Expect(verdict.Kind).To(Equal(expr.VerdictAccept))
			},
			Entry("In x In", firewallv1beta1.MatchOperationIn,
				firewallv1beta1.MatchDevPositionIn,
				[]string{"liqo-tunnel", "liqo-tunnel1"},
				expr.MetaKeyIIFNAME, "tunnel-list-2", false),
			Entry("In x Out", firewallv1beta1.MatchOperationIn,
				firewallv1beta1.MatchDevPositionOut,
				[]string{"liqo-tunnel", "liqo-tunnel1"},
				expr.MetaKeyOIFNAME, "tunnel-list-2", false),
			Entry("Nin x In", firewallv1beta1.MatchOperationNin,
				firewallv1beta1.MatchDevPositionIn,
				[]string{"liqo-tunnel", "liqo-tunnel1", "liqo-tunnel2"},
				expr.MetaKeyIIFNAME, "tunnel-list-3", true),
			Entry("Nin x Out", firewallv1beta1.MatchOperationNin,
				firewallv1beta1.MatchDevPositionOut,
				[]string{"liqo-tunnel", "liqo-tunnel1", "liqo-tunnel2"},
				expr.MetaKeyOIFNAME, "tunnel-list-3", true),
		)

		It("should forge the mss-clamping rule (proto + set + counter + clamp)", func() {
			fr := &firewallv1beta1.FilterRule{
				Name: ptr.To("mss-clamping-out"),
				Match: []firewallv1beta1.Match{
					{
						Op:    firewallv1beta1.MatchOperationEq,
						Proto: &firewallv1beta1.MatchProto{Value: firewallv1beta1.L4ProtoTCP},
					},
					{
						Op: firewallv1beta1.MatchOperationIn,
						Set: &firewallv1beta1.MatchSet{
							Values: []string{"liqo-tunnel", "liqo-tunnel1",
								"liqo-tunnel2", "liqo-tunnel3"},
							Position: firewallv1beta1.MatchDevPositionOut,
						},
					},
				},
				Counter: true,
				Action:  firewallv1beta1.ActionTCPMssClamp,
			}
			rule, err := forgeFilterRule(fr, chain)
			Expect(err).NotTo(HaveOccurred())

			// proto (2) + set (2) + counter (1) + clamp (>= 1)
			Expect(len(rule.Exprs)).To(BeNumerically(">", 5))

			protoMeta, ok := rule.Exprs[0].(*expr.Meta)
			Expect(ok).To(BeTrue())
			Expect(protoMeta.Key).To(Equal(expr.MetaKeyL4PROTO))
			_, ok = rule.Exprs[1].(*expr.Cmp)
			Expect(ok).To(BeTrue())

			setMeta, ok := rule.Exprs[2].(*expr.Meta)
			Expect(ok).To(BeTrue())
			Expect(setMeta.Key).To(Equal(expr.MetaKeyOIFNAME))
			lookup, ok := rule.Exprs[3].(*expr.Lookup)
			Expect(ok).To(BeTrue())
			Expect(lookup.SetName).To(Equal("tunnel-list-4"))
			Expect(lookup.Invert).To(BeFalse())

			_, ok = rule.Exprs[4].(*expr.Counter)
			Expect(ok).To(BeTrue())

			// The last expression writes the MSS into the TCP option.
			_, ok = rule.Exprs[len(rule.Exprs)-1].(*expr.Exthdr)
			Expect(ok).To(BeTrue())
		})

		DescribeTable("should propagate set match errors",
			func(op firewallv1beta1.MatchOperation, values []string) {
				fr := &firewallv1beta1.FilterRule{
					Name: ptr.To("set-invalid"),
					Match: []firewallv1beta1.Match{
						{
							Op: op,
							Set: &firewallv1beta1.MatchSet{
								Values:   values,
								Position: firewallv1beta1.MatchDevPositionOut,
							},
						},
					},
					Action: firewallv1beta1.ActionAccept,
				}
				rule, err := forgeFilterRule(fr, chain)
				Expect(err).To(HaveOccurred())
				Expect(rule).To(BeNil())
			},
			Entry("invalid op (eq)", firewallv1beta1.MatchOperationEq,
				[]string{"liqo-tunnel", "liqo-tunnel1"}),
			Entry("invalid op (neq)", firewallv1beta1.MatchOperationNeq,
				[]string{"liqo-tunnel", "liqo-tunnel1"}),
			Entry("nil values", firewallv1beta1.MatchOperationIn, nil),
			Entry("empty values", firewallv1beta1.MatchOperationIn, []string{}),
		)

		DescribeTable("Equal should distinguish set rules",
			func(opA firewallv1beta1.MatchOperation, valuesA []string,
				opB firewallv1beta1.MatchOperation, valuesB []string) {
				forge := func(op firewallv1beta1.MatchOperation,
					values []string) *firewallv1beta1.FilterRule {
					return &firewallv1beta1.FilterRule{
						Name: ptr.To("set-equal"),
						Match: []firewallv1beta1.Match{
							{
								Op: op,
								Set: &firewallv1beta1.MatchSet{
									Values:   values,
									Position: firewallv1beta1.MatchDevPositionOut,
								},
							},
						},
						Action: firewallv1beta1.ActionAccept,
					}
				}
				frA := forge(opA, valuesA)
				frB := forge(opB, valuesB)

				ruleA, err := forgeFilterRule(frA, chain)
				Expect(err).NotTo(HaveOccurred())
				ruleA.Table = table

				Expect((&FilterRuleWrapper{FilterRule: frA}).Equal(ruleA)).To(BeTrue())
				Expect((&FilterRuleWrapper{FilterRule: frB}).Equal(ruleA)).To(BeFalse())
			},
			Entry("in vs nin", firewallv1beta1.MatchOperationIn,
				[]string{"liqo-tunnel", "liqo-tunnel1"},
				firewallv1beta1.MatchOperationNin,
				[]string{"liqo-tunnel", "liqo-tunnel1"}),
			Entry("different length", firewallv1beta1.MatchOperationIn,
				[]string{"liqo-tunnel", "liqo-tunnel1"},
				firewallv1beta1.MatchOperationIn,
				[]string{"liqo-tunnel", "liqo-tunnel1", "liqo-tunnel2"}),
		)
	})

	Context("Mark match", func() {
		DescribeTable("should forge a rule with an IP match, a mark match and setmetamarkfromctmark",
			func(ip string, ipPos firewallv1beta1.MatchPosition, expectedOffset uint32,
				markValue string, expectedMark uint32) {
				fr := &firewallv1beta1.FilterRule{
					Name: ptr.To("mark-to-meta-mark"),
					Match: []firewallv1beta1.Match{
						{
							Op: firewallv1beta1.MatchOperationEq,
							IP: &firewallv1beta1.MatchIP{
								Value:    ip,
								Position: ipPos,
							},
						},
						{
							Op:   firewallv1beta1.MatchOperationEq,
							Mark: &firewallv1beta1.MatchMark{Value: markValue},
						},
					},
					Action: firewallv1beta1.ActionSetMetaMarkFromCtMark,
				}
				rule, err := forgeFilterRule(fr, chain)
				Expect(err).NotTo(HaveOccurred())
				Expect(rule.Exprs).To(HaveLen(6))

				// IP match
				payload, ok := rule.Exprs[0].(*expr.Payload)
				Expect(ok).To(BeTrue())
				Expect(payload.Offset).To(Equal(expectedOffset))
				ipCmp, ok := rule.Exprs[1].(*expr.Cmp)
				Expect(ok).To(BeTrue())
				Expect(net.IP(ipCmp.Data).Equal(net.ParseIP(ip))).To(BeTrue())

				// Mark match
				markMeta, ok := rule.Exprs[2].(*expr.Meta)
				Expect(ok).To(BeTrue())
				Expect(markMeta.Key).To(Equal(expr.MetaKeyMARK))
				Expect(markMeta.SourceRegister).To(BeFalse())
				markCmp, ok := rule.Exprs[3].(*expr.Cmp)
				Expect(ok).To(BeTrue())
				Expect(markCmp.Op).To(Equal(expr.CmpOpEq))
				Expect(markCmp.Data).To(Equal(binaryutil.NativeEndian.PutUint32(expectedMark)))

				// Action: meta mark set ct mark
				ct, ok := rule.Exprs[4].(*expr.Ct)
				Expect(ok).To(BeTrue())
				Expect(ct.Key).To(Equal(expr.CtKeyMARK))
				Expect(ct.SourceRegister).To(BeFalse())
				setMeta, ok := rule.Exprs[5].(*expr.Meta)
				Expect(ok).To(BeTrue())
				Expect(setMeta.Key).To(Equal(expr.MetaKeyMARK))
				Expect(setMeta.SourceRegister).To(BeTrue())
			},
			Entry("dst 10.71.0.0, mark 0xfe00 (decimal)",
				"10.71.0.0", firewallv1beta1.MatchPositionDst,
				uint32(16), "65024", uint32(0xFE00)),
			Entry("dst 10.72.0.0, mark 0xff00 (decimal)",
				"10.72.0.0", firewallv1beta1.MatchPositionDst,
				uint32(16), "65280", uint32(0xFF00)),
			Entry("src 10.71.0.0, mark 0xfe00 (hex)",
				"10.71.0.0", firewallv1beta1.MatchPositionSrc,
				uint32(12), "0xfe00", uint32(0xFE00)),
			Entry("real nodeport rule: dst 10.70.0.0, GwNodeMark (decimal)",
				"10.70.0.0", firewallv1beta1.MatchPositionDst,
				uint32(16), "65024", uint32(0xFE00)),
		)

		DescribeTable("should forge a rule with a mark match (neq)",
			func(markValue string, expectedMark uint32) {
				fr := &firewallv1beta1.FilterRule{
					Name: ptr.To("mark-neq"),
					Match: []firewallv1beta1.Match{
						{
							Op:   firewallv1beta1.MatchOperationNeq,
							Mark: &firewallv1beta1.MatchMark{Value: markValue},
						},
					},
					Action: firewallv1beta1.ActionAccept,
				}
				rule, err := forgeFilterRule(fr, chain)
				Expect(err).NotTo(HaveOccurred())
				Expect(rule.Exprs).To(HaveLen(3))

				meta, ok := rule.Exprs[0].(*expr.Meta)
				Expect(ok).To(BeTrue())
				Expect(meta.Key).To(Equal(expr.MetaKeyMARK))
				Expect(meta.Register).To(Equal(uint32(1)))
				Expect(meta.SourceRegister).To(BeFalse())

				cmp, ok := rule.Exprs[1].(*expr.Cmp)
				Expect(ok).To(BeTrue())
				Expect(cmp.Op).To(Equal(expr.CmpOpNeq))
				Expect(cmp.Register).To(Equal(uint32(1)))
				Expect(cmp.Data).To(Equal(binaryutil.NativeEndian.PutUint32(expectedMark)))

				verdict, ok := rule.Exprs[2].(*expr.Verdict)
				Expect(ok).To(BeTrue())
				Expect(verdict.Kind).To(Equal(expr.VerdictAccept))
			},
			Entry("GwExtMark (decimal)", "65280", uint32(0xFF00)),
			Entry("GwNodeMark (hex)", "0xfe00", uint32(0xFE00)),
		)

		DescribeTable("should propagate mark match errors",
			func(markValue string) {
				fr := &firewallv1beta1.FilterRule{
					Name: ptr.To("mark-invalid"),
					Match: []firewallv1beta1.Match{
						{
							Op: firewallv1beta1.MatchOperationEq,
							IP: &firewallv1beta1.MatchIP{
								Value:    "10.71.0.0",
								Position: firewallv1beta1.MatchPositionDst,
							},
						},
						{
							Op:   firewallv1beta1.MatchOperationEq,
							Mark: &firewallv1beta1.MatchMark{Value: markValue},
						},
					},
					Action: firewallv1beta1.ActionAccept,
				}
				rule, err := forgeFilterRule(fr, chain)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid mark value"))
				Expect(rule).To(BeNil())
			},
			Entry("not a number", "abc"),
			Entry("empty", ""),
			Entry("negative", "-1"),
			Entry("over 32 bit", "4294967296"),
		)

		DescribeTable("should forge the gw mark rules (wildcard dev + setmetamark)",
			func(prefix string, markValue string, expectedMark uint32) {
				fr := &firewallv1beta1.FilterRule{
					Name: ptr.To("gw-mark"),
					Match: []firewallv1beta1.Match{
						{
							Op: firewallv1beta1.MatchOperationEq,
							Dev: &firewallv1beta1.MatchDev{
								Value:    prefix,
								Position: firewallv1beta1.MatchDevPositionIn,
								Wildcard: true,
							},
						},
					},
					Action: firewallv1beta1.ActionSetMetaMark,
					Value:  ptr.To(markValue),
				}
				rule, err := forgeFilterRule(fr, chain)
				Expect(err).NotTo(HaveOccurred())
				Expect(rule.Exprs).To(HaveLen(4))

				// Dev wildcard match: bare prefix, no padding
				devMeta, ok := rule.Exprs[0].(*expr.Meta)
				Expect(ok).To(BeTrue())
				Expect(devMeta.Key).To(Equal(expr.MetaKeyIIFNAME))
				Expect(devMeta.SourceRegister).To(BeFalse())
				devCmp, ok := rule.Exprs[1].(*expr.Cmp)
				Expect(ok).To(BeTrue())
				Expect(devCmp.Op).To(Equal(expr.CmpOpEq))
				Expect(devCmp.Data).To(Equal([]byte(prefix)))

				// Action: meta mark set <value>
				imm, ok := rule.Exprs[2].(*expr.Immediate)
				Expect(ok).To(BeTrue())
				Expect(imm.Data).To(Equal(binaryutil.NativeEndian.PutUint32(expectedMark)))
				setMeta, ok := rule.Exprs[3].(*expr.Meta)
				Expect(ok).To(BeTrue())
				Expect(setMeta.Key).To(Equal(expr.MetaKeyMARK))
				Expect(setMeta.SourceRegister).To(BeTrue())
			},
			Entry("gw-ext-mark: liqo. -> 0xff00", "liqo.", "65280", uint32(0xFF00)),
			Entry("gw-node-mark: liqo-tunnel -> 0xfe00", "liqo-tunnel", "65024", uint32(0xFE00)),
		)

		DescribeTable("Equal should distinguish mark rules",
			func(opA firewallv1beta1.MatchOperation, valueA string,
				opB firewallv1beta1.MatchOperation, valueB string) {
				forge := func(op firewallv1beta1.MatchOperation, value string) *firewallv1beta1.FilterRule {
					return &firewallv1beta1.FilterRule{
						Name: ptr.To("mark-equal"),
						Match: []firewallv1beta1.Match{
							{
								Op:   op,
								Mark: &firewallv1beta1.MatchMark{Value: value},
							},
						},
						Action: firewallv1beta1.ActionAccept,
					}
				}
				frA := forge(opA, valueA)
				frB := forge(opB, valueB)

				ruleA, err := forgeFilterRule(frA, chain)
				Expect(err).NotTo(HaveOccurred())
				ruleA.Table = table

				Expect((&FilterRuleWrapper{FilterRule: frA}).Equal(ruleA)).To(BeTrue())
				Expect((&FilterRuleWrapper{FilterRule: frB}).Equal(ruleA)).To(BeFalse())
			},
			Entry("different value",
				firewallv1beta1.MatchOperationEq, "65280",
				firewallv1beta1.MatchOperationEq, "65024"),
			Entry("eq vs neq, same value",
				firewallv1beta1.MatchOperationEq, "65280",
				firewallv1beta1.MatchOperationNeq, "65280"),
		)
	})
})
