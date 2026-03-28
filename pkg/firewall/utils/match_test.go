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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

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
	})

	Context("ifname function", func() {
		It("should convert interface name to 16-byte array", func() {
			result := ifname("eth0")
			Expect(result).To(HaveLen(16))
			Expect(result[0:5]).To(Equal([]byte("eth0\x00")))
		})

		It("should handle long interface names", func() {
			result := ifname("verylonginterfacename")
			Expect(result).To(HaveLen(16))
		})
	})
})
