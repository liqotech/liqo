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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	firewallv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
)

var _ = Describe("Set Utils", func() {
	Describe("ConvertSetData", func() {
		Context("nil data type", func() {
			It("should return empty data when data is nil", func() {
				start, end, err := ConvertSetData(nil, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(start).To(BeEmpty())
				Expect(end).To(BeNil())
			})

			It("should error when data is set but data type is nil", func() {
				_, _, err := ConvertSetData(ptr.To("10.0.0.1"), nil)
				Expect(err).To(HaveOccurred())
			})
		})

		It("should error when data is nil but data type is set", func() {
			_, _, err := ConvertSetData(nil, ptr.To(firewallv1beta1.SetDataTypeIPAddr))
			Expect(err).To(HaveOccurred())
		})

		Context("integer", func() {
			DescribeTable("should encode the value as 4 bytes big endian",
				func(value string, expected []byte) {
					start, end, err := ConvertSetData(ptr.To(value), ptr.To(firewallv1beta1.SetDataTypeInteger))
					Expect(err).NotTo(HaveOccurred())
					Expect(start).To(Equal(expected))
					Expect(end).To(BeNil())
				},
				Entry("zero", "0", []byte{0, 0, 0, 0}),
				Entry("small", "42", []byte{0, 0, 0, 42}),
				Entry("multi byte", "16909060", []byte{1, 2, 3, 4}),
				Entry("max uint32", "4294967295", []byte{0xff, 0xff, 0xff, 0xff}),
			)

			It("should error on non integer value", func() {
				_, _, err := ConvertSetData(ptr.To("abc"), ptr.To(firewallv1beta1.SetDataTypeInteger))
				Expect(err).To(HaveOccurred())
			})
		})

		Context("ipv4_addr", func() {
			It("should encode the IP as 4 bytes", func() {
				start, end, err := ConvertSetData(ptr.To("192.168.1.10"), ptr.To(firewallv1beta1.SetDataTypeIPAddr))
				Expect(err).NotTo(HaveOccurred())
				Expect(start).To(Equal([]byte{192, 168, 1, 10}))
				Expect(end).To(BeNil())
			})

			It("should error on invalid IP", func() {
				_, _, err := ConvertSetData(ptr.To("not-an-ip"), ptr.To(firewallv1beta1.SetDataTypeIPAddr))
				Expect(err).To(HaveOccurred())
			})

			It("should error on IPv6 address", func() {
				_, _, err := ConvertSetData(ptr.To("fd00::1"), ptr.To(firewallv1beta1.SetDataTypeIPAddr))
				Expect(err).To(HaveOccurred())
			})

			It("should error on CIDR value", func() {
				_, _, err := ConvertSetData(ptr.To("10.0.0.0/24"), ptr.To(firewallv1beta1.SetDataTypeIPAddr))
				Expect(err).To(HaveOccurred())
			})
		})

		Context("ipv4_cidr", func() {
			DescribeTable("should return the interval start and the (exclusive) interval end",
				func(cidr string, expStart, expEnd []byte) {
					start, end, err := ConvertSetData(ptr.To(cidr), ptr.To(firewallv1beta1.SetDataTypeIPCIDR))
					Expect(err).NotTo(HaveOccurred())
					Expect(start).To(Equal(expStart))
					Expect(end).To(Equal(expEnd))
				},
				Entry("/24", "10.0.1.0/24", []byte{10, 0, 1, 0}, []byte{10, 0, 2, 0}),
				Entry("/32", "10.0.1.7/32", []byte{10, 0, 1, 7}, []byte{10, 0, 1, 8}),
				Entry("not aligned", "10.0.1.7/24", []byte{10, 0, 1, 0}, []byte{10, 0, 2, 0}),
				Entry("carry over multiple bytes", "10.255.255.0/24", []byte{10, 255, 255, 0}, []byte{11, 0, 0, 0}),
			)

			DescribeTable("should return no interval end when the range reaches the end of the address space",
				func(cidr string, expStart []byte) {
					start, end, err := ConvertSetData(ptr.To(cidr), ptr.To(firewallv1beta1.SetDataTypeIPCIDR))
					Expect(err).NotTo(HaveOccurred())
					Expect(start).To(Equal(expStart))
					Expect(end).To(BeNil())
				},
				Entry("/0", "0.0.0.0/0", []byte{0, 0, 0, 0}),
				Entry("last /24", "255.255.255.0/24", []byte{255, 255, 255, 0}),
				Entry("last /32", "255.255.255.255/32", []byte{255, 255, 255, 255}),
			)

			It("should error on IPv6 CIDR", func() {
				_, _, err := ConvertSetData(ptr.To("fd00::/64"), ptr.To(firewallv1beta1.SetDataTypeIPCIDR))
				Expect(err).To(HaveOccurred())
			})

			It("should error on plain IP", func() {
				_, _, err := ConvertSetData(ptr.To("10.0.0.1"), ptr.To(firewallv1beta1.SetDataTypeIPCIDR))
				Expect(err).To(HaveOccurred())
			})
		})

		It("should error on unknown data type", func() {
			_, _, err := ConvertSetData(ptr.To("1"), ptr.To(firewallv1beta1.SetDataType("unknown")))
			Expect(err).To(HaveOccurred())
		})
	})
})
