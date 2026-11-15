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
	"errors"
	"net"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/vishvananda/netlink"
	"k8s.io/utils/ptr"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
)

// nh forges a NexthopInfo. An empty gw leaves the gateway nil.
func nh(gw string, linkIndex, hops int) *netlink.NexthopInfo {
	var ip net.IP
	if gw != "" {
		ip = net.ParseIP(gw)
	}
	return &netlink.NexthopInfo{Gw: ip, LinkIndex: linkIndex, Hops: hops}
}

type sortNextHopsTestCase struct {
	input []*netlink.NexthopInfo
	want  []*netlink.NexthopInfo
}

var _ = Describe("sortNextHops", func() {

	DescribeTable("Sorting behavior", func(tc sortNextHopsTestCase) {
		got := sortNextHops(tc.input)

		Expect(got).To(HaveLen(len(tc.want)))
		for i := range tc.want {
			Expect(got[i].Gw.String()).To(Equal(tc.want[i].Gw.String()), "Unexpected gateway at position %d", i)
			Expect(got[i].LinkIndex).To(Equal(tc.want[i].LinkIndex), "Unexpected link index at position %d", i)
			Expect(got[i].Hops).To(Equal(tc.want[i].Hops), "Unexpected hops at position %d", i)
		}
	},
		Entry("Nil input", sortNextHopsTestCase{
			input: nil,
			want:  nil,
		}),
		Entry("Empty input", sortNextHopsTestCase{
			input: []*netlink.NexthopInfo{},
			want:  []*netlink.NexthopInfo{},
		}),
		Entry("Single element", sortNextHopsTestCase{
			input: []*netlink.NexthopInfo{nh("10.0.0.1", 1, 0)},
			want:  []*netlink.NexthopInfo{nh("10.0.0.1", 1, 0)},
		}),
		Entry("Sorted by gateway", sortNextHopsTestCase{
			input: []*netlink.NexthopInfo{nh("10.0.0.3", 1, 0), nh("10.0.0.1", 1, 0), nh("10.0.0.2", 1, 0)},
			want:  []*netlink.NexthopInfo{nh("10.0.0.1", 1, 0), nh("10.0.0.2", 1, 0), nh("10.0.0.3", 1, 0)},
		}),
		Entry("Gateway is compared as a string, not numerically", sortNextHopsTestCase{
			input: []*netlink.NexthopInfo{nh("10.0.0.2", 1, 0), nh("10.0.0.10", 1, 0)},
			want:  []*netlink.NexthopInfo{nh("10.0.0.10", 1, 0), nh("10.0.0.2", 1, 0)},
		}),
		Entry("Same gateway, sorted by link index", sortNextHopsTestCase{
			input: []*netlink.NexthopInfo{nh("10.0.0.1", 3, 0), nh("10.0.0.1", 1, 0), nh("10.0.0.1", 2, 0)},
			want:  []*netlink.NexthopInfo{nh("10.0.0.1", 1, 0), nh("10.0.0.1", 2, 0), nh("10.0.0.1", 3, 0)},
		}),
		Entry("Same gateway and link index, sorted by hops", sortNextHopsTestCase{
			input: []*netlink.NexthopInfo{nh("10.0.0.1", 1, 5), nh("10.0.0.1", 1, 0), nh("10.0.0.1", 1, 2)},
			want:  []*netlink.NexthopInfo{nh("10.0.0.1", 1, 0), nh("10.0.0.1", 1, 2), nh("10.0.0.1", 1, 5)},
		}),
		Entry("Gateway has priority over link index", sortNextHopsTestCase{
			input: []*netlink.NexthopInfo{nh("10.0.0.2", 1, 0), nh("10.0.0.1", 9, 0)},
			want:  []*netlink.NexthopInfo{nh("10.0.0.1", 9, 0), nh("10.0.0.2", 1, 0)},
		}),
		Entry("Duplicates are preserved", sortNextHopsTestCase{
			input: []*netlink.NexthopInfo{nh("10.0.0.2", 1, 0), nh("10.0.0.1", 1, 0), nh("10.0.0.1", 1, 0)},
			want:  []*netlink.NexthopInfo{nh("10.0.0.1", 1, 0), nh("10.0.0.1", 1, 0), nh("10.0.0.2", 1, 0)},
		}),
	)

	It("should not modify the input slice", func() {
		a, b, c := nh("10.0.0.3", 1, 0), nh("10.0.0.1", 1, 0), nh("10.0.0.2", 1, 0)
		input := []*netlink.NexthopInfo{a, b, c}

		got := sortNextHops(input)

		// The original slice keeps its order (same pointers, same positions)...
		Expect(input[0]).To(BeIdenticalTo(a))
		Expect(input[1]).To(BeIdenticalTo(b))
		Expect(input[2]).To(BeIdenticalTo(c))
		// ...while the returned one is sorted.
		Expect(got[0]).To(BeIdenticalTo(b))
		Expect(got[1]).To(BeIdenticalTo(c))
		Expect(got[2]).To(BeIdenticalTo(a))
	})

	It("should not panic with a nil gateway", func() {
		input := []*netlink.NexthopInfo{nh("10.0.0.1", 1, 0), nh("", 2, 0)}

		Expect(func() { sortNextHops(input) }).NotTo(Panic())
		Expect(sortNextHops(input)).To(HaveLen(2))
	})
})

type isEqualRouteTestCase struct {
	route1 *netlink.Route
	route2 *netlink.Route
	want   bool
}

// mpRoute forges a multipath route with the given next-hops.
func mpRoute(nhs ...*netlink.NexthopInfo) *netlink.Route {
	return &netlink.Route{MultiPath: nhs}
}

var _ = Describe("IsEqualRoute", func() {

	DescribeTable("Comparison behavior", func(tc isEqualRouteTestCase) {
		Expect(IsEqualRoute(tc.route1, tc.route2)).To(Equal(tc.want))
		// The comparison must be symmetric.
		Expect(IsEqualRoute(tc.route2, tc.route1)).To(Equal(tc.want), "Comparison is not symmetric")
	},
		Entry("Single-path routes, equal", isEqualRouteTestCase{
			route1: &netlink.Route{Gw: net.ParseIP("10.0.0.1"), LinkIndex: 1},
			route2: &netlink.Route{Gw: net.ParseIP("10.0.0.1"), LinkIndex: 1},
			want:   true,
		}),
		Entry("Single-path routes, different gateway", isEqualRouteTestCase{
			route1: &netlink.Route{Gw: net.ParseIP("10.0.0.1"), LinkIndex: 1},
			route2: &netlink.Route{Gw: net.ParseIP("10.0.0.2"), LinkIndex: 1},
			want:   false,
		}),
		Entry("Multipath, same next-hops in the same order", isEqualRouteTestCase{
			route1: mpRoute(nh("10.0.0.1", 1, 0), nh("10.0.0.2", 2, 0)),
			route2: mpRoute(nh("10.0.0.1", 1, 0), nh("10.0.0.2", 2, 0)),
			want:   true,
		}),
		Entry("Multipath, same next-hops in a different order", isEqualRouteTestCase{
			route1: mpRoute(nh("10.0.0.1", 1, 0), nh("10.0.0.2", 2, 0), nh("10.0.0.3", 3, 0)),
			route2: mpRoute(nh("10.0.0.3", 3, 0), nh("10.0.0.1", 1, 0), nh("10.0.0.2", 2, 0)),
			want:   true,
		}),
		Entry("Multipath, different lengths", isEqualRouteTestCase{
			route1: mpRoute(nh("10.0.0.1", 1, 0), nh("10.0.0.2", 2, 0)),
			route2: mpRoute(nh("10.0.0.1", 1, 0)),
			want:   false,
		}),
		Entry("One multipath route and one without next-hops", isEqualRouteTestCase{
			route1: mpRoute(nh("10.0.0.1", 1, 0), nh("10.0.0.2", 2, 0)),
			route2: &netlink.Route{Gw: net.ParseIP("10.0.0.1"), LinkIndex: 1},
			want:   false,
		}),
		Entry("Multipath, only the gateway differs", isEqualRouteTestCase{
			route1: mpRoute(nh("10.0.0.1", 1, 0), nh("10.0.0.2", 2, 0)),
			route2: mpRoute(nh("10.0.0.1", 1, 0), nh("10.0.0.9", 2, 0)),
			want:   false,
		}),
		Entry("Multipath, only the link index differs", isEqualRouteTestCase{
			route1: mpRoute(nh("10.0.0.1", 1, 0), nh("10.0.0.2", 2, 0)),
			route2: mpRoute(nh("10.0.0.1", 1, 0), nh("10.0.0.2", 7, 0)),
			want:   false,
		}),
		Entry("Multipath, only the hops (weight) differ", isEqualRouteTestCase{
			route1: mpRoute(nh("10.0.0.1", 1, 0), nh("10.0.0.2", 2, 0)),
			route2: mpRoute(nh("10.0.0.1", 1, 0), nh("10.0.0.2", 2, 3)),
			want:   false,
		}),
		Entry("Multipath, duplicates with the same length but different content", isEqualRouteTestCase{
			route1: mpRoute(nh("10.0.0.1", 1, 0), nh("10.0.0.1", 1, 0), nh("10.0.0.2", 2, 0)),
			route2: mpRoute(nh("10.0.0.1", 1, 0), nh("10.0.0.2", 2, 0), nh("10.0.0.2", 2, 0)),
			want:   false,
		}),
		Entry("Same next-hops but different flags", isEqualRouteTestCase{
			route1: &netlink.Route{Flags: 0, MultiPath: []*netlink.NexthopInfo{nh("10.0.0.1", 1, 0)}},
			route2: &netlink.Route{Flags: 4, MultiPath: []*netlink.NexthopInfo{nh("10.0.0.1", 1, 0)}},
			want:   false,
		}),
		Entry("Same next-hops but different destination", isEqualRouteTestCase{
			route1: &netlink.Route{
				Dst:       &net.IPNet{IP: net.ParseIP("192.168.0.0"), Mask: net.CIDRMask(24, 32)},
				MultiPath: []*netlink.NexthopInfo{nh("10.0.0.1", 1, 0)},
			},
			route2: &netlink.Route{
				Dst:       &net.IPNet{IP: net.ParseIP("192.168.1.0"), Mask: net.CIDRMask(24, 32)},
				MultiPath: []*netlink.NexthopInfo{nh("10.0.0.1", 1, 0)},
			},
			want: false,
		}),
	)

	It("should not modify the next-hops of the compared routes", func() {
		r1 := mpRoute(nh("10.0.0.3", 3, 0), nh("10.0.0.1", 1, 0))
		r2 := mpRoute(nh("10.0.0.1", 1, 0), nh("10.0.0.3", 3, 0))
		first := r1.MultiPath[0]

		Expect(IsEqualRoute(r1, r2)).To(BeTrue())
		Expect(r1.MultiPath[0]).To(BeIdenticalTo(first))
	})
})

var _ = Describe("forgeNetlinkRoute", func() {
	const (
		tableID     = uint32(1234)
		missingLink = "liqo-nolink0"
	)

	var loIndex int

	BeforeEach(func() {
		lo, err := netlink.LinkByName("lo")
		Expect(err).NotTo(HaveOccurred())
		loIndex = lo.Attrs().Index
	})

	cidr := func(s string) *networkingv1beta1.CIDR { return ptr.To(networkingv1beta1.CIDR(s)) }
	ip := func(s string) networkingv1beta1.IP { return networkingv1beta1.IP(s) }

	Context("single-path routes", func() {
		It("should forge a route with only the gateway and no multipath", func() {
			r, err := forgeNetlinkRoute(&networkingv1beta1.Route{
				Dst: cidr("10.1.0.0/24"),
				Gw:  ptr.To(ip("10.0.0.1")),
				Dev: ptr.To("lo"),
			}, tableID)

			Expect(err).NotTo(HaveOccurred())
			Expect(r.MultiPath).To(BeEmpty())
			Expect(r.Dst.String()).To(Equal("10.1.0.0/24"))
			Expect(r.Gw.String()).To(Equal("10.0.0.1"))
			Expect(r.LinkIndex).To(Equal(loIndex))
			Expect(r.Table).To(Equal(int(tableID)))
		})

		It("should forge a route with only the destination", func() {
			r, err := forgeNetlinkRoute(&networkingv1beta1.Route{Dst: cidr("10.1.0.0/24")}, tableID)

			Expect(err).NotTo(HaveOccurred())
			Expect(r.Gw).To(BeNil())
			Expect(r.LinkIndex).To(BeZero())
			Expect(r.MultiPath).To(BeEmpty())
		})

		It("should set the onlink flag", func() {
			r, err := forgeNetlinkRoute(&networkingv1beta1.Route{
				Dst: cidr("10.1.0.0/24"), Onlink: ptr.To(true),
			}, tableID)

			Expect(err).NotTo(HaveOccurred())
			Expect(r.Flags).To(Equal(int(netlink.FLAG_ONLINK)))
		})

		DescribeTable("should translate the scope", func(in networkingv1beta1.Scope, want netlink.Scope) {
			r, err := forgeNetlinkRoute(&networkingv1beta1.Route{
				Dst: cidr("10.1.0.0/24"), Scope: ptr.To(in),
			}, tableID)

			Expect(err).NotTo(HaveOccurred())
			Expect(r.Scope).To(Equal(want))
		},
			Entry("global", networkingv1beta1.GlobalScope, netlink.SCOPE_UNIVERSE),
			Entry("link", networkingv1beta1.LinkScope, netlink.SCOPE_LINK),
			Entry("host", networkingv1beta1.HostScope, netlink.SCOPE_HOST),
			Entry("site", networkingv1beta1.SiteScope, netlink.SCOPE_SITE),
			Entry("nowhere", networkingv1beta1.NowhereScope, netlink.SCOPE_NOWHERE),
		)

		It("should fail with an invalid destination", func() {
			_, err := forgeNetlinkRoute(&networkingv1beta1.Route{Dst: cidr("not-a-cidr")}, tableID)
			Expect(err).To(HaveOccurred())
		})

		It("should fail with a non-existent device", func() {
			_, err := forgeNetlinkRoute(&networkingv1beta1.Route{
				Dst: cidr("10.1.0.0/24"), Dev: ptr.To(missingLink),
			}, tableID)

			Expect(err).To(HaveOccurred())
			Expect(errors.As(err, &netlink.LinkNotFoundError{})).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring(missingLink))
		})
	})

	Context("multipath routes", func() {
		It("should forge the multipath and clear the main gateway and link index", func() {
			r, err := forgeNetlinkRoute(&networkingv1beta1.Route{
				Dst: cidr("10.1.0.0/24"),
				NextHops: []networkingv1beta1.NextHop{
					{Gw: ip("10.0.0.1"), Dev: "lo", Weight: ptr.To(5)},
					{Gw: ip("10.0.0.2"), Dev: "lo"},
				},
			}, tableID)

			Expect(err).NotTo(HaveOccurred())
			Expect(r.Gw).To(BeNil())
			Expect(r.LinkIndex).To(BeZero())
			Expect(r.MultiPath).To(HaveLen(2))

			Expect(r.MultiPath[0].Gw.String()).To(Equal("10.0.0.1"))
			Expect(r.MultiPath[0].LinkIndex).To(Equal(loIndex))
			Expect(r.MultiPath[0].Hops).To(Equal(5))

			Expect(r.MultiPath[1].Gw.String()).To(Equal("10.0.0.2"))
			Expect(r.MultiPath[1].LinkIndex).To(Equal(loIndex))
			Expect(r.MultiPath[1].Hops).To(BeZero(), "nil weight must translate to 0 hops")
		})

		It("should preserve the order of the next-hops", func() {
			r, err := forgeNetlinkRoute(&networkingv1beta1.Route{
				Dst: cidr("10.1.0.0/24"),
				NextHops: []networkingv1beta1.NextHop{
					{Gw: ip("10.0.0.3"), Dev: "lo"},
					{Gw: ip("10.0.0.1"), Dev: "lo"},
					{Gw: ip("10.0.0.2"), Dev: "lo"},
				},
			}, tableID)

			Expect(err).NotTo(HaveOccurred())
			Expect(r.MultiPath).To(HaveLen(3))
			Expect(r.MultiPath[0].Gw.String()).To(Equal("10.0.0.3"))
			Expect(r.MultiPath[1].Gw.String()).To(Equal("10.0.0.1"))
			Expect(r.MultiPath[2].Gw.String()).To(Equal("10.0.0.2"))
		})

		It("should ignore the route-level device and gateway when next-hops are present", func() {
			// gw + nextHops is rejected by the CEL rule at admission time,
			// but dev + nextHops is allowed: the main device must not leak into the route.
			r, err := forgeNetlinkRoute(&networkingv1beta1.Route{
				Dst:      cidr("10.1.0.0/24"),
				Dev:      ptr.To("lo"),
				Gw:       ptr.To(ip("10.0.0.9")),
				NextHops: []networkingv1beta1.NextHop{{Gw: ip("10.0.0.1"), Dev: "lo"}},
			}, tableID)

			Expect(err).NotTo(HaveOccurred())
			Expect(r.Gw).To(BeNil())
			Expect(r.LinkIndex).To(BeZero())
			Expect(r.MultiPath).To(HaveLen(1))
		})

		It("should report the index of the next-hop with a non-existent device", func() {
			_, err := forgeNetlinkRoute(&networkingv1beta1.Route{
				Dst: cidr("10.1.0.0/24"),
				NextHops: []networkingv1beta1.NextHop{
					{Gw: ip("10.0.0.1"), Dev: "lo"},
					{Gw: ip("10.0.0.2"), Dev: missingLink},
				},
			}, tableID)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("nexthop 1"))
			Expect(errors.As(err, &netlink.LinkNotFoundError{})).To(BeTrue())
		})

		It("should ignore a non-existent route-level device when next-hops are present", func() {
			r, err := forgeNetlinkRoute(&networkingv1beta1.Route{
				Dst:      cidr("10.1.0.0/24"),
				Dev:      ptr.To(missingLink),
				NextHops: []networkingv1beta1.NextHop{{Gw: ip("10.0.0.1"), Dev: "lo"}},
			}, tableID)

			Expect(err).NotTo(HaveOccurred())
			Expect(r.LinkIndex).To(BeZero())
			Expect(r.MultiPath).To(HaveLen(1))
		})
	})
})

func TestRoute(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Route test suite")
}
