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
			Expect(err.Error()).To(ContainSubstring("value is required for setmetamark action"))
		})

		It("should error on invalid SetMetaMark value", func() {
			fr := &firewallv1beta1.FilterRule{
				Name:   ptr.To("invalid-setmetamark"),
				Action: firewallv1beta1.ActionSetMetaMark,
				Value:  ptr.To("not-a-number"),
			}
			_, err := forgeFilterRule(fr, chain)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot apply setmetamark action"))
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
})
