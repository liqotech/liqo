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

package maps_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/liqotech/liqo/pkg/utils/maps"
)

var _ = Describe("Maps", func() {
	Describe("The FilterMap function", func() {
		var (
			input, output map[string]string
			filter        maps.FilterType[string]
		)

		BeforeEach(func() {
			input = map[string]string{
				"key1": "value1",
				"key2": "value2",
				"key3": "value3",
				"key4": "value4",
			}
		})

		JustBeforeEach(func() { output = maps.Filter(input, filter) })

		Context("whitelist filtering", func() {
			BeforeEach(func() { filter = maps.FilterWhitelist("key1", "key3") })

			It("should not mutate the original map", func() { Expect(input).To(HaveLen(4)) })
			It("should correctly preserve only the whitelisted elements", func() {
				Expect(output).To(HaveLen(2))
				Expect(output).To(HaveKeyWithValue("key1", "value1"))
				Expect(output).To(HaveKeyWithValue("key3", "value3"))
			})
		})

		Context("blacklist filtering", func() {
			BeforeEach(func() { filter = maps.FilterBlacklist("key2", "key4") })

			It("should not mutate the original map", func() { Expect(input).To(HaveLen(4)) })
			It("should correctly remove only the blacklisted elements", func() {
				GinkgoWriter.Println(output)
				Expect(output).To(HaveLen(2))
				Expect(output).To(HaveKeyWithValue("key1", "value1"))
				Expect(output).To(HaveKeyWithValue("key3", "value3"))
			})
		})
	})
})
