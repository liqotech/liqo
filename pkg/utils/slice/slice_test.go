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

package slice_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"github.com/liqotech/liqo/pkg/utils/slice"
)

var _ = Describe("Slice utility functions", func() {

	Describe("The ContainsString function", func() {
		type ContainsStringCase struct {
			Slice    []string
			Item     string
			Expected bool
		}

		DescribeTable("Should return the correct output",
			func(c ContainsStringCase) {
				Expect(slice.ContainsString(c.Slice, c.Item)).To(BeIdenticalTo(c.Expected))
			},
			Entry("When the slice is nil", ContainsStringCase{
				Slice: nil, Item: "baz", Expected: false,
			}),
			Entry("When the slice is empty", ContainsStringCase{
				Slice: []string{}, Item: "baz", Expected: false,
			}),
			Entry("When the slice does not contain the element", ContainsStringCase{
				Slice: []string{"foo", "bar"}, Item: "baz", Expected: false,
			}),
			Entry("When the slice contains the element", ContainsStringCase{
				Slice: []string{"foo", "bar", "baz"}, Item: "baz", Expected: true,
			}),
		)
	})

	Describe("The RemoveString function", func() {
		type RemoveStringCase struct {
			Slice    []string
			Item     string
			Expected []string
		}

		DescribeTable("Should return the correct output",
			func(c RemoveStringCase) {
				Expect(slice.RemoveString(c.Slice, c.Item)).To(ConsistOf(c.Expected))
			},
			Entry("When the slice is nil", RemoveStringCase{
				Slice: nil, Item: "baz", Expected: nil,
			}),
			Entry("When the slice is empty", RemoveStringCase{
				Slice: []string{}, Item: "baz", Expected: []string{},
			}),
			Entry("When the slice does not contain the element", RemoveStringCase{
				Slice: []string{"foo", "bar"}, Item: "baz", Expected: []string{"foo", "bar"},
			}),
			Entry("When the slice contains the element", RemoveStringCase{
				Slice: []string{"foo", "bar", "baz"}, Item: "baz", Expected: []string{"foo", "bar"},
			}),
		)
	})
})
