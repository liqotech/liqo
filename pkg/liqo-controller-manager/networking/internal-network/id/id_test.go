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

package id

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ID Tests", func() {

	var _ = Context("Generic Manager", func() {

		var (
			manager *Manager[uint64]
		)

		BeforeEach(func() {
			manager = New[uint64]()
		})

		Context("with an empty manager", func() {

			It("should allocate an ID", func() {
				id, err := manager.Allocate("test")
				Expect(err).To(Succeed())
				Expect(id).To(BeNumerically("==", 0))
				Expect(manager.allocated).To(HaveKey("test"))
				Expect(manager.allocatedReverse).To(HaveKey(uint64(0)))
			})

			It("should release an ID", func() {
				manager.Release("test")
				Expect(manager.allocated).To(Not(HaveKey("test")))
				Expect(manager.allocatedReverse).To(Not(HaveKey(uint64(0))))
			})

			It("should not allocate an ID larger than the max", func() {
				manager.nextAllocatedID = manager.maxID
				id, err := manager.Allocate("test")
				Expect(err).To(Succeed())
				Expect(id).To(BeNumerically("==", 0))
			})

		})

		Context("with a full Manager", func() {

			JustBeforeEach(func() {
				manager.maxID = 20
				for i := uint64(0); i <= manager.maxID-1; i++ {
					_, err := manager.Allocate(fmt.Sprintf("test%d", i))
					Expect(err).To(Succeed())
				}
			})

			It("should not allocate an ID", func() {
				id, err := manager.Allocate("test_different")
				Expect(err).To(HaveOccurred())
				Expect(id).To(BeNumerically("==", 0))
			})

			It("should release an ID", func() {
				Expect(manager.allocated).To(HaveKey("test0"))
				Expect(manager.allocatedReverse).To(HaveKey(uint64(0)))

				manager.Release("test0")

				Expect(manager.allocated).To(Not(HaveKey("test0")))
				Expect(manager.allocatedReverse).To(Not(HaveKey(uint64(0))))
			})

			It("should reallocate an ID", func() {
				manager.Release("test10")
				id, err := manager.Allocate("test_different")
				Expect(err).To(Succeed())
				Expect(id).To(BeNumerically("==", 10))
			})
		})

		Context("with an allocated ID", func() {

			JustBeforeEach(func() {
				_, err := manager.Allocate("test")
				Expect(err).To(Succeed())
			})

			It("should allocate a new ID", func() {
				id, err := manager.Allocate("test2")
				Expect(err).To(Succeed())
				Expect(id).To(BeNumerically("==", 1))
			})

			It("should allocate the same ID", func() {
				id, err := manager.Allocate("test")
				Expect(err).To(Succeed())
				Expect(id).To(BeNumerically("==", 0))
			})

		})

		Context("with a configured ID", func() {

			JustBeforeEach(func() {
				err := manager.Configure("test", 10)
				Expect(err).To(Succeed())
			})

			It("should allocate the configured ID", func() {
				id, err := manager.Allocate("test")
				Expect(err).To(Succeed())
				Expect(id).To(BeNumerically("==", 10))
			})

			It("should not allocate the configured ID", func() {
				id, err := manager.Allocate("test2")
				Expect(err).To(Succeed())
				Expect(id).To(BeNumerically("==", 0))
			})

		})

		Context("with two configured ID", func() {

			JustBeforeEach(func() {
				err := manager.Configure("test", 0)
				Expect(err).To(Succeed())

				err = manager.Configure("test2", 1)
				Expect(err).To(Succeed())
			})

			It("should release ID", func() {
				manager.Release("test")

				Expect(manager.allocated).To(Not(HaveKey("test")))
				Expect(manager.allocatedReverse).To(Not(HaveKey(uint64(0))))

				Expect(manager.allocated).To(HaveKey("test2"))
				Expect(manager.allocatedReverse).To(HaveKey(uint64(1)))
			})

		})

	})

})
