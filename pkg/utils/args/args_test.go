package args

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

func TestParseArguments(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ParseArguments Suite")
}

var _ = Describe("ParseArguments", func() {

	Context("StringMap", func() {

		type parseMapTestcase struct {
			str           string
			expectedError OmegaMatcher
			expectedMap   map[string]string
		}

		DescribeTable("StringMap table",

			func(c parseMapTestcase) {
				sm := StringMap{}
				err := sm.Set(c.str)
				Expect(err).To(c.expectedError)
				Expect(sm.StringMap).To(Equal(c.expectedMap))
				if err == nil {
					Expect(sm.String()).To(Equal(c.str))
				}
			},

			Entry("empty string", parseMapTestcase{
				str:           "",
				expectedError: Not(HaveOccurred()),
				expectedMap:   map[string]string{},
			}),

			Entry("single value map", parseMapTestcase{
				str:           "key1=val1",
				expectedError: Not(HaveOccurred()),
				expectedMap: map[string]string{
					"key1": "val1",
				},
			}),

			Entry("multi values map", parseMapTestcase{
				str:           "key1=val1,key2=val2",
				expectedError: Not(HaveOccurred()),
				expectedMap: map[string]string{
					"key1": "val1",
					"key2": "val2",
				},
			}),

			Entry("invalid map", parseMapTestcase{
				str:           "key1,key2=val2",
				expectedError: HaveOccurred(),
				expectedMap:   map[string]string{},
			}),
		)

	})

	Context("StringList", func() {

		type parseListTestcase struct {
			str          string
			expectedList []string
		}

		DescribeTable("StringList table",

			func(c parseListTestcase) {
				sl := StringList{}
				Expect(sl.Set(c.str)).To(Succeed())
				Expect(sl.StringList).To(Equal(c.expectedList))
				Expect(sl.String()).To(Equal(c.str))
			},

			Entry("empty string", parseListTestcase{
				str:          "",
				expectedList: []string{},
			}),

			Entry("single value list", parseListTestcase{
				str: "val1",
				expectedList: []string{
					"val1",
				},
			}),

			Entry("multi values list", parseListTestcase{
				str: "val1,val2",
				expectedList: []string{
					"val1",
					"val2",
				},
			}),
		)

	})

})
