// Copyright 2019-2026 The Liqo Authors
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

package utils_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	"github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/external-network/utils"
)

var _ = Describe("ParseEndpoint", func() {

	It("should return an empty endpoint from an empty map", func() {
		res := utils.ParseEndpoint(map[string]interface{}{})

		Expect(res.Addresses).To(BeEmpty())
		Expect(res.Port).To(BeZero()) //nolint:staticcheck // Port is intentionally used for backward compatibility.
		Expect(res.Ports).To(BeNil())
		Expect(res.Protocol).To(BeNil())
	})

	It("should parse the single legacy port and leave ports empty", func() {
		res := utils.ParseEndpoint(map[string]interface{}{
			"port": int64(51840),
		})

		Expect(res.Port).To(Equal(int32(51840))) //nolint:staticcheck // Port is intentionally used for backward compatibility.
		Expect(res.Ports).To(BeNil())
	})

	It("should parse the ports list preserving the order", func() {
		res := utils.ParseEndpoint(map[string]interface{}{
			"ports": []interface{}{int64(51840), int64(51821), int64(51830)},
		})

		Expect(res.Ports).To(Equal([]int32{51840, 51821, 51830}))
		Expect(res.Port).To(BeZero()) //nolint:staticcheck // Port is intentionally used for backward compatibility.
	})

	It("should leave ports nil with an empty list", func() {
		res := utils.ParseEndpoint(map[string]interface{}{
			"ports": []interface{}{},
		})

		Expect(res.Ports).To(BeNil())
	})

	It("should discard the elements that are not int64 without panicking", func() {
		input := map[string]interface{}{
			"ports": []interface{}{int64(51840), "51841", 51842, 51843.0, int64(51844)},
		}

		var ports []int32
		Expect(func() { ports = utils.ParseEndpoint(input).Ports }).NotTo(Panic())
		Expect(ports).To(Equal([]int32{51840, 51844}))
	})

	DescribeTable("should ignore a ports field that is not a list", func(value interface{}) {
		var ports []int32
		Expect(func() {
			ports = utils.ParseEndpoint(map[string]interface{}{"ports": value}).Ports
		}).NotTo(Panic())
		Expect(ports).To(BeNil())
	},
		Entry("an integer", int64(51840)),
		Entry("a string", "51840"),
		Entry("a map", map[string]interface{}{"a": int64(1)}),
	)

	It("should parse addresses and protocol together with ports", func() {
		res := utils.ParseEndpoint(map[string]interface{}{
			"addresses": []interface{}{"10.0.0.1", "10.0.0.2"},
			"ports":     []interface{}{int64(51840), int64(51841)},
			"protocol":  "UDP",
		})

		Expect(res.Addresses).To(Equal([]string{"10.0.0.1", "10.0.0.2"}))
		Expect(res.Ports).To(Equal([]int32{51840, 51841}))
		Expect(res.Protocol).NotTo(BeNil())
		Expect(*res.Protocol).To(Equal(corev1.ProtocolUDP))
	})

	It("should parse both port and ports when both are present", func() {
		// The parser does not enforce the mutual exclusion, which is delegated to the CRD validation.
		res := utils.ParseEndpoint(map[string]interface{}{
			"port":  int64(51840),
			"ports": []interface{}{int64(51841), int64(51842)},
		})

		Expect(res.Port).To(Equal(int32(51840))) //nolint:staticcheck // Port is intentionally used for backward compatibility.
		Expect(res.Ports).To(Equal([]int32{51841, 51842}))
	})
})
