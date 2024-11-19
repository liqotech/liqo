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

package ipam

import (
	"context"
	"net/netip"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	ipamcore "github.com/liqotech/liqo/pkg/ipam/core"
)

var _ = Describe("Sync routine tests", func() {
	const (
		syncFrequency = 0
		testNamespace = "test"
	)

	var (
		ctx               context.Context
		fakeClientBuilder *fake.ClientBuilder

		fakeIpamServer *LiqoIPAM

		newNetwork = func(name, cidr string) *ipamv1alpha1.Network {
			return &ipamv1alpha1.Network{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: testNamespace,
				},
				Spec: ipamv1alpha1.NetworkSpec{
					CIDR: networkingv1beta1.CIDR(cidr),
				},
				Status: ipamv1alpha1.NetworkStatus{
					CIDR: networkingv1beta1.CIDR(cidr),
				},
			}
		}

		newIP = func(name, ip, cidr string) *ipamv1alpha1.IP {
			return &ipamv1alpha1.IP{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: testNamespace,
				},
				Spec: ipamv1alpha1.IPSpec{
					IP: networkingv1beta1.IP(ip),
				},
				Status: ipamv1alpha1.IPStatus{
					IP:   networkingv1beta1.IP(ip),
					CIDR: networkingv1beta1.CIDR(cidr),
				},
			}
		}

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
					newNetwork("net1", "10.0.0.0/16"),
					newNetwork("net2", "10.1.0.0/16"),
					newNetwork("net3", "10.2.0.0/16"),
				).Build()

				ipamCore, err := ipamcore.NewIpam([]string{"10.0.0.0/8"})
				Expect(err).To(BeNil())

				// Populate the cache
				fakeIpamServer = &LiqoIPAM{
					Client:   client,
					IpamCore: ipamCore,
					opts: &ServerOptions{
						SyncFrequency:   syncFrequency,
						GraphvizEnabled: false,
					},
				}
				addNetwork(fakeIpamServer, "10.0.0.0/16")
				addNetwork(fakeIpamServer, "10.1.0.0/16")
				addNetwork(fakeIpamServer, "10.3.0.0/16")
				addNetwork(fakeIpamServer, "10.4.0.0/16")
			})

			It("should remove networks from cache if they are not present in the cluster", func() {
				// Run sync
				Expect(fakeIpamServer.syncNetworks(ctx)).To(Succeed())

				// Check the cache
				Expect(fakeIpamServer.networkIsAvailable(netip.MustParsePrefix("10.0.0.0/16"))).To(Equal(false))
				Expect(fakeIpamServer.networkIsAvailable(netip.MustParsePrefix("10.1.0.0/16"))).To(Equal(false))
				Expect(fakeIpamServer.networkIsAvailable(netip.MustParsePrefix("10.2.0.0/16"))).To(Equal(false))
				Expect(fakeIpamServer.networkIsAvailable(netip.MustParsePrefix("10.3.0.0/16"))).To(Equal(true))
				Expect(fakeIpamServer.networkIsAvailable(netip.MustParsePrefix("10.4.0.0/16"))).To(Equal(true))
			})
		})

		Context("Sync IPs", func() {
			BeforeEach(func() {
				// Add in-cluster IPs
				client := fakeClientBuilder.WithObjects(
					newNetwork("net1", "10.0.0.0/24"),

					newIP("ip1", "10.0.0.0", "10.0.0.0/24"),
					newIP("ip2", "10.0.0.1", "10.0.0.0/24"),
					newIP("ip3", "10.0.0.2", "10.0.0.0/24"),
				).Build()

				ipamCore, err := ipamcore.NewIpam([]string{"10.0.0.0/8"})
				Expect(err).To(BeNil())

				// Populate the cache
				fakeIpamServer = &LiqoIPAM{
					Client:   client,
					IpamCore: ipamCore,
					opts: &ServerOptions{
						SyncFrequency:   syncFrequency,
						GraphvizEnabled: false,
					},
				}

				addNetwork(fakeIpamServer, "10.0.0.0/24")

				addIP(fakeIpamServer, "10.0.0.0", "10.0.0.0/24")
				addIP(fakeIpamServer, "10.0.0.1", "10.0.0.0/24")
				addIP(fakeIpamServer, "10.0.0.3", "10.0.0.0/24")
				addIP(fakeIpamServer, "10.0.0.4", "10.0.0.0/24")
			})

			It("should remove IPs from cache if they are not present in the cluster", func() {
				// Run sync
				Expect(fakeIpamServer.syncIPs(ctx)).To(Succeed())

				// Check the cache
				Expect(fakeIpamServer.isIPAvailable(netip.MustParseAddr("10.0.0.0"), netip.MustParsePrefix("10.0.0.0/24"))).To(Equal(false))
				Expect(fakeIpamServer.isIPAvailable(netip.MustParseAddr("10.0.0.1"), netip.MustParsePrefix("10.0.0.0/24"))).To(Equal(false))
				Expect(fakeIpamServer.isIPAvailable(netip.MustParseAddr("10.0.0.2"), netip.MustParsePrefix("10.0.0.0/24"))).To(Equal(false))
				Expect(fakeIpamServer.isIPAvailable(netip.MustParseAddr("10.0.0.3"), netip.MustParsePrefix("10.0.0.0/24"))).To(Equal(true))
				Expect(fakeIpamServer.isIPAvailable(netip.MustParseAddr("10.0.0.4"), netip.MustParsePrefix("10.0.0.0/24"))).To(Equal(true))
			})
		})
	})
})
