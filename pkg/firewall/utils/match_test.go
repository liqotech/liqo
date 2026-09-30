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
	"golang.org/x/sys/unix"

	firewallv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
)

var _ = Describe("Match Functions", func() {
	var (
		table *nftables.Table
		chain *nftables.Chain
		rule  *nftables.Rule
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
		rule = &nftables.Rule{
			Table: table,
			Chain: chain,
		}
	})

	Context("applyMatch", func() {
		It("should apply single IP match (src)", func() {
			match := &firewallv1beta1.Match{
				Op: firewallv1beta1.MatchOperationEq,
				IP: &firewallv1beta1.MatchIP{
					Value:    "192.168.1.1",
					Position: firewallv1beta1.MatchPositionSrc,
				},
			}
			err := applyMatch(match, rule)
			Expect(err).NotTo(HaveOccurred())
			Expect(rule.Exprs).NotTo(BeEmpty())
		})

		It("should apply single IP match (dst)", func() {
			match := &firewallv1beta1.Match{
				Op: firewallv1beta1.MatchOperationEq,
				IP: &firewallv1beta1.MatchIP{
					Value:    "10.0.0.1",
					Position: firewallv1beta1.MatchPositionDst,
				},
			}
			err := applyMatch(match, rule)
			Expect(err).NotTo(HaveOccurred())
			Expect(rule.Exprs).NotTo(BeEmpty())
		})

		It("should apply single IP match with Neq operation", func() {
			match := &firewallv1beta1.Match{
				Op: firewallv1beta1.MatchOperationNeq,
				IP: &firewallv1beta1.MatchIP{
					Value:    "192.168.1.1",
					Position: firewallv1beta1.MatchPositionSrc,
				},
			}
			err := applyMatch(match, rule)
			Expect(err).NotTo(HaveOccurred())
			Expect(rule.Exprs).NotTo(BeEmpty())
		})

		It("should apply IP subnet match", func() {
			match := &firewallv1beta1.Match{
				Op: firewallv1beta1.MatchOperationEq,
				IP: &firewallv1beta1.MatchIP{
					Value:    "192.168.0.0/24",
					Position: firewallv1beta1.MatchPositionSrc,
				},
			}
			err := applyMatch(match, rule)
			Expect(err).NotTo(HaveOccurred())
			Expect(rule.Exprs).NotTo(BeEmpty())
		})

		It("should apply IP subnet match with Neq operation", func() {
			match := &firewallv1beta1.Match{
				Op: firewallv1beta1.MatchOperationNeq,
				IP: &firewallv1beta1.MatchIP{
					Value:    "10.0.0.0/8",
					Position: firewallv1beta1.MatchPositionDst,
				},
			}
			err := applyMatch(match, rule)
			Expect(err).NotTo(HaveOccurred())
			Expect(rule.Exprs).NotTo(BeEmpty())
		})

		It("should apply IP range match", func() {
			match := &firewallv1beta1.Match{
				Op: firewallv1beta1.MatchOperationEq,
				IP: &firewallv1beta1.MatchIP{
					Value:    "192.168.1.1-192.168.1.100",
					Position: firewallv1beta1.MatchPositionSrc,
				},
			}
			err := applyMatch(match, rule)
			Expect(err).NotTo(HaveOccurred())
			Expect(rule.Exprs).NotTo(BeEmpty())
		})

		It("should apply IP range match with Neq operation", func() {
			match := &firewallv1beta1.Match{
				Op: firewallv1beta1.MatchOperationNeq,
				IP: &firewallv1beta1.MatchIP{
					Value:    "10.0.0.1-10.0.0.255",
					Position: firewallv1beta1.MatchPositionDst,
				},
			}
			err := applyMatch(match, rule)
			Expect(err).NotTo(HaveOccurred())
			Expect(rule.Exprs).NotTo(BeEmpty())
		})

		It("should apply single port match (src)", func() {
			match := &firewallv1beta1.Match{
				Op: firewallv1beta1.MatchOperationEq,
				Port: &firewallv1beta1.MatchPort{
					Value:    "8080",
					Position: firewallv1beta1.MatchPositionSrc,
				},
			}
			err := applyMatch(match, rule)
			Expect(err).NotTo(HaveOccurred())
			Expect(rule.Exprs).NotTo(BeEmpty())
		})

		It("should apply single port match (dst)", func() {
			match := &firewallv1beta1.Match{
				Op: firewallv1beta1.MatchOperationEq,
				Port: &firewallv1beta1.MatchPort{
					Value:    "443",
					Position: firewallv1beta1.MatchPositionDst,
				},
			}
			err := applyMatch(match, rule)
			Expect(err).NotTo(HaveOccurred())
			Expect(rule.Exprs).NotTo(BeEmpty())
		})

		It("should apply single port match with Neq operation", func() {
			match := &firewallv1beta1.Match{
				Op: firewallv1beta1.MatchOperationNeq,
				Port: &firewallv1beta1.MatchPort{
					Value:    "22",
					Position: firewallv1beta1.MatchPositionDst,
				},
			}
			err := applyMatch(match, rule)
			Expect(err).NotTo(HaveOccurred())
			Expect(rule.Exprs).NotTo(BeEmpty())
		})

		It("should apply port range match", func() {
			match := &firewallv1beta1.Match{
				Op: firewallv1beta1.MatchOperationEq,
				Port: &firewallv1beta1.MatchPort{
					Value:    "8000-9000",
					Position: firewallv1beta1.MatchPositionDst,
				},
			}
			err := applyMatch(match, rule)
			Expect(err).NotTo(HaveOccurred())
			Expect(rule.Exprs).NotTo(BeEmpty())
		})

		It("should apply proto match TCP", func() {
			match := &firewallv1beta1.Match{
				Op: firewallv1beta1.MatchOperationEq,
				Proto: &firewallv1beta1.MatchProto{
					Value: firewallv1beta1.L4ProtoTCP,
				},
			}
			err := applyMatch(match, rule)
			Expect(err).NotTo(HaveOccurred())
			Expect(rule.Exprs).NotTo(BeEmpty())
		})

		It("should apply proto match UDP", func() {
			match := &firewallv1beta1.Match{
				Op: firewallv1beta1.MatchOperationEq,
				Proto: &firewallv1beta1.MatchProto{
					Value: firewallv1beta1.L4ProtoUDP,
				},
			}
			err := applyMatch(match, rule)
			Expect(err).NotTo(HaveOccurred())
			Expect(rule.Exprs).NotTo(BeEmpty())
		})

		It("should apply dev match (in)", func() {
			match := &firewallv1beta1.Match{
				Op: firewallv1beta1.MatchOperationEq,
				Dev: &firewallv1beta1.MatchDev{
					Value:    "eth0",
					Position: firewallv1beta1.MatchDevPositionIn,
				},
			}
			err := applyMatch(match, rule)
			Expect(err).NotTo(HaveOccurred())
			Expect(rule.Exprs).NotTo(BeEmpty())
		})

		It("should apply dev match (out)", func() {
			match := &firewallv1beta1.Match{
				Op: firewallv1beta1.MatchOperationEq,
				Dev: &firewallv1beta1.MatchDev{
					Value:    "eth1",
					Position: firewallv1beta1.MatchDevPositionOut,
				},
			}
			err := applyMatch(match, rule)
			Expect(err).NotTo(HaveOccurred())
			Expect(rule.Exprs).NotTo(BeEmpty())
		})

		It("should apply dev match with Neq operation", func() {
			match := &firewallv1beta1.Match{
				Op: firewallv1beta1.MatchOperationNeq,
				Dev: &firewallv1beta1.MatchDev{
					Value:    "lo",
					Position: firewallv1beta1.MatchDevPositionIn,
				},
			}
			err := applyMatch(match, rule)
			Expect(err).NotTo(HaveOccurred())
			Expect(rule.Exprs).NotTo(BeEmpty())
		})

		DescribeTable("should apply set match",
			func(op firewallv1beta1.MatchOperation, pos firewallv1beta1.MatchDevPosition, values []string,
				expectedKey expr.MetaKey, expectedSetName string, expectedInvert bool) {
				match := &firewallv1beta1.Match{
					Op: op,
					Set: &firewallv1beta1.MatchSet{
						Values:   values,
						Position: pos,
					},
				}
				err := applyMatch(match, rule)
				Expect(err).NotTo(HaveOccurred())
				Expect(rule.Exprs).To(HaveLen(2))
				meta, ok := rule.Exprs[0].(*expr.Meta)
				Expect(ok).To(BeTrue())
				Expect(meta.Key).To(Equal(expectedKey))
				Expect(meta.Register).To(Equal(uint32(1)))
				lookup, ok := rule.Exprs[1].(*expr.Lookup)
				Expect(ok).To(BeTrue())
				Expect(lookup.SourceRegister).To(Equal(uint32(1)))
				Expect(lookup.SetName).To(Equal(expectedSetName))
				Expect(lookup.Invert).To(Equal(expectedInvert))
				Expect(lookup.IsDestRegSet).To(BeFalse())
				Expect(lookup.DestRegister).To(Equal(uint32(0)))
			},
			Entry("In x In", firewallv1beta1.MatchOperationIn, firewallv1beta1.MatchDevPositionIn,
				[]string{"eth0", "eth1", "eth2"}, expr.MetaKeyIIFNAME, "tunnel-list-3", false),
			Entry("In x Out", firewallv1beta1.MatchOperationIn, firewallv1beta1.MatchDevPositionOut,
				[]string{"eth1", "eth2", "eth3", "eth4"}, expr.MetaKeyOIFNAME, "tunnel-list-4", false),
			Entry("Nin x In", firewallv1beta1.MatchOperationNin, firewallv1beta1.MatchDevPositionIn,
				[]string{"eth0", "eth1"}, expr.MetaKeyIIFNAME, "tunnel-list-2", true),
			Entry("Nin x Out", firewallv1beta1.MatchOperationNin, firewallv1beta1.MatchDevPositionOut,
				[]string{"eth1", "eth2", "eth3", "eth4", "eth5"}, expr.MetaKeyOIFNAME, "tunnel-list-5", true),
			// Test with 1-interface set
			Entry("Nin x Out (1 value)", firewallv1beta1.MatchOperationNin, firewallv1beta1.MatchDevPositionOut,
				[]string{"eth1"}, expr.MetaKeyOIFNAME, "tunnel-list-1", true),
		)

		It("should apply dev match with wildcard prefix", func() {
			match := &firewallv1beta1.Match{
				Op: firewallv1beta1.MatchOperationEq,
				Dev: &firewallv1beta1.MatchDev{
					Value:    "liqo.",
					Position: firewallv1beta1.MatchDevPositionIn,
					Wildcard: true,
				},
			}
			err := applyMatch(match, rule)
			Expect(err).NotTo(HaveOccurred())
			Expect(rule.Exprs).To(HaveLen(2))

			meta, ok := rule.Exprs[0].(*expr.Meta)
			Expect(ok).To(BeTrue())
			Expect(meta.Key).To(Equal(expr.MetaKeyIIFNAME))

			cmp, ok := rule.Exprs[1].(*expr.Cmp)
			Expect(ok).To(BeTrue())
			// The wildcard match compares only the prefix bytes, without null-padding to 16 bytes.
			Expect(cmp.Data).To(Equal([]byte("liqo.")))
		})

		It("should null-pad dev match without wildcard", func() {
			match := &firewallv1beta1.Match{
				Op: firewallv1beta1.MatchOperationEq,
				Dev: &firewallv1beta1.MatchDev{
					Value:    "liqo.",
					Position: firewallv1beta1.MatchDevPositionIn,
				},
			}
			err := applyMatch(match, rule)
			Expect(err).NotTo(HaveOccurred())
			Expect(rule.Exprs).To(HaveLen(2))

			cmp, ok := rule.Exprs[1].(*expr.Cmp)
			Expect(ok).To(BeTrue())
			Expect(cmp.Data).To(HaveLen(16))
			Expect(cmp.Data).To(Equal(Ifname("liqo.")))
		})

		DescribeTable("should apply mark match",
			func(op firewallv1beta1.MatchOperation, value string, expectedOp expr.CmpOp, expectedMark uint32) {
				match := &firewallv1beta1.Match{
					Op:   op,
					Mark: &firewallv1beta1.MatchMark{Value: value},
				}
				Expect(applyMatch(match, rule)).To(Succeed())
				Expect(rule.Exprs).To(HaveLen(2))

				meta, ok := rule.Exprs[0].(*expr.Meta)
				Expect(ok).To(BeTrue())
				Expect(meta.Key).To(Equal(expr.MetaKeyMARK))
				Expect(meta.Register).To(Equal(uint32(1)))

				cmp, ok := rule.Exprs[1].(*expr.Cmp)
				Expect(ok).To(BeTrue())
				Expect(cmp.Op).To(Equal(expectedOp))
				Expect(cmp.Register).To(Equal(uint32(1)))
				Expect(cmp.Data).To(Equal(binaryutil.NativeEndian.PutUint32(expectedMark)))
			},
			Entry("Eq GwExtMark (decimal)", firewallv1beta1.MatchOperationEq,
				"65280", expr.CmpOpEq, uint32(0xFF00)),
			Entry("Eq GwNodeMark (decimal)", firewallv1beta1.MatchOperationEq,
				"65024", expr.CmpOpEq, uint32(0xFE00)),
			Entry("Eq GwExtMark (hex)", firewallv1beta1.MatchOperationEq,
				"0xff00", expr.CmpOpEq, uint32(0xFF00)),
			Entry("Neq GwNodeMark (hex)", firewallv1beta1.MatchOperationNeq,
				"0xfe00", expr.CmpOpNeq, uint32(0xFE00)),
			Entry("Eq max uint32", firewallv1beta1.MatchOperationEq,
				"4294967295", expr.CmpOpEq, uint32(0xFFFFFFFF)),
		)

		It("should apply combined match (proto + IP + port + dev)", func() {
			matches := []firewallv1beta1.Match{
				{
					Op: firewallv1beta1.MatchOperationEq,
					Proto: &firewallv1beta1.MatchProto{
						Value: firewallv1beta1.L4ProtoTCP,
					},
				},
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
						Value:    "443",
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
			}
			for i := range matches {
				err := applyMatch(&matches[i], rule)
				Expect(err).NotTo(HaveOccurred())
			}
			Expect(rule.Exprs).NotTo(BeEmpty())
		})

		It("should emit proto before set when both are present", func() {
			match := &firewallv1beta1.Match{
				Op: firewallv1beta1.MatchOperationIn,
				Proto: &firewallv1beta1.MatchProto{
					Value: firewallv1beta1.L4ProtoTCP,
				},
				Set: &firewallv1beta1.MatchSet{
					Values:   []string{"eth0", "eth1"},
					Position: firewallv1beta1.MatchDevPositionIn,
				},
			}
			err := applyMatch(match, rule)
			Expect(err).NotTo(HaveOccurred())
			Expect(rule.Exprs).To(HaveLen(4))

			// Proto part
			protoMeta, ok := rule.Exprs[0].(*expr.Meta)
			Expect(ok).To(BeTrue())
			Expect(protoMeta.Key).To(Equal(expr.MetaKeyL4PROTO))

			protoCmp, ok := rule.Exprs[1].(*expr.Cmp)
			Expect(ok).To(BeTrue())
			Expect(protoCmp.Op).To(Equal(expr.CmpOpEq))
			Expect(protoCmp.Data).To(Equal([]byte{unix.IPPROTO_TCP}))

			// Set part
			setMeta, ok := rule.Exprs[2].(*expr.Meta)
			Expect(ok).To(BeTrue())
			Expect(setMeta.Key).To(Equal(expr.MetaKeyIIFNAME))

			lookup, ok := rule.Exprs[3].(*expr.Lookup)
			Expect(ok).To(BeTrue())
			Expect(lookup.SetName).To(Equal("tunnel-list-2"))
			Expect(lookup.Invert).To(BeFalse())
		})

		DescribeTable("should ignore other fields when set is present",
			func(other func(m *firewallv1beta1.Match)) {
				match := &firewallv1beta1.Match{
					Op: firewallv1beta1.MatchOperationIn,
					Set: &firewallv1beta1.MatchSet{
						Values:   []string{"eth0", "eth1"},
						Position: firewallv1beta1.MatchDevPositionIn,
					},
				}
				other(match)

				Expect(applyMatch(match, rule)).To(Succeed())

				// Only Meta + Lookup from the Set, nothing from the other field.
				Expect(rule.Exprs).To(HaveLen(2))

				meta, ok := rule.Exprs[0].(*expr.Meta)
				Expect(ok).To(BeTrue())
				Expect(meta.Key).To(Equal(expr.MetaKeyIIFNAME))

				lookup, ok := rule.Exprs[1].(*expr.Lookup)
				Expect(ok).To(BeTrue())
				Expect(lookup.SetName).To(Equal("tunnel-list-2"))
			},
			Entry("Dev", func(m *firewallv1beta1.Match) {
				m.Dev = &firewallv1beta1.MatchDev{
					Value: "eth9", Position: firewallv1beta1.MatchDevPositionOut,
				}
			}),
			Entry("IP", func(m *firewallv1beta1.Match) {
				m.IP = &firewallv1beta1.MatchIP{
					Value: "10.0.0.1", Position: firewallv1beta1.MatchPositionSrc,
				}
			}),
			Entry("Port", func(m *firewallv1beta1.Match) {
				m.Port = &firewallv1beta1.MatchPort{
					Value: "443", Position: firewallv1beta1.MatchPositionDst,
				}
			}),
			Entry("Mark", func(m *firewallv1beta1.Match) {
				m.Mark = &firewallv1beta1.MatchMark{Value: "0xfe00"}
			}),
		)

		Context("TunnelListSetName", func() {
			It("should build the set name from the number of interfaces", func() {
				Expect(TunnelListSetName(1)).To(Equal("tunnel-list-1"))
				Expect(TunnelListSetName(3)).To(Equal("tunnel-list-3"))
			})
		})

	})

	Context("Error cases", func() {
		It("should error on invalid match operation", func() {
			match := &firewallv1beta1.Match{
				Op: "invalid-op",
				IP: &firewallv1beta1.MatchIP{
					Value:    "192.168.1.1",
					Position: firewallv1beta1.MatchPositionSrc,
				},
			}
			err := applyMatch(match, rule)
			Expect(err).To(HaveOccurred())
		})

		It("should error on invalid IP value", func() {
			match := &firewallv1beta1.Match{
				Op: firewallv1beta1.MatchOperationEq,
				IP: &firewallv1beta1.MatchIP{
					Value:    "invalid-ip",
					Position: firewallv1beta1.MatchPositionSrc,
				},
			}
			err := applyMatch(match, rule)
			Expect(err).To(HaveOccurred())
		})

		It("should error on invalid port value", func() {
			match := &firewallv1beta1.Match{
				Op: firewallv1beta1.MatchOperationEq,
				Port: &firewallv1beta1.MatchPort{
					Value:    "invalid-port",
					Position: firewallv1beta1.MatchPositionDst,
				},
			}
			err := applyMatch(match, rule)
			Expect(err).To(HaveOccurred())
		})

		DescribeTable("should error on set match with an invalid operation",
			func(op firewallv1beta1.MatchOperation) {
				match := &firewallv1beta1.Match{
					Op: op,
					Set: &firewallv1beta1.MatchSet{
						Values:   []string{"eth0", "eth1"},
						Position: firewallv1beta1.MatchDevPositionIn,
					},
				}
				err := applyMatch(match, rule)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid match operation"))
				Expect(rule.Exprs).To(BeEmpty())
			},
			Entry("Eq", firewallv1beta1.MatchOperationEq),
			Entry("Neq", firewallv1beta1.MatchOperationNeq),
			Entry("empty op", firewallv1beta1.MatchOperation("")),
			Entry("uppercase IN", firewallv1beta1.MatchOperation("IN")),
			Entry("random string", firewallv1beta1.MatchOperation("invalid-op")),
		)

		It("should error on set match with an invalid position", func() {
			match := &firewallv1beta1.Match{
				Op: firewallv1beta1.MatchOperationIn,
				Set: &firewallv1beta1.MatchSet{
					Values:   []string{"eth0", "eth1"},
					Position: firewallv1beta1.MatchDevPosition("random"),
				},
			}
			err := applyMatch(match, rule)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid match set position"))
			Expect(rule.Exprs).To(BeEmpty())
		})

		It("should error when the set is not defined", func() {
			match := &firewallv1beta1.Match{
				Op: firewallv1beta1.MatchOperationIn,
			}
			_, err := getMatchSetMetaKey(match)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("match set is not defined"))
		})

		DescribeTable("should error on mark match with an invalid value",
			func(value string) {
				match := &firewallv1beta1.Match{
					Op:   firewallv1beta1.MatchOperationEq,
					Mark: &firewallv1beta1.MatchMark{Value: value},
				}
				err := applyMatch(match, rule)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid mark value"))
				Expect(rule.Exprs).To(BeEmpty())
			},
			Entry("not a number", "abc"),
			Entry("empty", ""),
			Entry("negative", "-1"),
			Entry("hex over 32 bit", "0x100000000"),
			Entry("decimal over 32 bit", "4294967296"),
		)

		DescribeTable("should error on mark match with an invalid operation",
			func(op firewallv1beta1.MatchOperation) {
				match := &firewallv1beta1.Match{
					Op:   op,
					Mark: &firewallv1beta1.MatchMark{Value: "65280"},
				}
				err := applyMatch(match, rule)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid match operation"))
				Expect(rule.Exprs).To(BeEmpty())
			},
			Entry("In", firewallv1beta1.MatchOperationIn),
			Entry("Nin", firewallv1beta1.MatchOperationNin),
			Entry("empty op", firewallv1beta1.MatchOperation("")),
			Entry("random string", firewallv1beta1.MatchOperation("invalid-op")),
		)

		DescribeTable("should error on in/nin operation with eq/neq-only matches",
			func(op firewallv1beta1.MatchOperation, fill func(m *firewallv1beta1.Match)) {
				match := &firewallv1beta1.Match{Op: op}
				fill(match)

				err := applyMatch(match, rule)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid match operation"))
				Expect(rule.Exprs).To(BeEmpty())
			},
			Entry("Dev In", firewallv1beta1.MatchOperationIn, func(m *firewallv1beta1.Match) {
				m.Dev = &firewallv1beta1.MatchDev{Value: "eth0",
					Position: firewallv1beta1.MatchDevPositionIn}
			}),
			Entry("Dev Nin", firewallv1beta1.MatchOperationNin, func(m *firewallv1beta1.Match) {
				m.Dev = &firewallv1beta1.MatchDev{Value: "eth0",
					Position: firewallv1beta1.MatchDevPositionIn}
			}),
			Entry("IP In", firewallv1beta1.MatchOperationIn, func(m *firewallv1beta1.Match) {
				m.IP = &firewallv1beta1.MatchIP{Value: "10.0.0.1",
					Position: firewallv1beta1.MatchPositionSrc}
			}),
			Entry("IP Nin", firewallv1beta1.MatchOperationNin, func(m *firewallv1beta1.Match) {
				m.IP = &firewallv1beta1.MatchIP{Value: "10.0.0.1",
					Position: firewallv1beta1.MatchPositionSrc}
			}),
			Entry("Port In", firewallv1beta1.MatchOperationIn, func(m *firewallv1beta1.Match) {
				m.Port = &firewallv1beta1.MatchPort{Value: "443",
					Position: firewallv1beta1.MatchPositionDst}
			}),
			Entry("Port Nin", firewallv1beta1.MatchOperationNin, func(m *firewallv1beta1.Match) {
				m.Port = &firewallv1beta1.MatchPort{Value: "443",
					Position: firewallv1beta1.MatchPositionDst}
			}),
		)

		DescribeTable("should error on invalid position or value without panicking",
			func(fill func(m *firewallv1beta1.Match), expectedMsg string) {
				match := &firewallv1beta1.Match{Op: firewallv1beta1.MatchOperationEq}
				fill(match)

				var err error
				Expect(func() { err = applyMatch(match, rule) }).NotTo(Panic())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(expectedMsg))
				Expect(rule.Exprs).To(BeEmpty())
			},
			Entry("IP position", func(m *firewallv1beta1.Match) {
				m.IP = &firewallv1beta1.MatchIP{Value: "10.0.0.1", Position: "random"}
			}, "invalid match IP position"),
			Entry("Port position", func(m *firewallv1beta1.Match) {
				m.Port = &firewallv1beta1.MatchPort{Value: "443", Position: "random"}
			}, "invalid match port position"),
			Entry("Proto value", func(m *firewallv1beta1.Match) {
				m.Proto = &firewallv1beta1.MatchProto{Value: "icmp"}
			}, "invalid match proto value"),
			Entry("Dev position", func(m *firewallv1beta1.Match) {
				m.Dev = &firewallv1beta1.MatchDev{Value: "eth0", Position: "random"}
			}, "invalid match dev position"),
		)

		DescribeTable("should error on set match with no values",
			func(values []string) {
				match := &firewallv1beta1.Match{
					Op: firewallv1beta1.MatchOperationIn,
					Set: &firewallv1beta1.MatchSet{
						Values:   values,
						Position: firewallv1beta1.MatchDevPositionIn,
					},
				}
				err := applyMatch(match, rule)
				Expect(err).To(HaveOccurred())
				Expect(rule.Exprs).To(BeEmpty())
			},
			Entry("nil values", nil),
			Entry("empty values", []string{}),
		)

	})

	Context("ifname function", func() {
		It("should convert interface name to 16-byte array", func() {
			result := Ifname("eth0")
			Expect(result).To(HaveLen(16))
			Expect(result[0:5]).To(Equal([]byte("eth0\x00")))
		})

		It("should handle long interface names", func() {
			result := Ifname("verylonginterfacename")
			Expect(result).To(HaveLen(16))
		})
	})
})
