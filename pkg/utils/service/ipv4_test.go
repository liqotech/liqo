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

package service_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	"github.com/liqotech/liqo/pkg/utils/service"
)

var _ = Describe("IPv4ClusterIP", func() {
	DescribeTable("selects the IPv4 ClusterIP of a service",
		func(clusterIP string, clusterIPs []string, expected string, expectedErr error) {
			svc := &corev1.Service{Spec: corev1.ServiceSpec{ClusterIP: clusterIP, ClusterIPs: clusterIPs}}
			ip, err := service.IPv4ClusterIP(svc)
			if expectedErr != nil {
				Expect(err).To(MatchError(expectedErr))
				Expect(ip).To(BeEmpty())
				return
			}
			Expect(err).ToNot(HaveOccurred())
			Expect(ip).To(Equal(expected))
		},
		Entry("IPv4 single-stack", "10.96.0.1", []string{"10.96.0.1"}, "10.96.0.1", nil),
		Entry("IPv4 single-stack, only clusterIP set", "10.96.0.1", nil, "10.96.0.1", nil),
		Entry("IPv4-primary dual-stack", "10.96.0.1", []string{"10.96.0.1", "fd00:10:96::1"}, "10.96.0.1", nil),
		Entry("IPv6-primary dual-stack", "fd00:10:96::1", []string{"fd00:10:96::1", "10.96.0.1"}, "10.96.0.1", nil),
		Entry("IPv6 single-stack", "fd00:10:96::1", []string{"fd00:10:96::1"}, "", service.ErrNoIPv4ClusterIP),
		Entry("IPv6 single-stack, only clusterIP set", "fd00:10:96::1", nil, "", service.ErrNoIPv4ClusterIP),
		Entry("headless", "None", []string{"None"}, "", service.ErrNoIPv4ClusterIP),
		Entry("no cluster IP", "", nil, "", service.ErrNoIPv4ClusterIP),
	)
})
