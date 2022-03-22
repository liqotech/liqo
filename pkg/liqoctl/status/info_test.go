// Copyright 2019-2022 The Liqo Authors
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

package status

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Info", func() {
	var (
		infoNode InfoNode
	)
	Context("Creating a new root InfoNode", func() {
		JustBeforeEach(func() {
			infoNode = newRootInfoNode("root")
		})
		It("should return a valid InfoNode", func() {
			infoNodeTest := InfoNode{
				title:     "root",
				detail:    "",
				nextNodes: nil,
				data:      nil,
			}
			Expect(infoNode).To(Equal(infoNodeTest))
		})
	})
	Context("Populating a root node with some sections", func() {
		JustBeforeEach(func() {
			infoNode = newRootInfoNode("root")
			infoNode.addSectionToNode("section1", "detail1").
				addSectionToNode("section1-1", "detail1-1")
			infoNode.addSectionToNode("section2", "detail2").
				addSectionToNode("section2-1", "detail2-1")
		})
		It("should return a valid InfoNode tree", func() {
			Expect(infoNode.title).To(Equal("root"))
			Expect(infoNode.nextNodes[0].title).To(Equal("section1"))
			Expect(infoNode.nextNodes[0].detail).To(Equal("detail1"))
			Expect(infoNode.nextNodes[0].nextNodes[0].title).To(Equal("section1-1"))
			Expect(infoNode.nextNodes[0].nextNodes[0].detail).To(Equal("detail1-1"))
			Expect(infoNode.nextNodes[1].title).To(Equal("section2"))
			Expect(infoNode.nextNodes[1].detail).To(Equal("detail2"))
			Expect(infoNode.nextNodes[1].nextNodes[0].title).To(Equal("section2-1"))
			Expect(infoNode.nextNodes[1].nextNodes[0].detail).To(Equal("detail2-1"))
		})
	})
	Context("Populating a root node with some data", func() {
		JustBeforeEach(func() {
			infoNode = newRootInfoNode("root")
			infoNode.addDataToNode("key1", "value1")
			infoNode.addDataToNode("key2", "value2")
			infoNode.addDataToNode("key3", "value3")
		})
		It("should return a valid InfoNode tree", func() {
			Expect(infoNode.title).To(Equal("root"))
			Expect(infoNode.data[0].key).To(Equal("key1"))
			Expect(infoNode.data[0].value[0]).To(Equal("value1"))
			Expect(infoNode.data[1].key).To(Equal("key2"))
			Expect(infoNode.data[1].value[0]).To(Equal("value2"))
			Expect(infoNode.data[2].key).To(Equal("key3"))
			Expect(infoNode.data[2].value[0]).To(Equal("value3"))
		})
	})
	Context("Populating a root node with some data and sections", func() {
		JustBeforeEach(func() {
			infoNode = newRootInfoNode("root")
			section1 := infoNode.addSectionToNode("section1", "detail1")
			section2 := infoNode.addSectionToNode("section2", "detail2")
			section3 := section2.addSectionToNode("section3", "detail3")
			section1.addDataToNode("key1", "value1")
			section1.addDataToNode("key2", "value2")
			section1.addDataToNode("key3", "value3")
			section3.addDataToNode("key1", "value1")
			section3.addDataToNode("key2", "value2")
			section3.addDataToNode("key3", "value3")
		})
		It("should return a valid InfoNode tree", func() {
			Expect(infoNode.title).To(Equal("root"))
			Expect(infoNode.nextNodes[0].title).To(Equal("section1"))
			Expect(infoNode.nextNodes[1].title).To(Equal("section2"))
			Expect(infoNode.nextNodes[1].nextNodes[0].title).To(Equal("section3"))

			Expect(infoNode.nextNodes[0].data[0].value[0]).To(Equal("value1"))
			Expect(infoNode.nextNodes[0].data[1].value[0]).To(Equal("value2"))
			Expect(infoNode.nextNodes[0].data[2].value[0]).To(Equal("value3"))
			Expect(infoNode.nextNodes[1].nextNodes[0].data[0].value[0]).To(Equal("value1"))
			Expect(infoNode.nextNodes[1].nextNodes[0].data[1].value[0]).To(Equal("value2"))
			Expect(infoNode.nextNodes[1].nextNodes[0].data[2].value[0]).To(Equal("value3"))
		})
	})
	Context("Checking if a title is contained between the next nodes", func() {
		var (
			node1, node2, node3 *InfoNode
		)
		When("the title is contained", func() {
			JustBeforeEach(func() {
				infoNode = newRootInfoNode("root")
				node1 = infoNode.addSectionToNode("section1", "detail1")
				node2 = infoNode.addSectionToNode("section2", "detail2")
				node3 = infoNode.addSectionToNode("section3", "detail3")
			})
			It("should return True", func() {
				Expect(findNodeByTitle(infoNode.nextNodes, "section1")).To(Equal(node1))
				Expect(findNodeByTitle(infoNode.nextNodes, "section2")).To(Equal(node2))
				Expect(findNodeByTitle(infoNode.nextNodes, "section3")).To(Equal(node3))
			})
		})
		When("the title is not contained", func() {

			JustBeforeEach(func() {
				infoNode = newRootInfoNode("root")
				infoNode.addSectionToNode("section1", "detail1")
			})
			It("should return False", func() {
				Expect(findNodeByTitle(infoNode.nextNodes, "section2")).To(BeNil())
			})
		})
	})
})
