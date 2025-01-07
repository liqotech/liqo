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

package ipam

import (
	"context"
	"net/netip"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	ipamcore "github.com/liqotech/liqo/pkg/ipam/core"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

var _ = Describe("Initialize routine tests", func() {
	const (
		testNamespace = "test"
	)

	var (
		ctx               context.Context
		fakeClientBuilder *fake.ClientBuilder

		fakeIpamServer *LiqoIPAM
	)

	BeforeEach(func() {
		ctx = context.Background()
		fakeClientBuilder = fake.NewClientBuilder().WithScheme(scheme.Scheme)
	})

	Describe("Testing ipam initialization", func() {
		Context("Initialize", func() {
			BeforeEach(func() {
				// Add in-cluster networks
				client := fakeClientBuilder.WithObjects(
					// First pool
					testutil.FakeNetwork("net1", testNamespace, "10.0.0.0/16", nil),
					testutil.FakeNetwork("net2", testNamespace, "10.2.0.0/16", nil),
					testutil.FakeNetwork("net3", testNamespace, "10.4.0.0/24", nil),
					testutil.FakeNetwork("net4", testNamespace, "10.3.0.0/16", nil), // network with some IPs
					testutil.FakeIP("ip1", testNamespace, "10.3.0.0", "10.3.0.0/16", nil, nil, false),
					testutil.FakeIP("ip2", testNamespace, "10.3.0.2", "10.3.0.0/16", nil, nil, false),

					// Second pool
					testutil.FakeNetwork("net5", testNamespace, "192.168.1.0/24", nil),

					// Network with full pool
					testutil.FakeNetwork("net6", testNamespace, "172.16.1.0/24", nil),
				).Build()

				ipamCore, err := ipamcore.NewIpam([]netip.Prefix{
					netip.MustParsePrefix("10.0.0.0/8"),
					netip.MustParsePrefix("192.168.0.0/16"),
					netip.MustParsePrefix("172.16.1.0/24"),
				})
				Expect(err).To(BeNil())

				// Init ipam server
				fakeIpamServer = &LiqoIPAM{
					Client:   client,
					IpamCore: ipamCore,
					opts: &ServerOptions{
						GraphvizEnabled: false,
					},
				}
			})

			It("should populate the cache", func() {
				// Run initialize
				Expect(fakeIpamServer.initialize(ctx)).To(Succeed())

				// Check the cache:

				// First pool
				Expect(fakeIpamServer.networkIsAvailable(netip.MustParsePrefix("10.0.0.0/16"))).To(Equal(false))
				Expect(fakeIpamServer.networkIsAvailable(netip.MustParsePrefix("10.1.0.0/16"))).To(Equal(true))
				Expect(fakeIpamServer.networkIsAvailable(netip.MustParsePrefix("10.2.0.0/16"))).To(Equal(false))
				Expect(fakeIpamServer.networkIsAvailable(netip.MustParsePrefix("10.4.0.0/24"))).To(Equal(false))
				Expect(fakeIpamServer.networkIsAvailable(netip.MustParsePrefix("10.4.0.0/16"))).To(Equal(false))
				Expect(fakeIpamServer.networkIsAvailable(netip.MustParsePrefix("10.4.0.0/30"))).To(Equal(false))
				Expect(fakeIpamServer.networkIsAvailable(netip.MustParsePrefix("10.3.0.0/16"))).To(Equal(false))
				Expect(fakeIpamServer.networkIsAvailable(netip.MustParsePrefix("172.16.1.0/24"))).To(Equal(false))
				available, err := fakeIpamServer.ipIsAvailable(netip.MustParseAddr("10.3.0.0"), netip.MustParsePrefix("10.3.0.0/16"))
				Expect(err).To(BeNil())
				Expect(available).To(Equal(false))
				available, err = fakeIpamServer.ipIsAvailable(netip.MustParseAddr("10.3.0.1"), netip.MustParsePrefix("10.3.0.0/16"))
				Expect(err).To(BeNil())
				Expect(available).To(Equal(true))
				available, err = fakeIpamServer.ipIsAvailable(netip.MustParseAddr("10.3.0.2"), netip.MustParsePrefix("10.3.0.0/16"))
				Expect(err).To(BeNil())
				Expect(available).To(Equal(false))

				// Second pool
				Expect(fakeIpamServer.networkIsAvailable(netip.MustParsePrefix("192.168.1.0/24"))).To(Equal(false))
				Expect(fakeIpamServer.networkIsAvailable(netip.MustParsePrefix("192.168.2.0/24"))).To(Equal(true))

				// Out of pools
				Expect(fakeIpamServer.networkIsAvailable(netip.MustParsePrefix("1.1.1.1/24"))).To(Equal(false))
			})
		})
	})
})
