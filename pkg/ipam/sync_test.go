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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
)

var _ = Describe("Sync routine tests", func() {
	const (
		syncFrequency = 3 * time.Second
		testNamespace = "test"
	)

	var (
		ctx               context.Context
		fakeClientBuilder *fake.ClientBuilder
		now               time.Time
		newEntryThreshold time.Time
		notNewTimestamp   time.Time

		fakeIpamServer *LiqoIPAM

		addNetowrkToCache = func(ipamServer *LiqoIPAM, cidr string, creationTimestamp time.Time) {
			ipamServer.cacheNetworks[cidr] = networkInfo{
				network: network{
					cidr: cidr,
				},
				creationTimestamp: creationTimestamp,
			}
		}

		addIPToCache = func(ipamServer *LiqoIPAM, ip, cidr string, creationTimestamp time.Time) {
			ipC := ipCidr{ip: ip, cidr: cidr}
			ipamServer.cacheIPs[ipC.String()] = ipInfo{
				ipCidr:            ipC,
				creationTimestamp: creationTimestamp,
			}
		}

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
	)

	BeforeEach(func() {
		ctx = context.Background()
		fakeClientBuilder = fake.NewClientBuilder().WithScheme(scheme.Scheme)
		now = time.Now()
		newEntryThreshold = now.Add(-syncFrequency)
		notNewTimestamp = newEntryThreshold.Add(-time.Minute)
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

				// Populate the cache
				fakeIpamServer = &LiqoIPAM{
					Client:        client,
					cacheNetworks: make(map[string]networkInfo),
				}
				addNetowrkToCache(fakeIpamServer, "10.0.0.0/16", now)
				addNetowrkToCache(fakeIpamServer, "10.1.0.0/16", notNewTimestamp)
				addNetowrkToCache(fakeIpamServer, "10.3.0.0/16", notNewTimestamp)
				addNetowrkToCache(fakeIpamServer, "10.4.0.0/16", now)
			})

			It("should remove networks from cache if they are not present in the cluster", func() {
				// Run sync
				Expect(fakeIpamServer.syncNetworks(ctx, newEntryThreshold)).To(Succeed())

				// Check the cache
				Expect(fakeIpamServer.cacheNetworks).To(HaveKey("10.0.0.0/16"))    // network in cluster and cache
				Expect(fakeIpamServer.cacheNetworks).To(HaveKey("10.1.0.0/16"))    // network in cluster and cache before new entry threshold
				Expect(fakeIpamServer.cacheNetworks).To(HaveKey("10.2.0.0/16"))    // network in cluster but not in cache
				Expect(fakeIpamServer.cacheNetworks).NotTo(HaveKey("10.3.0.0/16")) // network not in cluster but in cache before new entry threshold
				Expect(fakeIpamServer.cacheNetworks).To(HaveKey("10.4.0.0/16"))    // network not in cluster but in cache after new entry threshold
			})
		})

		Context("Sync IPs", func() {
			BeforeEach(func() {
				// Add in-cluster IPs
				client := fakeClientBuilder.WithObjects(
					newIP("ip1", "10.0.0.0", "10.0.0.0/24"),
					newIP("ip2", "10.0.0.1", "10.0.0.0/24"),
					newIP("ip3", "10.0.0.2", "10.0.0.0/24"),
				).Build()

				// Populate the cache
				fakeIpamServer = &LiqoIPAM{
					Client:   client,
					cacheIPs: make(map[string]ipInfo),
				}
				addIPToCache(fakeIpamServer, "10.0.0.0", "10.0.0.0/24", now)
				addIPToCache(fakeIpamServer, "10.0.0.1", "10.0.0.0/24", notNewTimestamp)
				addIPToCache(fakeIpamServer, "10.0.0.3", "10.0.0.0/24", notNewTimestamp)
				addIPToCache(fakeIpamServer, "10.0.0.4", "10.0.0.0/24", now)
			})

			It("should remove IPs from cache if they are not present in the cluster", func() {
				// Run sync
				Expect(fakeIpamServer.syncIPs(ctx, newEntryThreshold)).To(Succeed())

				// Check the cache
				Expect(fakeIpamServer.cacheIPs).To(HaveKey(
					ipCidr{ip: "10.0.0.0", cidr: "10.0.0.0/24"}.String())) // IP in cluster and cache
				Expect(fakeIpamServer.cacheIPs).To(HaveKey(
					ipCidr{ip: "10.0.0.1", cidr: "10.0.0.0/24"}.String())) // IP in cluster and cache before new entry threshold
				Expect(fakeIpamServer.cacheIPs).To(HaveKey(
					ipCidr{ip: "10.0.0.2", cidr: "10.0.0.0/24"}.String())) // IP in cluster but not in cache
				Expect(fakeIpamServer.cacheIPs).NotTo(HaveKey(
					ipCidr{ip: "10.0.0.3", cidr: "10.0.0.0/24"}.String())) // IP not in cluster but in cache before new entry threshold
				Expect(fakeIpamServer.cacheIPs).To(HaveKey(
					ipCidr{ip: "10.0.0.4", cidr: "10.0.0.0/24"}.String())) // IP not in cluster but in cache after new entry threshold
			})
		})
	})
})
