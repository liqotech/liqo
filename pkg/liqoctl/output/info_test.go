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

package output

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("Info", func() {
	var (
		root Section
	)
	BeforeEach(func() {
		root = NewRootSection()
	})
	Context("Creating a new root InfoSection", func() {

		It("should return a valid InfoSection", func() {
			sectionTest := section{
				title:  "",
				detail: "",
			}
			Expect(root.(*section)).To(PointTo(Equal(sectionTest)))
		})
	})
	Context("Populating a root node with some sections", func() {
		JustBeforeEach(func() {
			root.AddSectionWithDetail("section1", "detail1").
				AddSectionWithDetail("section1-1", "detail1-1")
			root.AddSectionWithDetail("section2", "detail2").
				AddSectionWithDetail("section2-1", "detail2-1")
		})
		It("should return a valid InfoSection tree", func() {
			s := root.(*section)
			Expect(len(s.sections)).To(Equal(2))
			Expect(s.sections[0].title).To(Equal("section1"))
			Expect(s.sections[0].detail).To(Equal("detail1"))
			Expect(s.sections[0].sections[0].title).To(Equal("section1-1"))
			Expect(s.sections[0].sections[0].detail).To(Equal("detail1-1"))
			Expect(s.sections[1].title).To(Equal("section2"))
			Expect(s.sections[1].detail).To(Equal("detail2"))
			Expect(s.sections[1].sections[0].title).To(Equal("section2-1"))
			Expect(s.sections[1].sections[0].detail).To(Equal("detail2-1"))
		})
	})
	Context("Populating a root node with some data", func() {
		JustBeforeEach(func() {
			root.AddEntry("key1", "value1")
			root.AddEntry("key2", "value2")
			root.AddEntry("key3", "value3")
		})
		It("should return a valid InfoSection tree", func() {
			s := root.(*section)
			Expect(len(s.sections)).To(Equal(0))
			Expect(len(s.entries)).To(Equal(3))
			Expect(s.entries[0].key).To(Equal("key1"))
			Expect(s.entries[0].values[0]).To(Equal("value1"))
			Expect(s.entries[1].key).To(Equal("key2"))
			Expect(s.entries[1].values[0]).To(Equal("value2"))
			Expect(s.entries[2].key).To(Equal("key3"))
			Expect(s.entries[2].values[0]).To(Equal("value3"))
		})
	})
	Context("Populating a root node with some data and sections", func() {
		JustBeforeEach(func() {
			section1 := root.AddSectionWithDetail("section1", "detail1")
			section2 := root.AddSectionWithDetail("section2", "detail2")
			section3 := section2.AddSectionWithDetail("section3", "detail3")
			section1.AddEntry("key1", "value1")
			section1.AddEntry("key2", "value2")
			section1.AddEntry("key3", "value3")
			section3.AddEntry("key1", "value1")
			section3.AddEntry("key2", "value2")
			section3.AddEntry("key3", "value3")
		})
		It("should return a valid InfoSection tree", func() {
			s := root.(*section)
			Expect(len(s.sections)).To(Equal(2))
			Expect(s.sections[0].title).To(Equal("section1"))
			Expect(s.sections[1].title).To(Equal("section2"))
			Expect(s.sections[1].sections[0].title).To(Equal("section3"))

			Expect(len(s.sections[0].entries)).To(Equal(3))
			Expect(s.sections[0].entries[0].values[0]).To(Equal("value1"))
			Expect(s.sections[0].entries[1].values[0]).To(Equal("value2"))
			Expect(s.sections[0].entries[2].values[0]).To(Equal("value3"))

			Expect(len(s.sections[1].sections[0].entries)).To(Equal(3))
			Expect(s.sections[1].sections[0].entries[0].values[0]).To(Equal("value1"))
			Expect(s.sections[1].sections[0].entries[1].values[0]).To(Equal("value2"))
			Expect(s.sections[1].sections[0].entries[2].values[0]).To(Equal("value3"))
		})
	})
})
