// Copyright 2019-2024 The Liqo Authors
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
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ipamutils "github.com/liqotech/liqo/pkg/ipam/utils"
)

const (
	invalidValue      = "invalidValue"
	CIDRAddressNetErr = "CIDR address"
	labelKey          = "ipam.liqo.io/LabelKey"
	labelValue        = "LabelValue"
	annotationKey     = "ipam.liqo.io/AnnotationKey"
	annotationValue   = "AnnotationValue"
)

var (
	// corev1.Pod impements the client.Object interface.
	testPod *corev1.Pod
)

var _ = Describe("Mapping", func() {
	JustBeforeEach(func() {
		testPod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					labelKey: labelValue,
				},
				Annotations: map[string]string{
					annotationKey: annotationValue,
				},
			}}
	})

	DescribeTable("MapIPToNetwork",
		func(oldIp, newPodCidr, expectedIP string, expectedErr string) {
			ip, err := ipamutils.MapIPToNetwork(oldIp, newPodCidr)
			if expectedErr != "" {
				Expect(err.Error()).To(Equal(expectedErr))
			} else {
				Expect(err).ToNot(HaveOccurred())
			}
			Expect(ip).To(Equal(expectedIP))
		},
		Entry("Mapping 10.2.1.3 to 10.0.4.0/24", "10.0.4.0/24", "10.2.1.3", "10.0.4.3", ""),
		Entry("Mapping 10.2.1.128 to 10.0.4.0/24", "10.0.4.0/24", "10.2.1.128", "10.0.4.128", ""),
		Entry("Mapping 10.2.1.1 to 10.0.4.0/24", "10.0.4.0/24", "10.2.1.1", "10.0.4.1", ""),
		Entry("Mapping 10.2.127.128 to 10.0.128.0/23", "10.0.128.0/23", "10.2.127.128", "10.0.129.128", ""),
		Entry("Mapping 10.2.128.128 to 10.0.126.0/23", "10.0.127.0/23", "10.2.128.128", "10.0.127.128", ""),
		Entry("Mapping 10.2.128.128 to 10.0.126.0/25", "10.0.126.0/25", "10.2.128.128", "10.0.126.0", ""),
		Entry("Using an invalid newPodCidr", "10.0..0/25", "10.2.128.128", "", "invalid CIDR address: 10.0..0/25"),
		Entry("Using an invalid oldIp", "10.0.0.0/25", "10.2...128", "", "cannot parse oldIP"),
	)

	DescribeTable("GetFirstIP",
		func(network, expectedIP string, expectedErr *net.ParseError) {
			ip, err := ipamutils.GetFirstIP(network)
			if expectedErr != nil {
				Expect(err).To(MatchError(expectedErr))
			} else {
				Expect(err).ToNot(HaveOccurred())
			}
			Expect(ip).To(Equal(expectedIP))
		},
		Entry("Passing an invalid network", invalidValue, "", &net.ParseError{Type: CIDRAddressNetErr, Text: invalidValue}),
		Entry("Passing an empty network", "", "", &net.ParseError{Type: CIDRAddressNetErr, Text: ""}),
		Entry("Passing an IP", "10.0.0.0", "", &net.ParseError{Type: CIDRAddressNetErr, Text: "10.0.0.0"}),
		Entry("Getting first IP of 10.0.0.0/8", "10.0.0.0/8", "10.0.0.0", nil),
		Entry("Getting first IP of 192.168.0.0/16", "192.168.0.0/16", "192.168.0.0", nil),
	)
})
