package utils

import (
	"net"

	firewallv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Firewall common utilities", func() {
	Describe("GetIPValueType", func() {
		It("returns Void for nil value", func() {
			var v *string = nil
			typ, err := GetIPValueType(v)
			Expect(err).To(BeNil())
			Expect(typ).To(Equal(firewallv1beta1.IPValueTypeVoid))
		})

		It("detects CIDR as Subnet", func() {
			s := "10.0.0.0/24"
			typ, err := GetIPValueType(&s)
			Expect(err).To(BeNil())
			Expect(typ).To(Equal(firewallv1beta1.IPValueTypeSubnet))
		})

		It("detects single IP", func() {
			s := "192.168.1.1"
			typ, err := GetIPValueType(&s)
			Expect(err).To(BeNil())
			Expect(typ).To(Equal(firewallv1beta1.IPValueTypeIP))
		})

		It("detects IP range", func() {
			s := "192.168.1.1-192.168.1.10"
			typ, err := GetIPValueType(&s)
			Expect(err).To(BeNil())
			Expect(typ).To(Equal(firewallv1beta1.IPValueTypeRange))
		})

		It("detects named set", func() {
			s := "@myset"
			typ, err := GetIPValueType(&s)
			Expect(err).To(BeNil())
			Expect(typ).To(Equal(firewallv1beta1.IPValueTypeNamedSet))
		})

		It("returns error for invalid value", func() {
			s := "not-an-ip"
			typ, err := GetIPValueType(&s)
			Expect(err).ToNot(BeNil())
			Expect(typ).To(Equal(firewallv1beta1.IPValueTypeVoid))
		})
	})

	Describe("GetIPValueRange", func() {
		It("parses a valid range", func() {
			start, end, err := GetIPValueRange("192.168.1.1 - 192.168.1.10")
			Expect(err).To(BeNil())
			Expect(start.String()).To(Equal(net.ParseIP("192.168.1.1").String()))
			Expect(end.String()).To(Equal(net.ParseIP("192.168.1.10").String()))
		})

		It("returns error on invalid format", func() {
			_, _, err := GetIPValueRange("192.168.1.1")
			Expect(err).ToNot(BeNil())
		})
	})

	Describe("GetIPValueNamedSet", func() {
		It("parses valid named set", func() {
			name, err := GetIPValueNamedSet("@somename")
			Expect(err).To(BeNil())
			Expect(name).To(Equal("somename"))
		})

		It("returns error for missing @", func() {
			_, err := GetIPValueNamedSet("somename")
			Expect(err).ToNot(BeNil())
		})

		It("returns error for empty name", func() {
			_, err := GetIPValueNamedSet("@")
			Expect(err).ToNot(BeNil())
		})
	})

	Describe("GetPortValueType", func() {
		It("returns Void for nil pointer", func() {
			var p *string = nil
			typ, err := GetPortValueType(p)
			Expect(err).To(BeNil())
			Expect(typ).To(Equal(firewallv1beta1.PortValueTypeVoid))
		})

		It("detects port range", func() {
			s := "1000-2000"
			typ, err := GetPortValueType(&s)
			Expect(err).To(BeNil())
			Expect(typ).To(Equal(firewallv1beta1.PortValueTypeRange))
		})

		It("detects single port", func() {
			s := "8080"
			typ, err := GetPortValueType(&s)
			Expect(err).To(BeNil())
			Expect(typ).To(Equal(firewallv1beta1.PortValueTypePort))
		})

		It("returns error for invalid port value", func() {
			s := "notaport"
			typ, err := GetPortValueType(&s)
			Expect(err).ToNot(BeNil())
			Expect(typ).To(Equal(firewallv1beta1.PortValueTypeVoid))
		})
	})
})
