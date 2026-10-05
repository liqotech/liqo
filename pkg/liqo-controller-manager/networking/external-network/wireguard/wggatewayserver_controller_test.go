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

package wireguard

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
)

// servicePorts forges a list of service ports with the given port numbers, all UDP.
func servicePorts(ports ...int32) []corev1.ServicePort {
	res := make([]corev1.ServicePort, len(ports))
	for i, p := range ports {
		res[i] = corev1.ServicePort{Port: p, Protocol: corev1.ProtocolUDP}
	}
	return res
}

// sequentialPorts returns n consecutive port numbers starting from 51840.
func sequentialPorts(n int) []int32 {
	res := make([]int32, n)
	for i := range res {
		res[i] = int32(51840 + i)
	}
	return res
}

type forgeFunc func(svc *corev1.Service) (*networkingv1beta1.EndpointStatus, error)

var _ = Describe("forgeEndpointStatus", func() {
	r := &WgGatewayServerReconciler{}

	// The ClusterIP and LoadBalancer variants share the same port logic.
	variants := []struct {
		name  string
		forge forgeFunc
	}{
		{"ClusterIP", r.forgeEndpointStatusClusterIP},
		{"LoadBalancer", r.forgeEndpointStatusLoadBalancer},
	}

	for _, v := range variants {
		forge := v.forge

		Describe(v.name, func() {
			It("should keep the legacy behavior with a single port", func() {
				res, err := forge(&corev1.Service{Spec: corev1.ServiceSpec{Ports: servicePorts(51840)}})

				Expect(err).NotTo(HaveOccurred())
				Expect(res.Port).To(Equal(int32(51840))) //nolint:staticcheck // Port is intentionally used for backward compatibility.
				Expect(res.Ports).To(BeEmpty())
			})

			It("should expose all the ports, in order, with multiple ports", func() {
				res, err := forge(&corev1.Service{Spec: corev1.ServiceSpec{Ports: servicePorts(51840, 51821, 51830)}})

				Expect(err).NotTo(HaveOccurred())
				Expect(res.Ports).To(Equal([]int32{51840, 51821, 51830}))
				Expect(res.Port).To(BeZero()) //nolint:staticcheck // Port is intentionally used for backward compatibility.
			})

			DescribeTable("should never set both port and ports (CEL invariant)", func(n int) {
				res, err := forge(&corev1.Service{Spec: corev1.ServiceSpec{Ports: servicePorts(sequentialPorts(n)...)}})

				Expect(err).NotTo(HaveOccurred())
				Expect(res.Port != 0 && len(res.Ports) > 0).To(BeFalse()) //nolint:staticcheck // Port is intentionally used for backward compatibility.
			},
				Entry("1 port", 1),
				Entry("2 ports", 2),
				Entry("64 ports", 64),
			)

			It("should take the protocol from the first port", func() {
				res, err := forge(&corev1.Service{Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{
					{Port: 51840, Protocol: corev1.ProtocolUDP},
					{Port: 51841, Protocol: corev1.ProtocolTCP},
				}}})

				Expect(err).NotTo(HaveOccurred())
				Expect(res.Protocol).NotTo(BeNil())
				Expect(*res.Protocol).To(Equal(corev1.ProtocolUDP))
			})

			It("should fail when the service has no ports", func() {
				res, err := forge(&corev1.Service{Spec: corev1.ServiceSpec{}})

				Expect(err).To(HaveOccurred())
				Expect(res).To(BeNil())
			})
		})
	}

	It("ClusterIP should report the cluster IPs as addresses", func() {
		res, err := r.forgeEndpointStatusClusterIP(&corev1.Service{Spec: corev1.ServiceSpec{
			Ports:      servicePorts(51840, 51841),
			ClusterIPs: []string{"10.96.0.10"},
		}})

		Expect(err).NotTo(HaveOccurred())
		Expect(res.Addresses).To(Equal([]string{"10.96.0.10"}))
	})

	It("LoadBalancer should report the ingress addresses", func() {
		res, err := r.forgeEndpointStatusLoadBalancer(&corev1.Service{
			Spec: corev1.ServiceSpec{Ports: servicePorts(51840, 51841)},
			Status: corev1.ServiceStatus{LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{{IP: "203.0.113.5"}},
			}},
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(res.Addresses).To(ConsistOf("203.0.113.5"))
	})
})

func TestWireguard(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "External network wireguard test suite")
}
