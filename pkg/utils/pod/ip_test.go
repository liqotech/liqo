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

package pod_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	"github.com/liqotech/liqo/pkg/utils/pod"
)

var _ = Describe("Pod IP utility functions", func() {
	podIPs := func(ips ...string) []corev1.PodIP {
		out := make([]corev1.PodIP, 0, len(ips))
		for _, ip := range ips {
			out = append(out, corev1.PodIP{IP: ip})
		}
		return out
	}

	type IPv4Case struct {
		Status      corev1.PodStatus
		Expected    string
		ExpectedErr error
	}

	DescribeTable("The IPv4FromStatus function",
		func(c IPv4Case) {
			ip, err := pod.IPv4FromStatus(&c.Status)
			if c.ExpectedErr != nil {
				Expect(err).To(MatchError(c.ExpectedErr))
				Expect(ip).To(BeEmpty())
				return
			}
			Expect(err).ToNot(HaveOccurred())
			Expect(ip).To(Equal(c.Expected))
		},
		Entry("IPv4 single-stack", IPv4Case{
			Status:   corev1.PodStatus{PodIP: "10.244.0.5", PodIPs: podIPs("10.244.0.5")},
			Expected: "10.244.0.5",
		}),
		Entry("IPv4-primary dual-stack", IPv4Case{
			Status:   corev1.PodStatus{PodIP: "10.244.0.5", PodIPs: podIPs("10.244.0.5", "fd00:10:244::5")},
			Expected: "10.244.0.5",
		}),
		Entry("IPv6-primary dual-stack", IPv4Case{
			Status:   corev1.PodStatus{PodIP: "fd00:10:244::5", PodIPs: podIPs("fd00:10:244::5", "10.244.0.5")},
			Expected: "10.244.0.5",
		}),
		Entry("only the singular PodIP field set, IPv4", IPv4Case{
			Status:   corev1.PodStatus{PodIP: "10.244.0.5"},
			Expected: "10.244.0.5",
		}),
		Entry("IPv6 single-stack", IPv4Case{
			Status:      corev1.PodStatus{PodIP: "fd00:10:244::5", PodIPs: podIPs("fd00:10:244::5")},
			ExpectedErr: pod.ErrNoIPv4Address,
		}),
		Entry("only the singular PodIP field set, IPv6", IPv4Case{
			Status:      corev1.PodStatus{PodIP: "fd00:10:244::5"},
			ExpectedErr: pod.ErrNoIPv4Address,
		}),
		Entry("no IP assigned yet", IPv4Case{
			Status:      corev1.PodStatus{},
			ExpectedErr: pod.ErrNoIPAddress,
		}),
		Entry("unparsable entries are skipped", IPv4Case{
			Status:   corev1.PodStatus{PodIP: "not-an-ip", PodIPs: podIPs("not-an-ip", "10.244.0.5")},
			Expected: "10.244.0.5",
		}),
	)

	It("The IPv4 function reads the pod status", func() {
		p := &corev1.Pod{Status: corev1.PodStatus{PodIP: "fd00:10:244::5", PodIPs: podIPs("fd00:10:244::5", "10.244.0.5")}}
		ip, err := pod.IPv4(p)
		Expect(err).ToNot(HaveOccurred())
		Expect(ip).To(Equal("10.244.0.5"))
	})
})
