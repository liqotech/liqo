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

package args

import (
	"net"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestParseArguments(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ParseArguments Suite")
}

var parseCIDR = func(s string) net.IPNet {
	_, cidr, err := net.ParseCIDR(s)
	Expect(err).ToNot(HaveOccurred())
	return *cidr
}

var _ = Describe("ParseArguments", func() {

	Context("StringMap", func() {

		type parseMapTestcase struct {
			str             string
			expectedError   OmegaMatcher
			expectedMap     map[string]string
			expectedStrings []types.GomegaMatcher
		}

		DescribeTable("StringMap table",

			func(c parseMapTestcase) {
				sm := StringMap{}
				err := sm.Set(c.str)
				Expect(err).To(c.expectedError)
				Expect(sm.StringMap).To(Equal(c.expectedMap))
				if err == nil {
					Expect(sm.String()).To(Or(c.expectedStrings...))
				}
			},

			Entry("empty string", parseMapTestcase{
				str:           "",
				expectedError: Not(HaveOccurred()),
				expectedMap:   map[string]string{},
				expectedStrings: []types.GomegaMatcher{
					Equal(""),
				},
			}),

			Entry("single value map", parseMapTestcase{
				str:           "key1=val1",
				expectedError: Not(HaveOccurred()),
				expectedMap: map[string]string{
					"key1": "val1",
				},
				expectedStrings: []types.GomegaMatcher{
					Equal("key1=val1"),
				},
			}),

			Entry("multi values map", parseMapTestcase{
				str:           "key1=val1,key2=val2",
				expectedError: Not(HaveOccurred()),
				expectedMap: map[string]string{
					"key1": "val1",
					"key2": "val2",
				},
				expectedStrings: []types.GomegaMatcher{
					Equal("key1=val1,key2=val2"),
					Equal("key2=val2,key1=val1"),
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

	Context("CIDRList", func() {

		type parseListTestcase struct {
			str          string
			expectedList []net.IPNet
		}

		DescribeTable("CIDRList table",

			func(c parseListTestcase) {
				cl := CIDRList{}
				Expect(cl.Set(c.str)).To(Succeed())
				Expect(cl.CIDRList).To(Equal(c.expectedList))
				Expect(cl.String()).To(Equal(c.str))
			},

			Entry("empty string", parseListTestcase{
				str:          "",
				expectedList: []net.IPNet{},
			}),

			Entry("single value list", parseListTestcase{
				str: "10.0.0.0/16",
				expectedList: []net.IPNet{
					parseCIDR("10.0.0.0/16"),
				},
			}),

			Entry("multi values list", parseListTestcase{
				str: "10.0.0.0/16,10.120.0.0/16",
				expectedList: []net.IPNet{
					parseCIDR("10.0.0.0/16"),
					parseCIDR("10.120.0.0/16"),
				},
			}),
		)

	})

	Context("CIDR", func() {

		type parseCidrTestCase struct {
			cidr          string
			expectedError OmegaMatcher
		}

		DescribeTable("CIDR table",
			func(c parseCidrTestCase) {
				cl := CIDR{}
				Expect(cl.Set(c.cidr)).To(c.expectedError)
			},

			Entry("empty string", parseCidrTestCase{
				cidr:          "",
				expectedError: HaveOccurred(),
			}),

			Entry("correct cidr", parseCidrTestCase{
				cidr:          "10.0.0.0/16",
				expectedError: Succeed(),
			}),

			Entry("incorrect cidr", parseCidrTestCase{
				cidr:          "10.0.0..0/16",
				expectedError: HaveOccurred(),
			}),
		)

	})

	Context("ClusterIdentity", func() {

		type parseClusterIDTestCase struct {
			args          []string
			expectedError OmegaMatcher
		}

		DescribeTable("ClusterID table",
			func(c parseClusterIDTestCase) {
				flagset := pflag.NewFlagSet("test", pflag.ContinueOnError)
				flags := NewClusterIDFlags(true, flagset)
				Expect(flagset.Parse(c.args)).To(Succeed())
				_, err := flags.Read()
				Expect(err).To(c.expectedError)
			},

			Entry("invalid cluster ID", parseClusterIDTestCase{
				args:          []string{"--cluster-id=Foo!"},
				expectedError: HaveOccurred(),
			}),
		)

	})

	Context("Percentage", func() {

		type parsePercentageTestcase struct {
			str            string
			expectedError  OmegaMatcher
			expectedValue  uint64
			expectedString string
		}

		DescribeTable("Percentage table",

			func(c parsePercentageTestcase) {
				p := Percentage{}
				err := p.Set(c.str)
				Expect(err).To(c.expectedError)

				if err == nil {
					Expect(p.Val).To(Equal(c.expectedValue))
					Expect(p.String()).To(Equal(c.expectedString))
				}
			},

			Entry("empty string", parsePercentageTestcase{
				str:            "",
				expectedError:  Not(HaveOccurred()),
				expectedValue:  0,
				expectedString: "0",
			}),

			Entry("invalid string", parsePercentageTestcase{
				str:           "test",
				expectedError: HaveOccurred(),
			}),

			Entry("value string", parsePercentageTestcase{
				str:            "67",
				expectedError:  Not(HaveOccurred()),
				expectedValue:  67,
				expectedString: "67",
			}),
		)

	})

	Context("Quantity", func() {
		type parseQuantityTestcase struct {
			str            string
			expectedError  OmegaMatcher
			expectedValue  resource.Quantity
			expectedString string
		}

		DescribeTable("Quantity table",
			func(c parseQuantityTestcase) {
				q := Quantity{}
				err := q.Set(c.str)
				Expect(err).To(c.expectedError)

				if err == nil {
					Expect(q.Quantity.Equal(c.expectedValue)).To(BeTrue())
					Expect(q.String()).To(Equal(c.expectedString))
				}
			},

			Entry("invalid string", parseQuantityTestcase{
				str:           "11z",
				expectedError: HaveOccurred(),
			}),

			Entry("valid string", parseQuantityTestcase{
				str:            "55m",
				expectedError:  Not(HaveOccurred()),
				expectedValue:  *resource.NewScaledQuantity(55, resource.Milli),
				expectedString: "55m",
			}),
		)
	})

})
