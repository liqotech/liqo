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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	ipamcore "github.com/liqotech/liqo/pkg/ipam/core"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

var _ = Describe("Sync routine tests", func() {
	const (
		syncGracePeriod = time.Second * 5
		testNamespace   = "test"
	)

	var (
		ctx               context.Context
		fakeClientBuilder *fake.ClientBuilder

		fakeIpamServer *LiqoIPAM

		addNetwork = func(server *LiqoIPAM, cidr string) {
			prefix, err := netip.ParsePrefix(cidr)
			Expect(err).ShouldNot(HaveOccurred())
			_, err = server.networkAcquireSpecific(prefix)
			Expect(err).ShouldNot(HaveOccurred())
		}

		addIP = func(server *LiqoIPAM, ip, cidr string) {
			addr, err := netip.ParseAddr(ip)
			Expect(err).ShouldNot(HaveOccurred())
			prefix, err := netip.ParsePrefix(cidr)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(server.ipAcquireWithAddr(addr, prefix)).Should(Succeed())
		}

		addPreAllocated = func(nw *ipamv1alpha1.Network, preAllocated uint32) *ipamv1alpha1.Network {
			nw.Spec.PreAllocated = preAllocated
			return nw
		}
	)

	BeforeEach(func() {
		ctx = context.Background()
		fakeClientBuilder = fake.NewClientBuilder().WithScheme(scheme.Scheme)
	})

	Describe("Testing the sync routine", func() {
		Context("Sync Networks", func() {
			BeforeEach(func() {
				// Add in-cluster networks
				client := fakeClientBuilder.WithObjects(
					testutil.FakeNetwork("net1", testNamespace, "10.0.0.0/16", nil),
					testutil.FakeNetwork("net2", testNamespace, "10.1.0.0/16", nil),
					testutil.FakeNetwork("net3", testNamespace, "10.2.0.0/16", nil),
					testutil.FakeNetwork("net4", testNamespace, "10.4.0.0/16", nil),
				).Build()

				ipamCore, err := ipamcore.NewIpam([]netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")})
				Expect(err).To(BeNil())

				// Populate the cache
				fakeIpamServer = &LiqoIPAM{
					Client:   client,
					IpamCore: ipamCore,
					opts: &ServerOptions{
						SyncGracePeriod: syncGracePeriod,
						GraphvizEnabled: false,
					},
				}
				addNetwork(fakeIpamServer, "10.0.0.0/16")
				addNetwork(fakeIpamServer, "10.1.0.0/16")
				addNetwork(fakeIpamServer, "10.3.0.0/16")
				addNetwork(fakeIpamServer, "10.5.0.0/16")
			})

			It("should remove networks from cache if they are not present in the cluster", func() {
				newLastUpdate := time.Now().Add(-syncGracePeriod)

				// Check the cache
				Expect(fakeIpamServer.networkIsAvailable(netip.MustParsePrefix("10.0.0.0/16"))).To(Equal(false))
				Expect(fakeIpamServer.networkIsAvailable(netip.MustParsePrefix("10.1.0.0/16"))).To(Equal(false))
				Expect(fakeIpamServer.networkIsAvailable(netip.MustParsePrefix("10.2.0.0/16"))).To(Equal(true))
				Expect(fakeIpamServer.networkIsAvailable(netip.MustParsePrefix("10.3.0.0/16"))).To(Equal(false))
				Expect(fakeIpamServer.networkIsAvailable(netip.MustParsePrefix("10.4.0.0/16"))).To(Equal(true))
				Expect(fakeIpamServer.networkIsAvailable(netip.MustParsePrefix("10.5.0.0/16"))).To(Equal(false))

				// Run sync
				Expect(fakeIpamServer.syncNetworks(ctx)).To(Succeed())

				Expect(fakeIpamServer.networkIsAvailable(netip.MustParsePrefix("10.0.0.0/16"))).To(Equal(false))
				Expect(fakeIpamServer.networkIsAvailable(netip.MustParsePrefix("10.1.0.0/16"))).To(Equal(false))
				Expect(fakeIpamServer.networkIsAvailable(netip.MustParsePrefix("10.2.0.0/16"))).To(Equal(false))
				Expect(fakeIpamServer.networkIsAvailable(netip.MustParsePrefix("10.3.0.0/16"))).To(Equal(false))
				Expect(fakeIpamServer.networkIsAvailable(netip.MustParsePrefix("10.4.0.0/16"))).To(Equal(false))
				Expect(fakeIpamServer.networkIsAvailable(netip.MustParsePrefix("10.5.0.0/16"))).To(Equal(false))

				// Update the last update timestamp of the networks
				Expect(fakeIpamServer.IpamCore.NetworkSetLastUpdateTimestamp(netip.MustParsePrefix("10.0.0.0/16"), newLastUpdate)).Should(Succeed())
				Expect(fakeIpamServer.IpamCore.NetworkSetLastUpdateTimestamp(netip.MustParsePrefix("10.1.0.0/16"), newLastUpdate)).Should(Succeed())
				Expect(fakeIpamServer.IpamCore.NetworkSetLastUpdateTimestamp(netip.MustParsePrefix("10.2.0.0/16"), newLastUpdate)).Should(Succeed())
				Expect(fakeIpamServer.IpamCore.NetworkSetLastUpdateTimestamp(netip.MustParsePrefix("10.4.0.0/16"), newLastUpdate)).Should(Succeed())
				Expect(fakeIpamServer.IpamCore.NetworkSetLastUpdateTimestamp(netip.MustParsePrefix("10.5.0.0/16"), newLastUpdate)).Should(Succeed())

				// Run sync
				Expect(fakeIpamServer.syncNetworks(ctx)).To(Succeed())

				// Check the cache
				Expect(fakeIpamServer.networkIsAvailable(netip.MustParsePrefix("10.0.0.0/16"))).To(Equal(false))
				Expect(fakeIpamServer.networkIsAvailable(netip.MustParsePrefix("10.1.0.0/16"))).To(Equal(false))
				Expect(fakeIpamServer.networkIsAvailable(netip.MustParsePrefix("10.2.0.0/16"))).To(Equal(false))
				Expect(fakeIpamServer.networkIsAvailable(netip.MustParsePrefix("10.3.0.0/16"))).To(Equal(false))
				Expect(fakeIpamServer.networkIsAvailable(netip.MustParsePrefix("10.4.0.0/16"))).To(Equal(false))
				Expect(fakeIpamServer.networkIsAvailable(netip.MustParsePrefix("10.5.0.0/16"))).To(Equal(true))

				// Update the last update timestamp of the networks
				Expect(fakeIpamServer.IpamCore.NetworkSetLastUpdateTimestamp(netip.MustParsePrefix("10.3.0.0/16"), newLastUpdate)).Should(Succeed())

				// Run sync
				Expect(fakeIpamServer.syncNetworks(ctx)).To(Succeed())

				// Check the cache
				Expect(fakeIpamServer.networkIsAvailable(netip.MustParsePrefix("10.0.0.0/16"))).To(Equal(false))
				Expect(fakeIpamServer.networkIsAvailable(netip.MustParsePrefix("10.1.0.0/16"))).To(Equal(false))
				Expect(fakeIpamServer.networkIsAvailable(netip.MustParsePrefix("10.2.0.0/16"))).To(Equal(false))
				Expect(fakeIpamServer.networkIsAvailable(netip.MustParsePrefix("10.3.0.0/16"))).To(Equal(true))
				Expect(fakeIpamServer.networkIsAvailable(netip.MustParsePrefix("10.4.0.0/16"))).To(Equal(false))
				Expect(fakeIpamServer.networkIsAvailable(netip.MustParsePrefix("10.5.0.0/16"))).To(Equal(true))
			})
		})

		Context("Sync IPs", func() {
			BeforeEach(func() {
				// Add in-cluster IPs
				client := fakeClientBuilder.WithObjects(
					testutil.FakeNetwork("net1", testNamespace, "10.0.0.0/24", nil),

					testutil.FakeIP("ip1", testNamespace, "10.0.0.0", "10.0.0.0/24", nil, nil, false),
					testutil.FakeIP("ip2", testNamespace, "10.0.0.1", "10.0.0.0/24", nil, nil, false),
					testutil.FakeIP("ip3", testNamespace, "10.0.0.2", "10.0.0.0/24", nil, nil, false),
				).Build()

				ipamCore, err := ipamcore.NewIpam([]netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")})
				Expect(err).To(BeNil())

				// Populate the cache
				fakeIpamServer = &LiqoIPAM{
					Client:   client,
					IpamCore: ipamCore,
					opts: &ServerOptions{
						GraphvizEnabled: false,
						SyncGracePeriod: syncGracePeriod,
					},
				}

				addNetwork(fakeIpamServer, "10.0.0.0/24")

				addIP(fakeIpamServer, "10.0.0.0", "10.0.0.0/24")
				addIP(fakeIpamServer, "10.0.0.1", "10.0.0.0/24")
				addIP(fakeIpamServer, "10.0.0.3", "10.0.0.0/24")
				addIP(fakeIpamServer, "10.0.0.4", "10.0.0.0/24")
			})

			It("should remove IPs from cache if they are not present in the cluster", func() {
				newCreationTimestamp := time.Now().Add(-syncGracePeriod)

				// Check the cache before grace period
				Expect(fakeIpamServer.ipIsAvailable(netip.MustParseAddr("10.0.0.0"), netip.MustParsePrefix("10.0.0.0/24"))).To(Equal(false))
				Expect(fakeIpamServer.ipIsAvailable(netip.MustParseAddr("10.0.0.1"), netip.MustParsePrefix("10.0.0.0/24"))).To(Equal(false))
				Expect(fakeIpamServer.ipIsAvailable(netip.MustParseAddr("10.0.0.2"), netip.MustParsePrefix("10.0.0.0/24"))).To(Equal(true))
				Expect(fakeIpamServer.ipIsAvailable(netip.MustParseAddr("10.0.0.3"), netip.MustParsePrefix("10.0.0.0/24"))).To(Equal(false))
				Expect(fakeIpamServer.ipIsAvailable(netip.MustParseAddr("10.0.0.4"), netip.MustParsePrefix("10.0.0.0/24"))).To(Equal(false))

				Expect(fakeIpamServer.syncIPs(ctx)).To(Succeed())

				Expect(fakeIpamServer.ipIsAvailable(netip.MustParseAddr("10.0.0.0"), netip.MustParsePrefix("10.0.0.0/24"))).To(Equal(false))
				Expect(fakeIpamServer.ipIsAvailable(netip.MustParseAddr("10.0.0.1"), netip.MustParsePrefix("10.0.0.0/24"))).To(Equal(false))
				Expect(fakeIpamServer.ipIsAvailable(netip.MustParseAddr("10.0.0.2"), netip.MustParsePrefix("10.0.0.0/24"))).To(Equal(false))
				Expect(fakeIpamServer.ipIsAvailable(netip.MustParseAddr("10.0.0.3"), netip.MustParsePrefix("10.0.0.0/24"))).To(Equal(false))
				Expect(fakeIpamServer.ipIsAvailable(netip.MustParseAddr("10.0.0.4"), netip.MustParsePrefix("10.0.0.0/24"))).To(Equal(false))

				// Update the creation timestamp of the IPs
				Expect(fakeIpamServer.IpamCore.IPSetCreationTimestamp(
					netip.MustParseAddr("10.0.0.0"), netip.MustParsePrefix("10.0.0.0/24"), newCreationTimestamp)).Should(Succeed())
				Expect(fakeIpamServer.IpamCore.IPSetCreationTimestamp(
					netip.MustParseAddr("10.0.0.1"), netip.MustParsePrefix("10.0.0.0/24"), newCreationTimestamp)).Should(Succeed())
				Expect(fakeIpamServer.IpamCore.IPSetCreationTimestamp(
					netip.MustParseAddr("10.0.0.2"), netip.MustParsePrefix("10.0.0.0/24"), newCreationTimestamp)).Should(Succeed())
				Expect(fakeIpamServer.IpamCore.IPSetCreationTimestamp(
					netip.MustParseAddr("10.0.0.4"), netip.MustParsePrefix("10.0.0.0/24"), newCreationTimestamp)).Should(Succeed())

				// Run sync
				Expect(fakeIpamServer.syncIPs(ctx)).To(Succeed())

				// Check the cache after grace period
				Expect(fakeIpamServer.ipIsAvailable(netip.MustParseAddr("10.0.0.0"), netip.MustParsePrefix("10.0.0.0/24"))).To(Equal(false))
				Expect(fakeIpamServer.ipIsAvailable(netip.MustParseAddr("10.0.0.1"), netip.MustParsePrefix("10.0.0.0/24"))).To(Equal(false))
				Expect(fakeIpamServer.ipIsAvailable(netip.MustParseAddr("10.0.0.2"), netip.MustParsePrefix("10.0.0.0/24"))).To(Equal(false))
				Expect(fakeIpamServer.ipIsAvailable(netip.MustParseAddr("10.0.0.3"), netip.MustParsePrefix("10.0.0.0/24"))).To(Equal(false))
				Expect(fakeIpamServer.ipIsAvailable(netip.MustParseAddr("10.0.0.4"), netip.MustParsePrefix("10.0.0.0/24"))).To(Equal(true))

				// Update the creation timestamp of the IPs
				Expect(fakeIpamServer.IpamCore.IPSetCreationTimestamp(
					netip.MustParseAddr("10.0.0.3"), netip.MustParsePrefix("10.0.0.0/24"), newCreationTimestamp)).Should(Succeed())

				// Run sync
				Expect(fakeIpamServer.syncIPs(ctx)).To(Succeed())

				// Check the cache after grace period
				Expect(fakeIpamServer.ipIsAvailable(netip.MustParseAddr("10.0.0.0"), netip.MustParsePrefix("10.0.0.0/24"))).To(Equal(false))
				Expect(fakeIpamServer.ipIsAvailable(netip.MustParseAddr("10.0.0.1"), netip.MustParsePrefix("10.0.0.0/24"))).To(Equal(false))
				Expect(fakeIpamServer.ipIsAvailable(netip.MustParseAddr("10.0.0.2"), netip.MustParsePrefix("10.0.0.0/24"))).To(Equal(false))
				Expect(fakeIpamServer.ipIsAvailable(netip.MustParseAddr("10.0.0.3"), netip.MustParsePrefix("10.0.0.0/24"))).To(Equal(true))
				Expect(fakeIpamServer.ipIsAvailable(netip.MustParseAddr("10.0.0.4"), netip.MustParsePrefix("10.0.0.0/24"))).To(Equal(true))
			})
		})

		Context("Sync preAllocated IPs", func() {
			BeforeEach(func() {
				client := fakeClientBuilder.Build()

				ipamCore, err := ipamcore.NewIpam([]netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")})
				Expect(err).To(BeNil())

				fakeIpamServer = &LiqoIPAM{
					Client:   client,
					IpamCore: ipamCore,
					opts: &ServerOptions{
						GraphvizEnabled: false,
						SyncGracePeriod: syncGracePeriod,
					},
				}

				// Acquire network with preAllocated IPs
				_, err = fakeIpamServer.NetworkAcquire(ctx, &NetworkAcquireRequest{
					Cidr:         "10.0.0.0/24",
					Immutable:    true,
					PreAllocated: 3,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(fakeIpamServer.networkIsAvailable(netip.MustParsePrefix("10.0.0.0/24"))).To(BeFalse())
				Expect(fakeIpamServer.ipIsAvailable(netip.MustParseAddr("10.0.0.0"), netip.MustParsePrefix("10.0.0.0/24"))).To(BeFalse()) // preAllocated
				Expect(fakeIpamServer.ipIsAvailable(netip.MustParseAddr("10.0.0.1"), netip.MustParsePrefix("10.0.0.0/24"))).To(BeFalse()) // preAllocated
				Expect(fakeIpamServer.ipIsAvailable(netip.MustParseAddr("10.0.0.2"), netip.MustParsePrefix("10.0.0.0/24"))).To(BeFalse()) // preAllocated

				// release preallocated IP
				_, err = fakeIpamServer.IPRelease(ctx, &IPReleaseRequest{
					Cidr: "10.0.0.0/24",
					Ip:   "10.0.0.1",
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(fakeIpamServer.ipIsAvailable(netip.MustParseAddr("10.0.0.0"), netip.MustParsePrefix("10.0.0.0/24"))).To(BeFalse())
				Expect(fakeIpamServer.ipIsAvailable(netip.MustParseAddr("10.0.0.1"), netip.MustParsePrefix("10.0.0.0/24"))).To(BeTrue())
				Expect(fakeIpamServer.ipIsAvailable(netip.MustParseAddr("10.0.0.2"), netip.MustParsePrefix("10.0.0.0/24"))).To(BeFalse())
			})

			It("it should re-allocate preAllocated IP if deleted", func() {
				// Run sync.
				// Add the network with the preallocated IP to the cluster (i.e., inject to the client),
				// so that the sync routine does not delete it.
				cl := fakeClientBuilder.WithObjects(
					addPreAllocated(testutil.FakeNetwork("net1", testNamespace, "10.0.0.0/24", nil), 3),
				).Build()
				fakeIpamServer.Client = cl
				Expect(fakeIpamServer.syncNetworks(ctx)).To(Succeed())
				Expect(fakeIpamServer.syncIPs(ctx)).To(Succeed())

				// The preAllocated IP should be re-allocated
				Expect(fakeIpamServer.ipIsAvailable(netip.MustParseAddr("10.0.0.0"), netip.MustParsePrefix("10.0.0.0/24"))).To(BeFalse())
				Expect(fakeIpamServer.ipIsAvailable(netip.MustParseAddr("10.0.0.1"), netip.MustParsePrefix("10.0.0.0/24"))).To(BeFalse())
				Expect(fakeIpamServer.ipIsAvailable(netip.MustParseAddr("10.0.0.2"), netip.MustParsePrefix("10.0.0.0/24"))).To(BeFalse())
			})
		})
	})
})
