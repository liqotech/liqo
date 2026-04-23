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

	Context("Ipam creation", func() {
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

			It("Should return false", func() {
				poolsFalse := []netip.Prefix{
					netip.MustParsePrefix("1.0.0.0/8"),
					netip.MustParsePrefix("0.0.0.0/0"),
				}
				poolsTrue := []netip.Prefix{
					netip.MustParsePrefix("10.0.0.0/24"),
					netip.MustParsePrefix("192.168.0.0/30"),
				}
				for _, pool := range poolsFalse {
					Expect(ipam.IsPrefixInRoots(pool)).To(BeFalse())
				}
				for _, pool := range poolsTrue {
					Expect(ipam.IsPrefixInRoots(pool)).To(BeTrue())
				}
			})
		})

		When("Using overlapping pools", func() {
			It("should return an error if one pool contains another", func() {
				_, err := NewIpam([]netip.Prefix{
					netip.MustParsePrefix("10.0.0.0/8"),
					netip.MustParsePrefix("10.0.0.0/16"),
				})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("overlap"))
			})

			It("should return an error if pools partially overlap", func() {
				_, err := NewIpam([]netip.Prefix{
					netip.MustParsePrefix("10.0.0.0/15"),
					netip.MustParsePrefix("10.1.0.0/16"),
				})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("overlap"))
			})
		})
	})

	Context("Ipam utilities", func() {
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
				acquiredPrefix = ipam.NetworkAcquire(32, false)
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
				acquiredAddr   *netip.Addr
			)

			BeforeEach(func() {
				acquiredPrefix = ipam.NetworkAcquire(32, false)
				Expect(acquiredPrefix).NotTo(BeNil())
				Expect(ipam.NetworkIsAvailable(*acquiredPrefix)).To(BeFalse())

				acquiredAddr, err = ipam.IPAcquire(*acquiredPrefix)
				Expect(err).NotTo(HaveOccurred())
				Expect(acquiredAddr).NotTo(BeNil())
				Expect(ipam.IPIsAllocated(*acquiredPrefix, *acquiredAddr)).To(BeTrue())
			})

			It("should succeed", func() {
				newTime := time.Now().Add(time.Hour)
				Expect(ipam.IPSetCreationTimestamp(*acquiredAddr, *acquiredPrefix, newTime)).Should(Succeed())
				node := search(*acquiredPrefix, &ipam.roots[0])
				Expect(node).NotTo(BeNil())
				Expect(node.ips).Should(HaveLen(1))
				Expect(node.ips[0].creationTimestamp).To(Equal(newTime))
			})

			It("should not succeed", func() {
				Expect(ipam.IPSetCreationTimestamp(*acquiredAddr, prefixOutOfPools, time.Now())).ShouldNot(Succeed())

				releasedIP, err := ipam.IPRelease(*acquiredPrefix, *acquiredAddr, 0)
				Expect(err).NotTo(HaveOccurred())
				Expect(releasedIP).NotTo(BeNil())
				Expect(ipam.IPIsAllocated(*acquiredPrefix, *acquiredAddr)).To(BeFalse())

				Expect(ipam.IPSetCreationTimestamp(*acquiredAddr, *acquiredPrefix, time.Now())).ShouldNot(Succeed())

				Expect(ipam.NetworkRelease(*acquiredPrefix, 0).String()).To(Equal(acquiredPrefix.String()))
				Expect(ipam.NetworkIsAvailable(*acquiredPrefix)).To(BeTrue())

				Expect(ipam.IPSetCreationTimestamp(*acquiredAddr, *acquiredPrefix, time.Now())).ShouldNot(Succeed())
			})
		})

		When("generating graphviz", func() {
			BeforeEach(func() {
				sizes := []int{21, 26, 27, 22, 30, 25, 28, 24, 16, 10, 29}
				for _, size := range sizes {
					for i := 0; i < 3; i++ {
						network := ipam.NetworkAcquire(size, false)
						Expect(network).ShouldNot(BeNil())
						for j := 0; j < 3; j++ {
							addr, err := ipam.IPAcquire(*network)
							Expect(err).NotTo(HaveOccurred())
							Expect(addr).NotTo(BeNil())
						}
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
				acquiredNetworks = append(acquiredNetworks, *ipam.NetworkAcquire(24, false))
				acquiredNetworks = append(acquiredNetworks, *ipam.NetworkAcquire(25, false))
				acquiredNetworks = append(acquiredNetworks, *ipam.NetworkAcquire(26, false))
				acquiredNetworks = append(acquiredNetworks, *ipam.NetworkAcquire(27, false))
				acquiredNetworks = append(acquiredNetworks, *ipam.NetworkAcquire(28, false))

				networks = ipam.ListNetworks()
				Expect(networks).Should(HaveLen(5))
				for i := range acquiredNetworks {
					Expect(networks).Should(ContainElement(acquiredNetworks[i]))
				}
			})
		})

		When("acquiring networks", func() {
			It("should succeed", func() {
				network := ipam.NetworkAcquire(24, false)
				Expect(network).ShouldNot(BeNil())
				Expect(ipam.NetworkIsAvailable(*network)).To(BeFalse())
			})

			It("should not succeed", func() {
				network := ipam.NetworkAcquire(4, false)
				Expect(network).Should(BeNil())
			})
		})

		When("releasing networks", func() {
			It("should succeed", func() {
				network := ipam.NetworkAcquire(16, false)
				Expect(network).ShouldNot(BeNil())
				Expect(ipam.NetworkIsAvailable(*network)).To(BeFalse())
				Expect(ipam.NetworkRelease(*network, 0).String()).To(Equal(network.String()))
				Expect(ipam.NetworkIsAvailable(*network)).To(BeTrue())
			})

			It("should succeed (using root prefix)", func() {
				network := validPools[0]
				Expect(ipam.NetworkIsAvailable(network)).To(BeTrue())
				Expect(ipam.NetworkAcquireWithPrefix(network, false).String()).To(Equal(network.String()))
				Expect(ipam.NetworkIsAvailable(network)).To(BeFalse())
				Expect(ipam.NetworkRelease(network, 0).String()).To(Equal(network.String()))
				Expect(ipam.NetworkIsAvailable(network)).To(BeTrue())
			})

			It("should succeed (with grace period expired)", func() {
				gracePeriod := time.Second * 5
				network := ipam.NetworkAcquire(16, false)
				Expect(ipam.NetworkSetLastUpdateTimestamp(*network, time.Now().Add(-gracePeriod))).Should(Succeed())
				Expect(network).ShouldNot(BeNil())
				Expect(ipam.NetworkIsAvailable(*network)).To(BeFalse())

				Expect(ipam.NetworkRelease(*network, gracePeriod).String()).To(Equal(network.String()))
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

			It("should not succeed (with allocated subnet)", func() {
				subnetwork := netip.MustParsePrefix("10.0.0.0/30")
				Expect(ipam.NetworkIsAvailable(subnetwork)).To(BeTrue())
				Expect(ipam.NetworkAcquireWithPrefix(subnetwork, false)).NotTo(BeNil())
				Expect(ipam.NetworkIsAvailable(subnetwork)).To(BeFalse())

				network := netip.MustParsePrefix("10.0.0.0/24")
				Expect(ipam.NetworkIsAvailable(network)).To(BeFalse())
				Expect(ipam.NetworkRelease(network, 0)).To(BeNil())
				Expect(ipam.NetworkIsAvailable(network)).To(BeFalse())
			})

			It("should not succeed (with root prefix)", func() {
				network := validPools[0]
				Expect(ipam.NetworkIsAvailable(network)).To(BeTrue())
				Expect(ipam.NetworkRelease(network, 0)).To(BeNil())
				Expect(ipam.NetworkIsAvailable(network)).To(BeTrue())
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
					networkAcquired := ipam.NetworkAcquireWithPrefix(network, false)
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
					Expect(ipam.NetworkAcquireWithPrefix(network, false)).To(BeNil())
				}
			})
		})

		When("acquiring a network", func() {
			BeforeEach(func() {
				prefix := netip.MustParsePrefix("10.0.0.0/16")
				Expect(ipam.NetworkAcquireWithPrefix(prefix, false)).NotTo(BeNil())
				Expect(ipam.NetworkIsAvailable(prefix)).To(BeFalse())

				prefix = netip.MustParsePrefix("10.5.0.0/16")
				Expect(ipam.NetworkAcquireWithPrefix(prefix, false)).NotTo(BeNil())
				Expect(ipam.NetworkIsAvailable(prefix)).To(BeFalse())
			})

			It("parent networks should not be available but non-exclusive acquire succeeds (overlapping)", func() {
				prefixes := []netip.Prefix{
					netip.MustParsePrefix("10.0.0.0/14"),
					netip.MustParsePrefix("10.0.0.0/15"),
					netip.MustParsePrefix("10.4.0.0/15"),
					netip.MustParsePrefix("10.4.0.0/14"),
					netip.MustParsePrefix("10.0.0.0/13"),
				}
				for _, prefix := range prefixes {
					Expect(ipam.NetworkIsAvailable(prefix)).To(BeFalse())
					Expect(ipam.NetworkAcquireWithPrefix(prefix, false)).NotTo(BeNil())
					Expect(ipam.NetworkIsAvailable(prefix)).To(BeFalse())
				}
			})

			It("parent networks should not be acquirable exclusively when children exist", func() {
				prefixes := []netip.Prefix{
					netip.MustParsePrefix("10.0.0.0/14"),
					netip.MustParsePrefix("10.0.0.0/15"),
					netip.MustParsePrefix("10.4.0.0/15"),
					netip.MustParsePrefix("10.4.0.0/14"),
					netip.MustParsePrefix("10.0.0.0/13"),
				}
				for _, prefix := range prefixes {
					Expect(ipam.NetworkAcquireWithPrefix(prefix, true)).To(BeNil())
				}
			})

			It("child networks should not be available but non-exclusive acquire succeeds (overlapping)", func() {
				children := []netip.Prefix{
					netip.MustParsePrefix("10.0.0.0/17"),
					netip.MustParsePrefix("10.0.0.0/18"),
					netip.MustParsePrefix("10.5.0.0/20"),
				}
				for _, prefix := range children {
					Expect(ipam.NetworkIsAvailable(prefix)).To(BeFalse())
					Expect(ipam.NetworkAcquireWithPrefix(prefix, false)).NotTo(BeNil())
					Expect(ipam.NetworkIsAvailable(prefix)).To(BeFalse())
				}

				outOfPool := []netip.Prefix{
					netip.MustParsePrefix("11.0.0.0/16"),
					netip.MustParsePrefix("0.0.0.0/0"),
				}
				for _, prefix := range outOfPool {
					Expect(ipam.NetworkIsAvailable(prefix)).To(BeFalse())
					Expect(ipam.NetworkAcquireWithPrefix(prefix, false)).To(BeNil())
					Expect(ipam.NetworkIsAvailable(prefix)).To(BeFalse())
				}
			})
		})
	})

	Context("Overlapping network acquisitions", func() {
		BeforeEach(func() {
			var err error
			ipam, err = NewIpam(validPools)
			Expect(err).NotTo(HaveOccurred())
		})

		When("acquiring the same prefix twice with overlapping mode", func() {
			It("should succeed and increment refcount", func() {
				prefix := netip.MustParsePrefix("10.20.0.0/16")

				r1 := ipam.NetworkAcquireWithPrefix(prefix, false)
				Expect(r1).NotTo(BeNil())
				Expect(r1.String()).To(Equal("10.20.0.0/16"))

				r2 := ipam.NetworkAcquireWithPrefix(prefix, false)
				Expect(r2).NotTo(BeNil())
				Expect(r2.String()).To(Equal("10.20.0.0/16"))

				Expect(ipam.NetworkIsAvailable(prefix)).To(BeFalse())

				// First release decrements refcount but does not free
				Expect(ipam.NetworkRelease(prefix, 0)).NotTo(BeNil())
				Expect(ipam.NetworkIsAvailable(prefix)).To(BeFalse())

				// Second release frees the prefix
				Expect(ipam.NetworkRelease(prefix, 0)).NotTo(BeNil())
				Expect(ipam.NetworkIsAvailable(prefix)).To(BeTrue())
			})
		})

		When("double-releasing a shared network", func() {
			It("should be a no-op after refcount reaches zero", func() {
				prefix := netip.MustParsePrefix("10.20.0.0/16")

				Expect(ipam.NetworkAcquireWithPrefix(prefix, false)).NotTo(BeNil())

				// First release frees it
				Expect(ipam.NetworkRelease(prefix, 0)).NotTo(BeNil())
				Expect(ipam.NetworkIsAvailable(prefix)).To(BeTrue())

				// Second release is a no-op (refCount stays at 0, not -1)
				Expect(ipam.NetworkRelease(prefix, 0)).To(BeNil())
				Expect(ipam.NetworkIsAvailable(prefix)).To(BeTrue())

				// Must still be acquirable as shared (proves it didn't become exclusive)
				Expect(ipam.NetworkAcquireWithPrefix(prefix, false)).NotTo(BeNil())
				Expect(ipam.NetworkAcquireWithPrefix(prefix, false)).NotTo(BeNil())
			})
		})

		When("acquiring a parent prefix after a child is already acquired", func() {
			It("should succeed via non-exclusive (overlapping) mode", func() {
				child := netip.MustParsePrefix("10.20.0.0/16")
				parent := netip.MustParsePrefix("10.0.0.0/8")

				Expect(ipam.NetworkAcquireWithPrefix(child, false)).NotTo(BeNil())
				Expect(ipam.NetworkIsAvailable(child)).To(BeFalse())

				r := ipam.NetworkAcquireWithPrefix(parent, false)
				Expect(r).NotTo(BeNil())
				Expect(r.String()).To(Equal("10.0.0.0/8"))

				// Both should be in the list
				networks := ipam.ListNetworks()
				Expect(networks).To(ContainElement(child))
				Expect(networks).To(ContainElement(parent))
			})
		})

		When("acquiring a child prefix after a parent is already acquired", func() {
			It("should succeed via overlapping mode", func() {
				parent := netip.MustParsePrefix("10.20.0.0/16")
				child := netip.MustParsePrefix("10.20.0.0/17")

				Expect(ipam.NetworkAcquireWithPrefix(parent, false)).NotTo(BeNil())
				Expect(ipam.NetworkIsAvailable(parent)).To(BeFalse())

				// Overlapping acquire of child should succeed (splits the parent node)
				r := ipam.NetworkAcquireWithPrefix(child, false)
				Expect(r).NotTo(BeNil())
				Expect(r.String()).To(Equal("10.20.0.0/17"))

				// Both should be listed
				networks := ipam.ListNetworks()
				Expect(networks).To(ContainElement(parent))
				Expect(networks).To(ContainElement(child))
			})
		})

		When("releasing overlapping prefixes independently", func() {
			It("should release parent without affecting child", func() {
				child := netip.MustParsePrefix("10.20.0.0/16")
				parent := netip.MustParsePrefix("10.0.0.0/8")

				Expect(ipam.NetworkAcquireWithPrefix(child, false)).NotTo(BeNil())
				Expect(ipam.NetworkAcquireWithPrefix(parent, false)).NotTo(BeNil())

				// Release parent
				Expect(ipam.NetworkRelease(parent, 0)).NotTo(BeNil())

				// Child should still be acquired
				Expect(ipam.NetworkIsAvailable(child)).To(BeFalse())
				networks := ipam.ListNetworks()
				Expect(networks).To(ContainElement(child))
				Expect(networks).NotTo(ContainElement(parent))
			})

			It("should release child without affecting parent", func() {
				child := netip.MustParsePrefix("10.20.0.0/16")
				parent := netip.MustParsePrefix("10.0.0.0/8")

				Expect(ipam.NetworkAcquireWithPrefix(child, false)).NotTo(BeNil())
				Expect(ipam.NetworkAcquireWithPrefix(parent, false)).NotTo(BeNil())

				// Release child
				Expect(ipam.NetworkRelease(child, 0)).NotTo(BeNil())

				// Parent should still be acquired
				networks := ipam.ListNetworks()
				Expect(networks).To(ContainElement(parent))
				Expect(networks).NotTo(ContainElement(child))
			})
		})

		When("listing networks with overlapping acquisitions", func() {
			It("should list both parent and child", func() {
				child1 := netip.MustParsePrefix("10.20.0.0/16")
				child2 := netip.MustParsePrefix("10.96.0.0/12")
				parent := netip.MustParsePrefix("10.0.0.0/8")

				Expect(ipam.NetworkAcquireWithPrefix(child1, false)).NotTo(BeNil())
				Expect(ipam.NetworkAcquireWithPrefix(child2, false)).NotTo(BeNil())
				Expect(ipam.NetworkAcquireWithPrefix(parent, false)).NotTo(BeNil())

				networks := ipam.ListNetworks()
				Expect(networks).To(ContainElement(child1))
				Expect(networks).To(ContainElement(child2))
				Expect(networks).To(ContainElement(parent))
			})
		})

		When("mutable allocation should not use ref-counted space", func() {
			It("should not allocate within an overlapping-acquired parent", func() {
				child := netip.MustParsePrefix("10.20.0.0/16")
				parent := netip.MustParsePrefix("10.0.0.0/8")

				Expect(ipam.NetworkAcquireWithPrefix(child, false)).NotTo(BeNil())
				Expect(ipam.NetworkAcquireWithPrefix(parent, false)).NotTo(BeNil())

				// Mutable allocation of a /16 should NOT come from within 10.0.0.0/8
				// because the /8 is ref-counted. It should come from another pool.
				network := ipam.NetworkAcquire(16, false)
				Expect(network).NotTo(BeNil())
				// The result should be from 192.168.x or 172.16.x (other pools), not from 10.x
				Expect(network.Overlaps(parent)).To(BeFalse(),
					"mutable allocation %s should not overlap with ref-counted parent %s", network, parent)
			})
		})
	})

	Context("Exclusive network acquisitions", func() {
		BeforeEach(func() {
			var err error
			ipam, err = NewIpam(validPools)
			Expect(err).NotTo(HaveOccurred())
		})

		When("acquiring a prefix exclusively", func() {
			It("should succeed on a free prefix", func() {
				prefix := netip.MustParsePrefix("10.50.0.0/16")
				r := ipam.NetworkAcquireWithPrefix(prefix, true)
				Expect(r).NotTo(BeNil())
				Expect(r.String()).To(Equal("10.50.0.0/16"))
				Expect(ipam.NetworkIsAvailable(prefix)).To(BeFalse())
			})

			It("should fail if the prefix is already acquired (shared)", func() {
				prefix := netip.MustParsePrefix("10.50.0.0/16")
				Expect(ipam.NetworkAcquireWithPrefix(prefix, false)).NotTo(BeNil())

				r := ipam.NetworkAcquireWithPrefix(prefix, true)
				Expect(r).To(BeNil())
			})

			It("should fail if a descendant is acquired", func() {
				child := netip.MustParsePrefix("10.50.0.0/24")
				Expect(ipam.NetworkAcquireWithPrefix(child, false)).NotTo(BeNil())

				parent := netip.MustParsePrefix("10.50.0.0/16")
				r := ipam.NetworkAcquireWithPrefix(parent, true)
				Expect(r).To(BeNil())
			})

			It("should fail if an ancestor is ref-counted (shared parent blocks exclusive child)", func() {
				parent := netip.MustParsePrefix("10.0.0.0/8")
				Expect(ipam.NetworkAcquireWithPrefix(parent, false)).NotTo(BeNil())

				child := netip.MustParsePrefix("10.50.0.0/16")
				r := ipam.NetworkAcquireWithPrefix(child, true)
				Expect(r).To(BeNil())
			})
		})

		When("an exclusive prefix blocks other acquisitions", func() {
			It("should block overlapping acquire on the same prefix", func() {
				prefix := netip.MustParsePrefix("10.50.0.0/16")
				Expect(ipam.NetworkAcquireWithPrefix(prefix, true)).NotTo(BeNil())

				r := ipam.NetworkAcquireWithPrefix(prefix, false)
				Expect(r).To(BeNil())
			})

			It("should block overlapping acquire on a child prefix", func() {
				parent := netip.MustParsePrefix("10.50.0.0/16")
				Expect(ipam.NetworkAcquireWithPrefix(parent, true)).NotTo(BeNil())

				child := netip.MustParsePrefix("10.50.0.0/24")
				r := ipam.NetworkAcquireWithPrefix(child, false)
				Expect(r).To(BeNil())
			})

			It("should block non-overlapping acquire on a child prefix", func() {
				parent := netip.MustParsePrefix("10.50.0.0/16")
				Expect(ipam.NetworkAcquireWithPrefix(parent, true)).NotTo(BeNil())

				child := netip.MustParsePrefix("10.50.0.0/24")
				r := ipam.NetworkAcquireWithPrefix(child, false)
				Expect(r).To(BeNil())
			})

			It("should block mutable allocation within the exclusive range", func() {
				prefix := netip.MustParsePrefix("10.0.0.0/8")
				Expect(ipam.NetworkAcquireWithPrefix(prefix, true)).NotTo(BeNil())

				network := ipam.NetworkAcquire(16, false)
				Expect(network).NotTo(BeNil())
				Expect(network.Overlaps(prefix)).To(BeFalse(),
					"mutable allocation %s should not overlap with exclusive %s", network, prefix)
			})

			It("should NOT block overlapping acquire on a parent prefix", func() {
				child := netip.MustParsePrefix("10.50.0.0/16")
				Expect(ipam.NetworkAcquireWithPrefix(child, true)).NotTo(BeNil())

				parent := netip.MustParsePrefix("10.0.0.0/8")
				r := ipam.NetworkAcquireWithPrefix(parent, false)
				Expect(r).NotTo(BeNil())
				Expect(r.String()).To(Equal("10.0.0.0/8"))
			})
		})

		When("releasing an exclusive prefix", func() {
			It("should free the prefix completely", func() {
				prefix := netip.MustParsePrefix("10.50.0.0/16")
				Expect(ipam.NetworkAcquireWithPrefix(prefix, true)).NotTo(BeNil())
				Expect(ipam.NetworkIsAvailable(prefix)).To(BeFalse())

				Expect(ipam.NetworkRelease(prefix, 0)).NotTo(BeNil())
				Expect(ipam.NetworkIsAvailable(prefix)).To(BeTrue())
			})

			It("should allow re-acquisition after release", func() {
				prefix := netip.MustParsePrefix("10.50.0.0/16")
				Expect(ipam.NetworkAcquireWithPrefix(prefix, true)).NotTo(BeNil())
				Expect(ipam.NetworkRelease(prefix, 0)).NotTo(BeNil())

				// Should be acquirable again (any mode)
				Expect(ipam.NetworkAcquireWithPrefix(prefix, false)).NotTo(BeNil())
			})
		})

		When("acquiring a network exclusively with mutable fallback", func() {
			It("should allocate a free /16 exclusively", func() {
				r := ipam.NetworkAcquire(16, true)
				Expect(r).NotTo(BeNil())
				Expect(r.Bits()).To(Equal(16))

				// Should be exclusive — overlapping acquire must fail
				r2 := ipam.NetworkAcquireWithPrefix(*r, false)
				Expect(r2).To(BeNil())
			})

			It("should not land in ref-counted space", func() {
				parent := netip.MustParsePrefix("10.0.0.0/8")
				Expect(ipam.NetworkAcquireWithPrefix(parent, false)).NotTo(BeNil())

				r := ipam.NetworkAcquire(16, true)
				Expect(r).NotTo(BeNil())
				Expect(r.Overlaps(parent)).To(BeFalse(),
					"exclusive mutable %s should not overlap with ref-counted %s", r, parent)
			})
		})

		When("listing networks with exclusive acquisitions", func() {
			It("should include exclusive networks in the list", func() {
				exclusive := netip.MustParsePrefix("10.50.0.0/16")
				shared := netip.MustParsePrefix("10.60.0.0/16")

				Expect(ipam.NetworkAcquireWithPrefix(exclusive, true)).NotTo(BeNil())
				Expect(ipam.NetworkAcquireWithPrefix(shared, false)).NotTo(BeNil())

				networks := ipam.ListNetworks()
				Expect(networks).To(ContainElement(exclusive))
				Expect(networks).To(ContainElement(shared))
			})
		})
	})

	Context("Ipam IPs", func() {
		var (
			// WARNING: availableIPs must be a power of 2
			availableIPs         = 256
			prefixAcquired       = netip.MustParsePrefix(fmt.Sprintf("10.0.0.0/%d", int(32-math.Log2(float64(availableIPs)))))
			prefixNotAcquired    = netip.MustParsePrefix(fmt.Sprintf("10.1.0.0/%d", int(32-math.Log2(float64(availableIPs)))))
			subPrefixNotAcquired = netip.MustParsePrefix(fmt.Sprintf("10.1.0.0/%d", int(32-math.Log2(float64(availableIPs)))+1))
		)

		BeforeEach(func() {
			var err error
			ipam, err = NewIpam(validPools)
			Expect(err).NotTo(HaveOccurred())

			Expect(ipam.NetworkIsAvailable(prefixAcquired)).To(BeTrue())
			Expect(ipam.NetworkAcquireWithPrefix(prefixAcquired, false)).NotTo(BeNil())
			Expect(ipam.NetworkIsAvailable(prefixAcquired)).To(BeFalse())
		})

		When("acquiring an IP from not existing network", func() {
			It("should not succeed (out of pools)", func() {
				addr, err := ipam.IPAcquire(prefixOutOfPools)
				Expect(err).To(HaveOccurred())
				Expect(addr).To(BeNil())

				allocated, err := ipam.IPIsAllocated(prefixOutOfPools, prefixOutOfPools.Addr())
				Expect(err).To(HaveOccurred())
				Expect(allocated).To(BeFalse())

				addr, err = ipam.IPAcquireWithAddr(prefixOutOfPools, prefixOutOfPools.Addr())
				Expect(err).To(HaveOccurred())
				Expect(addr).To(BeNil())
			})

			It("should not succeed (prefix not acquired)", func() {
				addr, err := ipam.IPAcquire(prefixNotAcquired)
				Expect(err).NotTo(HaveOccurred())
				Expect(addr).To(BeNil())

				addr, err = ipam.IPAcquireWithAddr(prefixNotAcquired, prefixNotAcquired.Addr())
				Expect(err).NotTo(HaveOccurred())
				Expect(addr).To(BeNil())
			})

			It("should not succeed (prefix not acquired with subprefix acquired)", func() {
				Expect(ipam.NetworkIsAvailable(subPrefixNotAcquired)).To(BeTrue())
				Expect(ipam.NetworkAcquireWithPrefix(subPrefixNotAcquired, false)).NotTo(BeNil())
				Expect(ipam.NetworkIsAvailable(subPrefixNotAcquired)).To(BeFalse())

				addr, err := ipam.IPAcquire(prefixNotAcquired)
				Expect(err).NotTo(HaveOccurred())
				Expect(addr).To(BeNil())

				addr, err = ipam.IPAcquireWithAddr(prefixNotAcquired, prefixNotAcquired.Addr())
				Expect(err).NotTo(HaveOccurred())
				Expect(addr).To(BeNil())
			})
		})

		When("acquiring an IP from existing network", func() {
			It("should succeed", func() {
				allocated, err := ipam.IPIsAllocated(prefixAcquired, prefixAcquired.Addr())
				Expect(err).NotTo(HaveOccurred())
				Expect(allocated).To(BeFalse())

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

			It("should not reallocate the same IP", func() {
				addr, err := ipam.IPAcquire(prefixAcquired)
				Expect(err).NotTo(HaveOccurred())
				Expect(addr).NotTo(BeNil())
				Expect(ipam.IPIsAllocated(prefixAcquired, *addr)).To(BeTrue())

				addr, err = ipam.IPAcquireWithAddr(prefixAcquired, *addr)
				Expect(err).NotTo(HaveOccurred())
				Expect(addr).To(BeNil())
			})

			It("should not allocate an IP from a not coherent prefix", func() {
				addr, err := ipam.IPAcquireWithAddr(prefixAcquired, prefixNotAcquired.Addr())
				Expect(err).To(HaveOccurred())
				Expect(addr).To(BeNil())
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

			It("should not succeed (prefix not coherent with addr)", func() {
				addr, err := ipam.IPAcquireWithAddr(prefixAcquired, prefixNotAcquired.Addr())
				Expect(err).To(HaveOccurred())
				Expect(addr).To(BeNil())
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

			It("should not succeed (grace period not expired)", func() {
				gracePeriod := time.Second * 5

				Expect(ipam.IPIsAllocated(prefixAcquired, prefixAcquired.Addr())).To(BeFalse())
				addr, err := ipam.IPAcquireWithAddr(prefixAcquired, prefixAcquired.Addr())
				Expect(err).NotTo(HaveOccurred())
				Expect(addr).NotTo(BeNil())
				Expect(ipam.IPIsAllocated(prefixAcquired, *addr)).To(BeTrue())

				addr, err = ipam.IPRelease(prefixAcquired, *addr, gracePeriod)
				Expect(err).To(BeNil())
				Expect(addr).To(BeNil())
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

			It("should fail", func() {
				cachedIPs, err := ipam.ListIPs(prefixOutOfPools)
				Expect(err).To(HaveOccurred())
				Expect(cachedIPs).Should(HaveLen(0))
			})
		})

		When("checking if an IP is allocated in a not allocated network", func() {
			It("should return false", func() {
				allocated, err := ipam.IPIsAllocated(prefixNotAcquired, prefixNotAcquired.Addr())
				Expect(err).NotTo(HaveOccurred())
				Expect(allocated).To(BeFalse())
			})
		})
	})
})
