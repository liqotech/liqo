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

package ipamcore

import (
	"fmt"
	"math"
	"net/netip"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Ipam", func() {
	var (
		err        error
		ipam       *Ipam
		validPools = []netip.Prefix{
			netip.MustParsePrefix("10.0.0.0/8"),
			netip.MustParsePrefix("192.168.0.0/16"),
			netip.MustParsePrefix("172.16.0.0/12"),
		}
		invalidPools = []netip.Prefix{
			netip.MustParsePrefix("10.0.1.0/8"),
			netip.MustParsePrefix("192.168.1.0/16"),
			netip.MustParsePrefix("172.16.0.5/12"),
		}
		prefixOutOfPools = netip.MustParsePrefix("11.0.0.0/8")
	)

	BeforeEach(func() {
		var err error
		ipam, err = NewIpam(validPools)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("Ipam Creation", func() {
		When("Using valid pools", func() {
			It("should create an Ipam object", func() {
				ipam, err = NewIpam(validPools)
				Expect(err).NotTo(HaveOccurred())
				Expect(ipam).NotTo(BeNil())
			})
		})
		When("Using invalid pools", func() {
			It("should return an error", func() {
				_, err := NewIpam(invalidPools)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Context("Ipam Utilities", func() {
		When("checking if a prefix is child of another one", func() {
			It("should return true", func() {
				parentPrefix := netip.MustParsePrefix("10.0.0.0/16")
				childPrefixes := []netip.Prefix{
					netip.MustParsePrefix("10.0.0.0/24"),
					netip.MustParsePrefix("10.0.1.0/24"),
					netip.MustParsePrefix("10.0.1.6/24"),
					netip.MustParsePrefix("10.0.0.0/16"),
				}

				for _, childPrefix := range childPrefixes {
					Expect(isPrefixChildOf(parentPrefix, childPrefix)).To(BeTrue())
				}
			})
			It("should return false", func() {
				parentPrefix := netip.MustParsePrefix("10.0.0.0/16")
				childPrefixes := []netip.Prefix{
					netip.MustParsePrefix("0.0.0.0/0"),
					netip.MustParsePrefix("10.0.0.0/15"),
					netip.MustParsePrefix("10.0.0.0/8"),
					netip.MustParsePrefix("10.0.1.0/8"),
				}

				for _, childPrefix := range childPrefixes {
					Expect(isPrefixChildOf(parentPrefix, childPrefix)).To(BeFalse())
				}
			})
		})
		When("forcing a network last update timestamp", func() {
			var (
				acquiredPrefix *netip.Prefix
			)
			BeforeEach(func() {
				acquiredPrefix = ipam.NetworkAcquire(32)
				Expect(acquiredPrefix).NotTo(BeNil())
				Expect(ipam.NetworkIsAvailable(*acquiredPrefix)).To(BeFalse())
			})
			It("should succeed", func() {
				newTime := time.Now().Add(time.Hour)
				Expect(ipam.NetworkSetLastUpdateTimestamp(*acquiredPrefix, newTime)).Should(Succeed())
				node := search(*acquiredPrefix, &ipam.roots[0])
				Expect(node).NotTo(BeNil())
				Expect(node.lastUpdateTimestamp).To(Equal(newTime))
			})
			It("should not succeed", func() {
				Expect(ipam.NetworkSetLastUpdateTimestamp(prefixOutOfPools, time.Now())).ShouldNot(Succeed())

				Expect(ipam.NetworkRelease(*acquiredPrefix, 0).String()).To(Equal(acquiredPrefix.String()))
				Expect(ipam.NetworkIsAvailable(*acquiredPrefix)).To(BeTrue())

				Expect(ipam.NetworkSetLastUpdateTimestamp(*acquiredPrefix, time.Now())).ShouldNot(Succeed())
			})
		})
		When("forcing an IP creation timestamp", func() {
			var (
				acquiredPrefix *netip.Prefix
				acquiredIP     *netip.Addr
			)
			BeforeEach(func() {
				acquiredPrefix = ipam.NetworkAcquire(32)
				Expect(acquiredPrefix).NotTo(BeNil())
				Expect(ipam.NetworkIsAvailable(*acquiredPrefix)).To(BeFalse())

				acquiredIP, err = ipam.IPAcquire(*acquiredPrefix)
				Expect(err).NotTo(HaveOccurred())
				Expect(acquiredIP).NotTo(BeNil())
				Expect(ipam.IPIsAllocated(*acquiredPrefix, *acquiredIP)).To(BeTrue())
			})
			It("should succeed", func() {
				newTime := time.Now().Add(time.Hour)
				Expect(ipam.IPSetCreationTimestamp(*acquiredIP, *acquiredPrefix, newTime)).Should(Succeed())
				node := search(*acquiredPrefix, &ipam.roots[0])
				Expect(node).NotTo(BeNil())
				Expect(node.ips).Should(HaveLen(1))
				Expect(node.ips[0].creationTimestamp).To(Equal(newTime))
			})

			It("should not succeed", func() {
				Expect(ipam.IPSetCreationTimestamp(*acquiredIP, prefixOutOfPools, time.Now())).ShouldNot(Succeed())

				releasedIP, err := ipam.IPRelease(*acquiredPrefix, *acquiredIP, 0)
				Expect(err).NotTo(HaveOccurred())
				Expect(releasedIP).NotTo(BeNil())
				Expect(ipam.IPIsAllocated(*acquiredPrefix, *acquiredIP)).To(BeFalse())

				Expect(ipam.IPSetCreationTimestamp(*acquiredIP, *acquiredPrefix, time.Now())).ShouldNot(Succeed())

				Expect(ipam.NetworkRelease(*acquiredPrefix, 0).String()).To(Equal(acquiredPrefix.String()))
				Expect(ipam.NetworkIsAvailable(*acquiredPrefix)).To(BeTrue())

				Expect(ipam.IPSetCreationTimestamp(*acquiredIP, *acquiredPrefix, time.Now())).ShouldNot(Succeed())
			})
		})
	})

	Context("Ipam networks", func() {
		BeforeEach(func() {
			var err error
			ipam, err = NewIpam(validPools)
			Expect(err).NotTo(HaveOccurred())
		})

		When("creating an Ipam object", func() {
			It("should succeed", func() {
				Expect(ipam).NotTo(BeNil())
			})
		})

		When("listing networks", func() {
			It("should succeed", func() {
				networks := ipam.ListNetworks()
				Expect(networks).Should(HaveLen(0))

				acquiredNetworks := []netip.Prefix{}
				acquiredNetworks = append(acquiredNetworks, *ipam.NetworkAcquire(24))
				acquiredNetworks = append(acquiredNetworks, *ipam.NetworkAcquire(25))
				acquiredNetworks = append(acquiredNetworks, *ipam.NetworkAcquire(26))
				acquiredNetworks = append(acquiredNetworks, *ipam.NetworkAcquire(27))
				acquiredNetworks = append(acquiredNetworks, *ipam.NetworkAcquire(28))

				networks = ipam.ListNetworks()
				Expect(networks).Should(HaveLen(5))
				for i := range acquiredNetworks {
					Expect(networks).Should(ContainElement(acquiredNetworks[i]))
				}
			})
		})

		When("acquiring networks", func() {
			It("should succeed", func() {
				network := ipam.NetworkAcquire(24)
				Expect(network).ShouldNot(BeNil())
				Expect(ipam.NetworkIsAvailable(*network)).To(BeFalse())
			})
			It("should not succeed", func() {
				network := ipam.NetworkAcquire(4)
				Expect(network).Should(BeNil())
			})
		})

		When("releasing networks", func() {
			It("should succeed", func() {
				network := ipam.NetworkAcquire(16)
				Expect(network).ShouldNot(BeNil())
				Expect(ipam.NetworkIsAvailable(*network)).To(BeFalse())
				Expect(ipam.NetworkRelease(*network, 0).String()).To(Equal(network.String()))
				Expect(ipam.NetworkIsAvailable(*network)).To(BeTrue())
			})
			It("should succeed (with grace period)", func() {
				network := ipam.NetworkAcquire(16)
				Expect(network).ShouldNot(BeNil())
				Expect(ipam.NetworkIsAvailable(*network)).To(BeFalse())
				Expect(ipam.NetworkRelease(*network, time.Second*0).String()).To(Equal(network.String()))
				Expect(ipam.NetworkIsAvailable(*network)).To(BeTrue())
			})

			It("should not succeed", func() {
				networks := []netip.Prefix{
					netip.MustParsePrefix("10.0.1.0/24"),
					netip.MustParsePrefix("10.0.2.0/24"),
					netip.MustParsePrefix("10.1.0.0/16"),
					netip.MustParsePrefix("10.2.0.0/16"),
					netip.MustParsePrefix("10.3.0.0/30"),
					netip.MustParsePrefix("10.4.0.0/27"),
				}
				for _, network := range networks {
					Expect(ipam.NetworkIsAvailable(network)).To(BeTrue())
					Expect(ipam.NetworkRelease(network, 0)).To(BeNil())
					Expect(ipam.NetworkRelease(network, time.Second*5)).To(BeNil())
				}
			})
		})

		When("acquiring networks with prefix", func() {
			It("should succeed", func() {
				networks := []netip.Prefix{
					netip.MustParsePrefix("10.0.1.0/24"),
					netip.MustParsePrefix("10.0.2.0/24"),
					netip.MustParsePrefix("10.1.0.0/16"),
					netip.MustParsePrefix("10.2.0.0/16"),
					netip.MustParsePrefix("10.3.8.0/21"),
					netip.MustParsePrefix("10.4.16.0/20"),
					netip.MustParsePrefix("10.128.0.0/20"),
					netip.MustParsePrefix("10.130.64.0/18"),
					netip.MustParsePrefix("10.4.2.0/27"),
					netip.MustParsePrefix("10.3.0.0/30"),
					netip.MustParsePrefix("10.4.0.0/27"),
					netip.MustParsePrefix("10.5.0.0/16"),
					netip.MustParsePrefix("10.4.2.128/25"),
					netip.MustParsePrefix("10.4.3.0/27"),
					netip.MustParsePrefix("10.3.0.24/29"),
				}

				for _, network := range networks {
					Expect(ipam.NetworkIsAvailable(network)).To(BeTrue())
				}

				for _, network := range networks {
					networkAcquired := ipam.NetworkAcquireWithPrefix(network)
					Expect(networkAcquired).NotTo(BeNil())
					Expect(networkAcquired.String()).To(Equal(network.String()))
				}

				for _, network := range networks {
					Expect(ipam.NetworkIsAvailable(network)).To(BeFalse())
				}

				for _, network := range networks {
					Expect(ipam.NetworkRelease(network, 0).String()).To(Equal(network.String()))
				}

				for _, network := range networks {
					Expect(ipam.NetworkIsAvailable(network)).To(BeTrue())
				}
			})

			It("should not succeed", func() {
				networks := []netip.Prefix{
					netip.MustParsePrefix("10.0.1.0/8"),
					netip.MustParsePrefix("11.0.2.0/24"),
					netip.MustParsePrefix("11.1.0.0/16"),
					netip.MustParsePrefix("11.2.0.0/16"),
					netip.MustParsePrefix("12.0.0.0/8"),
					netip.MustParsePrefix("13.4.0.0/8"),
				}
				for _, network := range networks {
					Expect(ipam.NetworkAcquireWithPrefix(network)).To(BeNil())
				}

			})
		})

		When("acquiring a network", func() {
			BeforeEach(func() {
				prefix := netip.MustParsePrefix("10.0.0.0/16")
				Expect(ipam.NetworkAcquireWithPrefix(prefix)).NotTo(BeNil())
				Expect(ipam.NetworkIsAvailable(prefix)).To(BeFalse())

				prefix = netip.MustParsePrefix("10.5.0.0/16")
				Expect(ipam.NetworkAcquireWithPrefix(prefix)).NotTo(BeNil())
				Expect(ipam.NetworkIsAvailable(prefix)).To(BeFalse())
			})
			It("parent networks should not be available", func() {
				prefix := netip.MustParsePrefix("10.0.0.0/14")
				Expect(ipam.NetworkIsAvailable(prefix)).To(BeFalse())
				Expect(ipam.NetworkAcquireWithPrefix(prefix)).To(BeNil())
				Expect(ipam.NetworkIsAvailable(prefix)).To(BeFalse())

				prefix = netip.MustParsePrefix("10.0.0.0/15")
				Expect(ipam.NetworkIsAvailable(prefix)).To(BeFalse())
				Expect(ipam.NetworkAcquireWithPrefix(prefix)).To(BeNil())
				Expect(ipam.NetworkIsAvailable(prefix)).To(BeFalse())

				prefix = netip.MustParsePrefix("10.4.0.0/15")
				Expect(ipam.NetworkIsAvailable(prefix)).To(BeFalse())
				Expect(ipam.NetworkAcquireWithPrefix(prefix)).To(BeNil())
				Expect(ipam.NetworkIsAvailable(prefix)).To(BeFalse())

				prefix = netip.MustParsePrefix("10.4.0.0/14")
				Expect(ipam.NetworkIsAvailable(prefix)).To(BeFalse())
				Expect(ipam.NetworkAcquireWithPrefix(prefix)).To(BeNil())
				Expect(ipam.NetworkIsAvailable(prefix)).To(BeFalse())

				prefix = netip.MustParsePrefix("10.0.0.0/13")
				Expect(ipam.NetworkIsAvailable(prefix)).To(BeFalse())
				Expect(ipam.NetworkAcquireWithPrefix(prefix)).To(BeNil())
				Expect(ipam.NetworkIsAvailable(prefix)).To(BeFalse())
			})

			It("child networks should not be available", func() {
				prefix := netip.MustParsePrefix("10.0.0.0/17")
				Expect(ipam.NetworkIsAvailable(prefix)).To(BeFalse())
				Expect(ipam.NetworkAcquireWithPrefix(prefix)).To(BeNil())
				Expect(ipam.NetworkIsAvailable(prefix)).To(BeFalse())

				prefix = netip.MustParsePrefix("10.0.0.0/18")
				Expect(ipam.NetworkIsAvailable(prefix)).To(BeFalse())
				Expect(ipam.NetworkAcquireWithPrefix(prefix)).To(BeNil())
				Expect(ipam.NetworkIsAvailable(prefix)).To(BeFalse())

				prefix = netip.MustParsePrefix("10.5.0.0/20")
				Expect(ipam.NetworkIsAvailable(prefix)).To(BeFalse())
				Expect(ipam.NetworkAcquireWithPrefix(prefix)).To(BeNil())
				Expect(ipam.NetworkIsAvailable(prefix)).To(BeFalse())

				prefix = netip.MustParsePrefix("11.0.0.0/16")
				Expect(ipam.NetworkIsAvailable(prefix)).To(BeFalse())
				Expect(ipam.NetworkAcquireWithPrefix(prefix)).To(BeNil())
				Expect(ipam.NetworkIsAvailable(prefix)).To(BeFalse())

				prefix = netip.MustParsePrefix("0.0.0.0/0")
				Expect(ipam.NetworkIsAvailable(prefix)).To(BeFalse())
				Expect(ipam.NetworkAcquireWithPrefix(prefix)).To(BeNil())
				Expect(ipam.NetworkIsAvailable(prefix)).To(BeFalse())
			})
		})

		When("generating graphviz", func() {
			BeforeEach(func() {
				sizes := []int{21, 26, 27, 22, 30, 25, 28, 24, 16, 10, 29}
				for _, size := range sizes {
					for i := 0; i < 3; i++ {
						network := ipam.NetworkAcquire(size)
						Expect(network).ShouldNot(BeNil())
					}
				}
			})
			It("it should succeed", func() {
				Expect(ipam.ToGraphviz()).Should(Succeed())
			})
			AfterEach(func() {
				_, err := os.Stat(graphvizFolder)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(os.RemoveAll(graphvizFolder)).ShouldNot(HaveOccurred())
			})
		})
	})

	Context("Ipam IPs", func() {
		var (
			// WARNING: availableIPs must be a power of 2
			availableIPs      = 256
			prefixAcquired    = netip.MustParsePrefix(fmt.Sprintf("10.0.0.0/%d", int(32-math.Log2(float64(availableIPs)))))
			prefixNotAcquired = netip.MustParsePrefix(fmt.Sprintf("10.1.0.0/%d", int(32-math.Log2(float64(availableIPs)))))
		)
		BeforeEach(func() {
			var err error
			ipam, err = NewIpam(validPools)
			Expect(err).NotTo(HaveOccurred())

			Expect(ipam.NetworkIsAvailable(prefixAcquired)).To(BeTrue())
			Expect(ipam.NetworkAcquireWithPrefix(prefixAcquired)).NotTo(BeNil())
			Expect(ipam.NetworkIsAvailable(prefixAcquired)).To(BeFalse())
		})

		When("acquiring an IP from not existing network", func() {
			It("should not succeed (out of pools)", func() {
				addr, err := ipam.IPAcquire(prefixOutOfPools)
				Expect(err).To(HaveOccurred())
				Expect(addr).To(BeNil())
			})
			It("should not succeed (prefix not acquired)", func() {
				addr, err := ipam.IPAcquire(prefixNotAcquired)
				Expect(err).NotTo(HaveOccurred())
				Expect(addr).To(BeNil())
			})
		})
		When("acquiring an IP from existing network", func() {
			It("should succeed", func() {
				addr, err := ipam.IPAcquire(prefixAcquired)
				Expect(err).NotTo(HaveOccurred())
				Expect(addr).NotTo(BeNil())
				Expect(ipam.IPIsAllocated(prefixAcquired, *addr)).To(BeTrue())
			})

			It("should succeed, with specific IP", func() {
				addr, err := ipam.IPAcquireWithAddr(prefixAcquired, prefixAcquired.Addr())
				Expect(err).NotTo(HaveOccurred())
				Expect(addr).NotTo(BeNil())
				Expect(addr.String()).To(Equal(prefixAcquired.Addr().String()))
				Expect(ipam.IPIsAllocated(prefixAcquired, *addr)).To(BeTrue())
			})

			It("should not overflow available IPs)", func() {
				for i := 0; i < availableIPs; i++ {
					addr, err := ipam.IPAcquire(prefixAcquired)
					Expect(err).NotTo(HaveOccurred())
					Expect(addr).NotTo(BeNil())
					Expect(ipam.IPIsAllocated(prefixAcquired, *addr)).To(BeTrue())
				}
				for i := 0; i < availableIPs; i++ {
					addr, err := ipam.IPAcquire(prefixAcquired)
					Expect(err).NotTo(HaveOccurred())
					Expect(addr).To(BeNil())
				}
			})

			It("should not overflow available IPs, with specific IPs)", func() {
				addr := prefixAcquired.Addr()
				for i := 0; i < availableIPs; i++ {
					result, err := ipam.IPAcquire(prefixAcquired)
					Expect(err).NotTo(HaveOccurred())
					Expect(result).NotTo(BeNil())
					Expect(ipam.IPIsAllocated(prefixAcquired, *result)).To(BeTrue())
					addr = addr.Next()
				}
				for i := 0; i < availableIPs; i++ {
					result, err := ipam.IPAcquire(prefixAcquired)
					Expect(err).NotTo(HaveOccurred())
					Expect(result).To(BeNil())
					addr = addr.Next()
				}
			})

			It("should succeed (after an ip has been released)", func() {
				addrs := []netip.Addr{}
				for i := 0; i < availableIPs; i++ {
					addr, err := ipam.IPAcquire(prefixAcquired)
					Expect(err).NotTo(HaveOccurred())
					Expect(addr).NotTo(BeNil())
					addrs = append(addrs, *addr)
				}

				addr, err := ipam.IPRelease(prefixAcquired, addrs[availableIPs/2], 0)
				Expect(err).To(BeNil())
				Expect(addr).NotTo(BeNil())
				Expect(addr.String()).To(Equal(addrs[availableIPs/2].String()))
				Expect(ipam.IPIsAllocated(prefixAcquired, *addr)).To(BeFalse())

				addr, err = ipam.IPAcquire(prefixAcquired)
				Expect(err).NotTo(HaveOccurred())
				Expect(addr).NotTo(BeNil())
				Expect(ipam.IPIsAllocated(prefixAcquired, *addr)).To(BeTrue())
			})
		})
		When("releasing an IP from not existing network", func() {
			It("should not succeed", func() {
				addr := prefixAcquired.Addr()
				for i := 0; i < availableIPs*2; i++ {
					result, err := ipam.IPRelease(prefixNotAcquired, addr, 0)
					Expect(err).NotTo(HaveOccurred())
					Expect(result).To(BeNil())
					addr = addr.Next()
				}
			})
			It("should not succeed (out of pools)", func() {
				addr := prefixAcquired.Addr()
				for i := 0; i < availableIPs*2; i++ {
					result, err := ipam.IPRelease(prefixOutOfPools, addr, 0)
					Expect(err).To(HaveOccurred())
					Expect(result).To(BeNil())
					addr = addr.Next()
				}
			})
		})
		When("releasing an IP from existing network", func() {
			It("should not succeed", func() {
				addr := prefixAcquired.Addr()
				for i := 0; i < availableIPs*2; i++ {
					result, err := ipam.IPRelease(prefixAcquired, addr, 0)
					Expect(err).NotTo(HaveOccurred())
					Expect(result).To(BeNil())
					addr = addr.Next()
				}
			})
			It("should succeed", func() {
				addrs := []netip.Addr{}
				for i := 0; i < availableIPs; i++ {
					addr, err := ipam.IPAcquire(prefixAcquired)
					Expect(err).NotTo(HaveOccurred())
					Expect(addr).NotTo(BeNil())
					addrs = append(addrs, *addr)
				}
				for i := range addrs {
					addr, err := ipam.IPRelease(prefixAcquired, addrs[i], 0)
					Expect(err).To(BeNil())
					Expect(addr).NotTo(BeNil())
					Expect(addr.String()).To(Equal(addrs[i].String()))
				}
			})

		})

		When("listing IPs in a network", func() {
			var acquiredIPs []netip.Addr
			BeforeEach(func() {
				addr := prefixAcquired.Addr()
				for i := 0; i < availableIPs/2; i++ {
					result, err := ipam.IPAcquireWithAddr(prefixAcquired, addr)
					Expect(err).NotTo(HaveOccurred())
					Expect(result).NotTo(BeNil())
					acquiredIPs = append(acquiredIPs, addr)
					addr = addr.Next()
				}
				for i := 0; i < availableIPs/2; i++ {
					result, err := ipam.IPAcquire(prefixAcquired)
					Expect(err).NotTo(HaveOccurred())
					Expect(result).NotTo(BeNil())
					acquiredIPs = append(acquiredIPs, *result)
				}

			})
			It("should contains the correct IPs", func() {
				cachedIPs, err := ipam.ListIPs(prefixAcquired)
				Expect(err).NotTo(HaveOccurred())
				for i := range acquiredIPs {
					Expect(cachedIPs).Should(ContainElement(acquiredIPs[i]))
				}
			})
			It("should be void", func() {
				cachedIPs, err := ipam.ListIPs(prefixNotAcquired)
				Expect(err).NotTo(HaveOccurred())
				Expect(cachedIPs).Should(HaveLen(0))
			})
		})
	})
})
