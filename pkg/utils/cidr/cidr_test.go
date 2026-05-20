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

package cidr_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	cidrutils "github.com/liqotech/liqo/pkg/utils/cidr"
)

var _ = Describe("CIDR utils", func() {
	Describe("AreAllVoid", func() {
		It("is true for nil and empty slices", func() {
			Expect(cidrutils.AreAllVoid(nil)).To(BeTrue())
			Expect(cidrutils.AreAllVoid([]networkingv1beta1.CIDR{})).To(BeTrue())
		})
		It("is true for slices containing only empty strings", func() {
			Expect(cidrutils.AreAllVoid([]networkingv1beta1.CIDR{"", ""})).To(BeTrue())
		})
		It("is false when at least one element is non-empty", func() {
			Expect(cidrutils.AreAllVoid([]networkingv1beta1.CIDR{"", "10.0.0.0/16"})).To(BeFalse())
			Expect(cidrutils.AreAllVoid([]networkingv1beta1.CIDR{"10.0.0.0/16"})).To(BeFalse())
		})
	})

	Describe("EscapeForName", func() {
		It("replaces / and . with -", func() {
			Expect(cidrutils.EscapeForName("10.244.0.0/16")).To(Equal("10-244-0-0-16"))
			Expect(cidrutils.EscapeForName("192.168.1.0/24")).To(Equal("192-168-1-0-24"))
		})
		It("is a no-op for empty input", func() {
			Expect(cidrutils.EscapeForName("")).To(Equal(""))
		})
	})

	Describe("AllNonVoid", func() {
		It("is false for nil and empty slices", func() {
			Expect(cidrutils.AllNonVoid(nil)).To(BeFalse())
			Expect(cidrutils.AllNonVoid([]networkingv1beta1.CIDR{})).To(BeFalse())
		})
		It("is true when every entry is non-empty", func() {
			Expect(cidrutils.AllNonVoid([]networkingv1beta1.CIDR{"10.0.0.0/16"})).To(BeTrue())
			Expect(cidrutils.AllNonVoid([]networkingv1beta1.CIDR{"10.0.0.0/16", "10.1.0.0/16"})).To(BeTrue())
		})
		It("is false when at least one entry is empty", func() {
			Expect(cidrutils.AllNonVoid([]networkingv1beta1.CIDR{"", "10.0.0.0/16"})).To(BeFalse())
			Expect(cidrutils.AllNonVoid([]networkingv1beta1.CIDR{"10.0.0.0/16", ""})).To(BeFalse())
		})
	})

	Describe("Strings", func() {
		It("returns nil for nil input", func() {
			Expect(cidrutils.Strings(nil)).To(BeNil())
		})
		It("returns an empty slice for an empty input slice", func() {
			Expect(cidrutils.Strings([]networkingv1beta1.CIDR{})).To(Equal([]string{}))
		})
		It("preserves order", func() {
			in := []networkingv1beta1.CIDR{"10.0.0.0/16", "10.1.0.0/16", ""}
			Expect(cidrutils.Strings(in)).To(Equal([]string{"10.0.0.0/16", "10.1.0.0/16", ""}))
		})
	})

	Describe("FromStrings", func() {
		It("returns nil for nil input", func() {
			Expect(cidrutils.FromStrings(nil)).To(BeNil())
		})
		It("returns an empty slice for an empty input slice", func() {
			Expect(cidrutils.FromStrings([]string{})).To(Equal([]networkingv1beta1.CIDR{}))
		})
		It("preserves order", func() {
			in := []string{"10.0.0.0/16", "10.1.0.0/16", ""}
			Expect(cidrutils.FromStrings(in)).To(Equal([]networkingv1beta1.CIDR{"10.0.0.0/16", "10.1.0.0/16", ""}))
		})
		It("round-trips with Strings", func() {
			in := []string{"10.0.0.0/16", "10.1.0.0/16"}
			Expect(cidrutils.Strings(cidrutils.FromStrings(in))).To(Equal(in))
		})
	})

	Describe("EqualOrdered", func() {
		It("returns true for two nil slices", func() {
			Expect(cidrutils.EqualOrdered(nil, nil)).To(BeTrue())
		})
		It("returns true for two equal slices", func() {
			a := []networkingv1beta1.CIDR{"10.0.0.0/16", "10.1.0.0/16"}
			b := []networkingv1beta1.CIDR{"10.0.0.0/16", "10.1.0.0/16"}
			Expect(cidrutils.EqualOrdered(a, b)).To(BeTrue())
		})
		It("returns false when order differs", func() {
			a := []networkingv1beta1.CIDR{"10.0.0.0/16", "10.1.0.0/16"}
			b := []networkingv1beta1.CIDR{"10.1.0.0/16", "10.0.0.0/16"}
			Expect(cidrutils.EqualOrdered(a, b)).To(BeFalse())
		})
		It("returns false when lengths differ", func() {
			a := []networkingv1beta1.CIDR{"10.0.0.0/16"}
			b := []networkingv1beta1.CIDR{"10.0.0.0/16", "10.1.0.0/16"}
			Expect(cidrutils.EqualOrdered(a, b)).To(BeFalse())
		})
	})

	Describe("EqualAsSet", func() {
		It("returns true for two nil slices", func() {
			Expect(cidrutils.EqualAsSet(nil, nil)).To(BeTrue())
		})
		It("returns true regardless of order", func() {
			a := []networkingv1beta1.CIDR{"10.0.0.0/16", "10.1.0.0/16"}
			b := []networkingv1beta1.CIDR{"10.1.0.0/16", "10.0.0.0/16"}
			Expect(cidrutils.EqualAsSet(a, b)).To(BeTrue())
		})
		It("treats duplicates as significant", func() {
			a := []networkingv1beta1.CIDR{"10.0.0.0/16", "10.0.0.0/16", "10.1.0.0/16"}
			b := []networkingv1beta1.CIDR{"10.0.0.0/16", "10.1.0.0/16", "10.1.0.0/16"}
			Expect(cidrutils.EqualAsSet(a, b)).To(BeFalse())
		})
		It("returns false when lengths differ", func() {
			a := []networkingv1beta1.CIDR{"10.0.0.0/16"}
			b := []networkingv1beta1.CIDR{"10.0.0.0/16", "10.1.0.0/16"}
			Expect(cidrutils.EqualAsSet(a, b)).To(BeFalse())
		})
	})
})
