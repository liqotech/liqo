// Copyright 2019-2025 The Liqo Authors
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

package syncset_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/liqotech/liqo/pkg/utils/syncset"
)

var _ = Describe("SyncSet utility functions", func() {
	var (
		ss       *syncset.SyncSet
		elements []string
	)

	BeforeEach(func() { ss = syncset.New() })

	JustBeforeEach(func() {
		elements = make([]string, 0)
		ss.ForEach(func(el string) {
			elements = append(elements, el)
		})
	})

	Describe("The Add function", func() {
		When("Adding a single element", func() {
			BeforeEach(func() { ss.Add("foo") })
			It("The set should contain that single element", func() {
				Expect(elements).To(HaveLen(1))
				Expect(elements).To(ContainElement("foo"))
			})
		})

		When("Adding a single element twice", func() {
			BeforeEach(func() {
				ss.Add("foo")
				ss.Add("foo")
			})
			It("The set should contain that single element (once)", func() {
				Expect(elements).To(HaveLen(1))
				Expect(elements).To(ContainElement("foo"))
			})
		})

		When("Adding multiple elements", func() {
			BeforeEach(func() {
				ss.Add("foo")
				ss.Add("bar")
				ss.Add("baz")
			})
			It("The set should contain that elements", func() {
				Expect(elements).To(HaveLen(3))
				Expect(elements).To(ContainElement("foo"))
				Expect(elements).To(ContainElement("bar"))
				Expect(elements).To(ContainElement("baz"))
			})
		})
	})

	Describe("The Remove function", func() {
		BeforeEach(func() {
			ss.Add("foo")
			ss.Add("bar")
			ss.Add("baz")
		})

		When("Removing a single element", func() {
			BeforeEach(func() { ss.Remove("foo") })
			It("The set should no longer contain that element", func() {
				Expect(elements).To(HaveLen(2))
				Expect(elements).To(ContainElement("bar"))
				Expect(elements).To(ContainElement("baz"))
			})
		})

		When("Removing all elements", func() {
			BeforeEach(func() {
				ss.Remove("foo")
				ss.Remove("bar")
				ss.Remove("baz")
			})
			It("The set should contain no elements", func() {
				Expect(elements).To(HaveLen(0))
			})
		})

		When("Removing a non-existing element", func() {
			BeforeEach(func() {
				ss.Remove("other")
			})
			It("The set should still contain all elements", func() {
				Expect(elements).To(HaveLen(3))
				Expect(elements).To(ContainElement("foo"))
				Expect(elements).To(ContainElement("bar"))
				Expect(elements).To(ContainElement("baz"))
			})
		})
	})
})
