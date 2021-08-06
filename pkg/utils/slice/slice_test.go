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
