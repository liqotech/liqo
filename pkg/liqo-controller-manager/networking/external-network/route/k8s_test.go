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

package route

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/gateway"
)

type getPortsTestCase struct {
	ports []int32
	port  int32
	want  []int32
}

var _ = Describe("getPorts", func() {
	DescribeTable("should select the ports", func(tc getPortsTestCase) {
		// Equal uses reflect.DeepEqual, so nil and an empty slice are distinguished.
		Expect(getPorts(tc.ports, tc.port)).To(Equal(tc.want))
	},
		Entry("Ports only, order is preserved", getPortsTestCase{
			ports: []int32{51840, 51821, 51830},
			want:  []int32{51840, 51821, 51830},
		}),
		Entry("Ports with a single element", getPortsTestCase{
			ports: []int32{51840},
			want:  []int32{51840},
		}),
		Entry("Ports takes precedence over the legacy port", getPortsTestCase{
			ports: []int32{51840, 51841},
			port:  9999,
			want:  []int32{51840, 51841},
		}),
		Entry("Legacy port only", getPortsTestCase{
			port: 51840,
			want: []int32{51840},
		}),
		Entry("Empty (non-nil) ports falls back to the legacy port", getPortsTestCase{
			ports: []int32{},
			port:  51840,
			want:  []int32{51840},
		}),
		Entry("Nothing set returns nil", getPortsTestCase{
			want: nil,
		}),
		Entry("Empty (non-nil) ports and no legacy port returns nil", getPortsTestCase{
			ports: []int32{},
			want:  nil,
		}),
	)
})

var _ = Describe("forgeMutateRouteConfiguration", func() {
	const (
		namespace       = "liqo-tenant-test"
		cfgName         = "test-cfg"
		remoteClusterID = liqov1beta1.ClusterID("remote-cluster")
	)

	var (
		scheme   *runtime.Scheme
		cfg      *networkingv1beta1.Configuration
		routecfg *networkingv1beta1.RouteConfiguration
	)

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(networkingv1beta1.AddToScheme(scheme)).To(Succeed())

		cfg = &networkingv1beta1.Configuration{
			ObjectMeta: metav1.ObjectMeta{Name: cfgName, Namespace: namespace, UID: "cfg-uid"},
		}
		cfg.Spec.Remote.CIDR.Pod = []networkingv1beta1.CIDR{"10.1.0.0/24"}
		cfg.Spec.Remote.CIDR.External = []networkingv1beta1.CIDR{"10.2.0.0/24"}

		routecfg = &networkingv1beta1.RouteConfiguration{
			ObjectMeta: metav1.ObjectMeta{Name: "test-route", Namespace: namespace},
		}
	})

	mutate := func(ips, names []string) {
		Expect(forgeMutateRouteConfiguration(cfg, routecfg, scheme, remoteClusterID, ips, names)()).To(Succeed())
	}

	rules := func() []networkingv1beta1.Rule { return routecfg.Spec.Table.Rules }

	It("should set metadata, table name, owner and labels", func() {
		mutate([]string{"169.254.18.2"}, []string{"liqo-tunnel"})

		Expect(routecfg.Spec.Table.Name).To(Equal(cfgName))
		Expect(routecfg.Labels).To(Equal(gateway.ForgeRouteExternalTargetLabels(string(remoteClusterID))))
		Expect(routecfg.OwnerReferences).To(HaveLen(1))
		Expect(routecfg.OwnerReferences[0].Name).To(Equal(cfgName))
	})

	It("should forge one rule per remote CIDR, pod CIDRs first, with the fwmark", func() {
		mutate([]string{"169.254.18.2"}, []string{"liqo-tunnel"})

		Expect(rules()).To(HaveLen(2))
		Expect(*rules()[0].Dst).To(Equal(networkingv1beta1.CIDR("10.1.0.0/24")))
		Expect(*rules()[1].Dst).To(Equal(networkingv1beta1.CIDR("10.2.0.0/24")))
		for _, r := range rules() {
			Expect(*r.FwMark).To(Equal(gwExtMark))
			Expect(r.Routes).To(HaveLen(1))
			Expect(*r.Routes[0].Dst).To(Equal(*r.Dst))
		}
	})

	It("should forge no rules without remote CIDRs", func() {
		cfg.Spec.Remote.CIDR.Pod = nil
		cfg.Spec.Remote.CIDR.External = nil
		mutate([]string{"169.254.18.2"}, []string{"liqo-tunnel"})

		Expect(rules()).To(BeEmpty())
	})

	It("should use a plain gateway with a single interface", func() {
		mutate([]string{"169.254.18.2"}, []string{"liqo-tunnel"})

		for _, r := range rules() {
			route := r.Routes[0]
			Expect(route.Gw).NotTo(BeNil())
			Expect(string(*route.Gw)).To(Equal("169.254.18.2"))
			Expect(route.NextHops).To(BeEmpty())
		}
	})

	It("should use next-hops with multiple interfaces", func() {
		mutate(
			[]string{"169.254.18.2", "169.254.18.6", "169.254.18.10"},
			[]string{"liqo-tunnel", "liqo-tunnel1", "liqo-tunnel2"},
		)

		for _, r := range rules() {
			route := r.Routes[0]
			Expect(route.Gw).To(BeNil())
			Expect(route.NextHops).To(HaveLen(3))

			for i, want := range []struct{ gw, dev string }{
				{"169.254.18.2", "liqo-tunnel"},
				{"169.254.18.6", "liqo-tunnel1"},
				{"169.254.18.10", "liqo-tunnel2"},
			} {
				Expect(string(route.NextHops[i].Gw)).To(Equal(want.gw))
				Expect(route.NextHops[i].Dev).To(Equal(want.dev))
				Expect(route.NextHops[i].Weight).NotTo(BeNil())
				Expect(*route.NextHops[i].Weight).To(BeZero())
			}
		}
	})

	DescribeTable("should never set both gw and nextHops (CEL invariant)", func(ips, names []string) {
		mutate(ips, names)

		for _, r := range rules() {
			route := r.Routes[0]
			Expect(route.Gw != nil && len(route.NextHops) > 0).To(BeFalse())
		}
	},
		Entry("one interface", []string{"169.254.18.2"}, []string{"liqo-tunnel"}),
		Entry("two interfaces", []string{"169.254.18.2", "169.254.18.6"}, []string{"liqo-tunnel", "liqo-tunnel1"}),
	)

	It("should be idempotent", func() {
		ips := []string{"169.254.18.2", "169.254.18.6"}
		names := []string{"liqo-tunnel", "liqo-tunnel1"}

		mutate(ips, names)
		first := routecfg.Spec.DeepCopy()
		mutate(ips, names)

		Expect(rules()).To(HaveLen(2), "rules must not be duplicated")
		Expect(routecfg.Spec).To(Equal(*first))
	})

	It("should replace the previous rules when the number of interfaces changes", func() {
		mutate([]string{"169.254.18.2", "169.254.18.6"}, []string{"liqo-tunnel", "liqo-tunnel1"})
		mutate([]string{"169.254.18.2"}, []string{"liqo-tunnel"})

		for _, r := range rules() {
			Expect(r.Routes[0].NextHops).To(BeEmpty())
			Expect(r.Routes[0].Gw).NotTo(BeNil())
		}
	})
})

func TestRoute(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "External network route test suite")
}
