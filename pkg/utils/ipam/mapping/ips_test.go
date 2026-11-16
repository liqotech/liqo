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

package mapping

import (
	"context"
	"fmt"
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	cidrutils "github.com/liqotech/liqo/pkg/utils/cidr"
)

const liqoNamespace = "liqo"

var _ = Describe("API server IP remapping", func() {
	var scheme *runtime.Scheme

	forgeService := func(name, namespace string, labels map[string]string, clusterIP string, clusterIPs ...string) *corev1.Service {
		return &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: labels},
			Spec:       corev1.ServiceSpec{ClusterIP: clusterIP, ClusterIPs: clusterIPs},
		}
	}

	getIP := func(cl client.Client, name string) string {
		var ip ipamv1alpha1.IP
		Expect(cl.Get(context.Background(), client.ObjectKey{Name: name, Namespace: liqoNamespace}, &ip)).To(Succeed())
		return string(ip.Spec.IP)
	}

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
		Expect(ipamv1alpha1.AddToScheme(scheme)).To(Succeed())
	})

	DescribeTable("EnforceAPIServerIPRemapping",
		func(svc *corev1.Service, expectedIP string, expectNoIPv4 bool) {
			cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(svc).Build()
			err := EnforceAPIServerIPRemapping(context.Background(), cl, liqoNamespace)
			if expectNoIPv4 {
				Expect(err).To(MatchError(ErrAPIServerNoIPv4ClusterIP))
				Expect(err.Error()).To(ContainSubstring("apiServerAccessThroughEndpointSlices"))
				return
			}
			Expect(err).ToNot(HaveOccurred())
			Expect(getIP(cl, consts.IPTypeAPIServer)).To(Equal(expectedIP))
		},
		Entry("IPv4 single-stack", forgeService("kubernetes", "default", nil, "10.96.0.1", "10.96.0.1"), "10.96.0.1", false),
		Entry("IPv4-primary dual-stack", forgeService("kubernetes", "default", nil, "10.96.0.1", "10.96.0.1", "fd00:10:96::1"),
			"10.96.0.1", false),
		Entry("IPv6-primary dual-stack with IPv4 ClusterIP", forgeService("kubernetes", "default", nil, "fd00:10:96::1",
			"fd00:10:96::1", "10.96.0.1"), "10.96.0.1", false),
		Entry("IPv6 single-stack kubernetes service", forgeService("kubernetes", "default", nil, "fd00:10:96::1", "fd00:10:96::1"),
			"", true),
	)

	DescribeTable("EnforceAPIServerProxyIPRemapping",
		func(svc *corev1.Service, expectedIP string, expectedErr error) {
			cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(svc).Build()
			err := EnforceAPIServerProxyIPRemapping(context.Background(), cl, liqoNamespace)
			if expectedErr != nil {
				Expect(err).To(MatchError(expectedErr))
				return
			}
			Expect(err).ToNot(HaveOccurred())
			Expect(getIP(cl, consts.IPTypeAPIServerProxy)).To(Equal(expectedIP))
		},
		Entry("IPv4 single-stack", forgeService("proxy", liqoNamespace, map[string]string{consts.K8sAppNameKey: "proxy"},
			"10.96.0.5", "10.96.0.5"), "10.96.0.5", nil),
		Entry("IPv6-primary dual-stack with IPv4 ClusterIP", forgeService("proxy", liqoNamespace,
			map[string]string{consts.K8sAppNameKey: "proxy"}, "fd00:10:96::5", "fd00:10:96::5", "10.96.0.5"), "10.96.0.5", nil),
		Entry("IPv6 single-stack", forgeService("proxy", liqoNamespace, map[string]string{consts.K8sAppNameKey: "proxy"},
			"fd00:10:96::5", "fd00:10:96::5"), "", ErrAPIServerNoIPv4ClusterIP),
	)
})

var _ = Describe("MapAddressWithConfiguration", func() {
	var cfg *networkingv1beta1.Configuration

	BeforeEach(func() {
		cfg = &networkingv1beta1.Configuration{
			ObjectMeta: metav1.ObjectMeta{Generation: 1},
			Spec: networkingv1beta1.ConfigurationSpec{
				Remote: networkingv1beta1.ClusterConfig{
					CIDR: networkingv1beta1.ClusterConfigCIDR{
						Pod:      cidrutils.FromStrings([]string{"10.10.0.0/16", "10.20.0.0/16"}),
						External: cidrutils.FromStrings([]string{"192.168.0.0/16", "172.16.0.0/16"}),
					},
				},
			},
			Status: networkingv1beta1.ConfigurationStatus{
				Conditions: []metav1.Condition{{
					Type:               networkingv1beta1.ConfigurationConditionNetworkCIDRsConfigured,
					Status:             metav1.ConditionTrue,
					Reason:             "NetworkCIDRsConfigured",
					Message:            "All network CIDRs are configured",
					ObservedGeneration: 1,
					LastTransitionTime: metav1.Now(),
				}},
				Remote: &networkingv1beta1.ClusterConfig{
					CIDR: networkingv1beta1.ClusterConfigCIDR{
						Pod:      cidrutils.FromStrings([]string{"10.30.0.0/16", "10.40.0.0/16"}),
						External: cidrutils.FromStrings([]string{"192.168.0.0/16", "172.18.0.0/16"}),
					},
				},
			},
		}
	})

	It("remaps addresses from the second pod CIDR using the aligned status entry", func() {
		mapped, err := MapAddressWithConfiguration(cfg, "10.20.5.10")
		Expect(err).ToNot(HaveOccurred())
		Expect(mapped).To(Equal("10.40.5.10"))
	})

	It("remaps addresses from the second external CIDR using the aligned status entry", func() {
		mapped, err := MapAddressWithConfiguration(cfg, "172.16.5.10")
		Expect(err).ToNot(HaveOccurred())
		Expect(mapped).To(Equal("172.18.5.10"))
	})

	It("leaves addresses unchanged when the aligned status CIDR matches the spec CIDR", func() {
		mapped, err := MapAddressWithConfiguration(cfg, "192.168.5.10")
		Expect(err).ToNot(HaveOccurred())
		Expect(mapped).To(Equal("192.168.5.10"))
	})

	It("leaves addresses outside the configured CIDRs unchanged", func() {
		mapped, err := MapAddressWithConfiguration(cfg, "8.8.8.8")
		Expect(err).ToNot(HaveOccurred())
		Expect(mapped).To(Equal("8.8.8.8"))
	})

	It("errors when the input address is invalid", func() {
		_, err := MapAddressWithConfiguration(cfg, "not-an-ip")
		Expect(err).To(HaveOccurred())
	})

	It("errors when the configuration is not marked as remapped", func() {
		cfg.Status.Remote.CIDR.Pod = cidrutils.FromStrings([]string{"10.30.0.0/16"})
		cfg.Status.Conditions[0].Status = metav1.ConditionFalse
		cfg.Status.Conditions[0].Reason = "WaitingForNetworks"
		cfg.Status.Conditions[0].Message = "Waiting for all networks to be ready"

		_, err := MapAddressWithConfiguration(cfg, "10.10.5.10")
		Expect(err).To(MatchError("configuration not remapped yet"))
	})
})

var _ = Describe("RemapMask", func() {
	DescribeTable("IPv4 remapping",
		func(ipStr, cidrStr string, expectedStr string) {
			addr := net.ParseIP(ipStr)
			Expect(addr).NotTo(BeNil())

			_, mask, err := net.ParseCIDR(cidrStr)
			Expect(err).NotTo(HaveOccurred()) // Check no error occurs
			Expect(mask).NotTo(BeNil())

			expected := net.ParseIP(expectedStr).To4()
			Expect(expected).NotTo(BeNil())

			result := RemapMask(addr, *mask)
			Expect(result).To(Equal(expected))
		},
		Entry("IPv4 remapping", "192.168.1.20", "10.0.0.0/27", "10.0.0.20"),
		Entry("IPv4 remapping", "192.168.2.20", "10.0.0.0/27", "10.0.0.20"),
		Entry("IPv4 remapping", "192.168.1.50", "10.0.0.0/27", "10.0.0.18"),
		Entry("IPv4 remapping", "192.168.2.50", "10.0.0.0/27", "10.0.0.18"),
		Entry("IPv4 remapping", "192.168.1.1", "255.255.255.128/25", "255.255.255.129"),
		Entry("IPv4 remapping", "172.16.0.1", "255.255.255.128/25", "255.255.255.129"),
		Entry("IPv4 remapping", "192.168.1.200", "255.255.255.128/25", "255.255.255.200"),
		Entry("IPv4 remapping", "192.168.1.10", "255.255.255.0/24", "255.255.255.10"),
		Entry("IPv4 remapping", "192.168.1.100", "255.255.255.0/24", "255.255.255.100"),
		Entry("IPv4 remapping", "172.16.0.1", "255.255.240.0/20", "255.255.240.1"),
		Entry("IPv4 remapping", "172.16.1.1", "255.255.240.0/20", "255.255.241.1"),
		Entry("IPv4 remapping", "192.168.1.10", "255.4.224.0/19", "255.4.225.10"),
		Entry("IPv4 remapping", "192.168.2.10", "255.4.224.0/19", "255.4.226.10"),
		Entry("IPv4 remapping", "10.0.0.1", "255.255.192.0/18", "255.255.192.1"),
		Entry("IPv4 remapping", "10.255.1.1", "78.5.78.143/18", "78.5.65.1"),
		Entry("IPv4 remapping", "192.168.1.20", "192.168.0.0/16", "192.168.1.20"),
		Entry("IPv4 remapping", "192.168.2.20", "192.168.0.0/16", "192.168.2.20"),
		Entry("IPv4 remapping", "172.16.1.1", "172.16.0.0/12", "172.16.1.1"),
		Entry("IPv4 remapping", "172.16.2.1", "172.16.0.0/12", "172.16.2.1"),
		Entry("IPv4 remapping", "10.0.0.1", "255.192.0.0/10", "255.192.0.1"),
		Entry("IPv4 remapping", "10.1.0.1", "40.32.0.0/10", "40.1.0.1"),
		Entry("IPv4 remapping", "10.0.0.1", "255.0.0.0/8", "255.0.0.1"),
		Entry("IPv4 remapping", "10.0.0.50", "255.0.0.0/8", "255.0.0.50"),
		Entry("IPv4 remapping", "10.1.1.1", "20.0.0.0/8", "20.1.1.1"),
		Entry("IPv4 remapping", "10.2.1.1", "20.0.0.0/8", "20.2.1.1"),
		Entry("IPv4 remapping", "192.168.1.10", "240.0.0.0/4", "240.168.1.10"),
		Entry("IPv4 remapping", "192.168.2.10", "240.0.0.0/4", "240.168.2.10"),
		Entry("IPv4 remapping", "192.168.1.100", "0.0.0.0/0", "192.168.1.100"),
		Entry("IPv4 remapping", "10.0.0.1", "255.255.255.255/32", "255.255.255.255"),
		Entry("IPv4 remapping", "172.16.1.0", "172.16.0.0/16", "172.16.1.0"),
		Entry("IPv4 remapping", "172.16.1.255", "172.16.0.0/16", "172.16.1.255"),
		Entry("IPv4 remapping", "10.10.10.10", "192.168.1.0/24", "192.168.1.10"),
		Entry("IPv4 remapping", "192.168.1.255", "10.0.0.0/8", "10.168.1.255"),
		Entry("IPv4 remapping", "224.0.0.1", "192.168.1.0/24", "192.168.1.1"),
		Entry("IPv4 remapping", "127.0.0.1", "10.0.0.0/8", "10.0.0.1"),
		Entry("IPv4 remapping", "192.168.1.1", "10.0.0.0/30", "10.0.0.1"),
		Entry("IPv4 remapping", "192.168.1.2", "10.0.0.0/30", "10.0.0.2"),
		Entry("IPv4 remapping", "192.168.1.10", "10.0.0.0/29", "10.0.0.2"),
		Entry("IPv4 remapping", "172.16.5.10", "192.168.0.0/21", "192.168.5.10"),
		Entry("IPv4 remapping", "10.0.5.20", "172.16.0.0/13", "172.16.5.20"),
		Entry("IPv4 remapping", "192.168.1.100", "10.0.0.0/27", "10.0.0.4"),
		Entry("IPv4 remapping", "172.20.100.5", "192.168.0.0/17", "192.168.100.5"),
		Entry("IPv4 remapping", "192.168.3.50", "10.0.0.0/23", "10.0.1.50"),
		Entry("IPv4 remapping", "10.1.1.2", "192.168.1.0/30", "192.168.1.2"),
		Entry("IPv4 remapping", "192.168.5.33", "172.16.1.0/26", "172.16.1.33"),
	)

	DescribeTable("IPv6 address remapping",
		func(ipStr, cidrStr string, expectedStr string) {
			addr := net.ParseIP(ipStr)
			Expect(addr).NotTo(BeNil())

			_, mask, err := net.ParseCIDR(cidrStr)
			Expect(err).NotTo(HaveOccurred())
			Expect(mask).NotTo(BeNil())

			expected := net.ParseIP(expectedStr).To16()
			Expect(expected).NotTo(BeNil())

			result := RemapMask(addr, *mask)

			fmt.Printf("expected: %s\n", expected)
			fmt.Printf("result: %s\n", result)

			Expect(result).To(Equal(expected))
		},
		Entry("IPv6 remapping", "2001:db8::1", "2001:db8::/32", "2001:db8::1"),
		Entry("IPv6 remapping", "2001:db8:1::1", "2001:db8::/32", "2001:db8:1::1"),
		Entry("IPv6 remapping", "2001:db8::1", "2001:db8:1::/48", "2001:db8:1::1"),
		Entry("IPv6 remapping", "2001:db8:2::1", "2001:db8:1::/48", "2001:db8:1::1"),
		Entry("IPv6 remapping", "2001:db8::1", "2001:db8:1:2::/64", "2001:db8:1:2::1"),
		Entry("IPv6 remapping", "2001:db8:1::1", "2001:db8:1:2::/64", "2001:db8:1:2::1"),
		Entry("IPv6 remapping", "2001:db8::1", "2001:db8:1:2:3::/80", "2001:db8:1:2:3::1"),
		Entry("IPv6 remapping", "2001:db8:1::1", "2001:db8:1:2:3::/80", "2001:db8:1:2:3::1"),
		Entry("IPv6 remapping", "2001:db8::1", "2001:db8:1:2:3:4::/96", "2001:db8:1:2:3:4::1"),
		Entry("IPv6 remapping", "2001:db8:1::1", "2001:db8:1:2:3:4::/96", "2001:db8:1:2:3:4::1"),
		Entry("IPv6 remapping", "2001:db8::1", "2001:db8:1:2:3:4:5::/112", "2001:db8:1:2:3:4:5:1"),
		Entry("IPv6 remapping", "2001:db8:1::1", "2001:db8:1:2:3:4:5::/112", "2001:db8:1:2:3:4:5:1"),
		Entry("IPv6 remapping", "2001:db8:1::1", "2001:db8:1:2:3:4:5:6/128", "2001:db8:1:2:3:4:5:6"),
		Entry("IPv6 remapping", "2001:db8::1", "2001:db8::/16", "2001:db8::1"),
		Entry("IPv6 remapping", "2001:db8:abcd::1", "2001:db8::/56", "2001:db8::1"),
		Entry("IPv6 remapping", "2001:db8:abcd:1234::1", "2001:db8::/64", "2001:db8::1"),
		Entry("IPv6 remapping", "fc00::1", "fd00::/8", "fd00::1"),
		Entry("IPv6 remapping", "fd00:1234::1", "fc00::/7", "fd00:1234::1"),
		Entry("IPv6 remapping", "fe80::1", "fd00::/8", "fd80::1"),
		Entry("IPv6 remapping", "2001:db8:abcd::1", "2001:db8:1234::/32", "2001:db8:abcd::1"),
		Entry("IPv6 remapping", "2001::1", "2002::/16", "2002::1"),
		Entry("IPv6 remapping", "2001:db8:abcd::1", "2001:db8::/49", "2001:db8::1"),
		Entry("IPv6 remapping", "2001:db8:abcd:1234::1", "2001:db8::/57", "2001:db8:0:34::1"),
		Entry("IPv6 remapping", "2001:db8:abcd:1234::1", "2001:db8::/69", "2001:db8::1"),
		Entry("IPv6 remapping", "2001:db8:abcd:1234::1", "2001:db8::/73", "2001:db8::1"),
		Entry("IPv6 remapping", "2001:db8::1", "2001:db8::/127", "2001:db8::1"),
		Entry("IPv6 remapping", "2001:db8:abcd::1", "2001:db8::/37", "2001:db8:3cd::1"),
		Entry("IPv6 remapping", "2001:fdb8:abcd::45a3:1", "2001:d2f::/53", "2001:d2f::45a3:1"),
		Entry("IPv6 remapping", "2001:db8:abcd:1234::1", "2001:db8::/61", "2001:db8:0:4::1"),
	)
})
