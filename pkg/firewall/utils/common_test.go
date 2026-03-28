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
	"testing"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	firewallv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
)

func TestUtils(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Utils Suite")
}

var _ = Describe("Common Utils", func() {
	Context("Value Type Parsing", func() {
		Describe("GetIPValueType", func() {
			It("should handle nil value", func() {
				valType, err := GetIPValueType(nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(valType).To(Equal(firewallv1beta1.IPValueTypeVoid))
			})

			It("should detect Subnet", func() {
				val := "192.168.1.0/24"
				valType, err := GetIPValueType(&val)
				Expect(err).NotTo(HaveOccurred())
				Expect(valType).To(Equal(firewallv1beta1.IPValueTypeSubnet))
			})

			It("should detect IP", func() {
				val := "192.168.1.1"
				valType, err := GetIPValueType(&val)
				Expect(err).NotTo(HaveOccurred())
				Expect(valType).To(Equal(firewallv1beta1.IPValueTypeIP))
			})

			It("should detect Range", func() {
				val := "192.168.1.1-192.168.1.10"
				valType, err := GetIPValueType(&val)
				Expect(err).NotTo(HaveOccurred())
				Expect(valType).To(Equal(firewallv1beta1.IPValueTypeRange))
			})

			It("should error on invalid format", func() {
				val := "invalid-ip"
				valType, err := GetIPValueType(&val)
				Expect(err).To(HaveOccurred())
				Expect(valType).To(Equal(firewallv1beta1.IPValueTypeVoid))
			})
		})

		Describe("GetIPValueRange", func() {
			It("should parse valid range", func() {
				start, end, err := GetIPValueRange("192.168.1.1-192.168.1.10")
				Expect(err).NotTo(HaveOccurred())
				Expect(start.String()).To(Equal("192.168.1.1"))
				Expect(end.String()).To(Equal("192.168.1.10"))
			})

			It("should error on invalid range format", func() {
				_, _, err := GetIPValueRange("192.168.1.1")
				Expect(err).To(HaveOccurred())
			})

			It("should error on invalid IPs in range", func() {
				_, _, err := GetIPValueRange("192.168.1.1-invalid")
				Expect(err).To(HaveOccurred())
			})
		})

		Describe("GetPortValueType", func() {
			It("should handle nil value", func() {
				valType, err := GetPortValueType(nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(valType).To(Equal(firewallv1beta1.PortValueTypeVoid))
			})

			It("should detect Port", func() {
				val := "8080"
				valType, err := GetPortValueType(&val)
				Expect(err).NotTo(HaveOccurred())
				Expect(valType).To(Equal(firewallv1beta1.PortValueTypePort))
			})

			It("should detect Range", func() {
				val := "8080-8090"
				valType, err := GetPortValueType(&val)
				Expect(err).NotTo(HaveOccurred())
				Expect(valType).To(Equal(firewallv1beta1.PortValueTypeRange))
			})

			It("should error on invalid format", func() {
				val := "invalid-port"
				valType, err := GetPortValueType(&val)
				Expect(err).To(HaveOccurred())
				Expect(valType).To(Equal(firewallv1beta1.PortValueTypeVoid))
			})
		})
	})

	Context("filterUnstableExprs", func() {
		It("should remove unstable expressions", func() {
			exprs := []expr.Any{
				&expr.Meta{Key: expr.MetaKeyIIFNAME, Register: 1},
				&expr.Counter{Bytes: 100, Packets: 10},
				&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte("eth0")},
				&expr.Rt{Register: 1},
				&expr.Byteorder{SourceRegister: 1, DestRegister: 2, Op: expr.ByteorderHton, Len: 2, Size: 2},
			}

			filtered := filterUnstableExprs(exprs)

			Expect(filtered).To(HaveLen(2))
			Expect(filtered[0]).To(BeAssignableToTypeOf(&expr.Meta{}))
			Expect(filtered[1]).To(BeAssignableToTypeOf(&expr.Cmp{}))
		})

		It("should keep stable expressions", func() {
			exprs := []expr.Any{
				&expr.Meta{Key: expr.MetaKeyIIFNAME, Register: 1},
				&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte("eth0")},
			}

			filtered := filterUnstableExprs(exprs)

			Expect(filtered).To(HaveLen(2))
			Expect(filtered[0]).To(Equal(exprs[0]))
			Expect(filtered[1]).To(Equal(exprs[1]))
		})
	})

	Context("compareRuleExpressions", func() {
		var (
			rule1, rule2 *nftables.Rule
			table        *nftables.Table
		)

		BeforeEach(func() {
			table = &nftables.Table{Family: nftables.TableFamilyIPv4}
			rule1 = &nftables.Rule{Table: table}
			rule2 = &nftables.Rule{Table: table}
		})

		It("should return true for identical rules", func() {
			rule1.Exprs = []expr.Any{
				&expr.Meta{Key: expr.MetaKeyIIFNAME, Register: 1},
				&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte("eth0")},
			}
			rule2.Exprs = []expr.Any{
				&expr.Meta{Key: expr.MetaKeyIIFNAME, Register: 1},
				&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte("eth0")},
			}

			Expect(compareRuleExpressions("test-rule", rule1, rule2)).To(BeTrue())
		})

		It("should return true when unstable expressions differ", func() {
			rule1.Exprs = []expr.Any{
				&expr.Meta{Key: expr.MetaKeyIIFNAME, Register: 1},
				&expr.Counter{Bytes: 100, Packets: 10},
				&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte("eth0")},
			}
			rule2.Exprs = []expr.Any{
				&expr.Meta{Key: expr.MetaKeyIIFNAME, Register: 1},
				&expr.Counter{Bytes: 200, Packets: 20}, // Different counter
				&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte("eth0")},
			}

			Expect(compareRuleExpressions("test-rule", rule1, rule2)).To(BeTrue())
		})

		It("should return false for different rules", func() {
			rule1.Exprs = []expr.Any{
				&expr.Meta{Key: expr.MetaKeyIIFNAME, Register: 1},
				&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte("eth0")},
			}
			rule2.Exprs = []expr.Any{
				&expr.Meta{Key: expr.MetaKeyIIFNAME, Register: 1},
				&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte("eth1")}, // Different interface
			}

			Expect(compareRuleExpressions("test-rule", rule1, rule2)).To(BeFalse())
		})

		It("should return false when expression count differs (after filtering)", func() {
			rule1.Exprs = []expr.Any{
				&expr.Meta{Key: expr.MetaKeyIIFNAME, Register: 1},
				&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte("eth0")},
			}
			rule2.Exprs = []expr.Any{
				&expr.Meta{Key: expr.MetaKeyIIFNAME, Register: 1},
			}

			Expect(compareRuleExpressions("test-rule", rule1, rule2)).To(BeFalse())
		})

		It("should return false when expression types differ", func() {
			rule1.Exprs = []expr.Any{
				&expr.Meta{Key: expr.MetaKeyIIFNAME, Register: 1},
			}
			rule2.Exprs = []expr.Any{
				&expr.Lookup{SourceRegister: 1, SetName: "set", SetID: 1},
			}

			Expect(compareRuleExpressions("test-rule", rule1, rule2)).To(BeFalse())
		})

		It("should return true regardless of expression order (current implementation behavior)", func() {
			// Note: The current implementation of compareRuleExpressions allows for expressions
			// to be in different order. This test documents this behavior.
			e1 := &expr.Meta{Key: expr.MetaKeyIIFNAME, Register: 1}
			e2 := &expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte("eth0")}

			rule1.Exprs = []expr.Any{e1, e2}
			rule2.Exprs = []expr.Any{e2, e1} // Swapped order

			Expect(compareRuleExpressions("test-rule", rule1, rule2)).To(BeTrue())
		})
	})
})
