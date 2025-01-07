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

package slice_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/liqotech/liqo/pkg/utils/slice"
)

var _ = Describe("Slice utility functions", func() {
	Describe("The Remove function", func() {
		type RemoveStringCase struct {
			Slice    []string
			Item     string
			Expected []string
		}

		DescribeTable("Should return the correct output",
			func(c RemoveStringCase) {
				Expect(slice.Remove(c.Slice, c.Item)).To(ConsistOf(c.Expected))
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
