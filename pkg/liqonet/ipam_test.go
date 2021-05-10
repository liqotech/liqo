package liqonet_test

import (
	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"

	"github.com/liqotech/liqo/pkg/liqonet"
)

var ipam *liqonet.IPAM
var dynClient dynamic.Interface

func fillNetworkPool(pool string, ipam *liqonet.IPAM) error {

	// Get halves mask length
	mask := liqonet.GetMask(pool)
	mask += 1

	// Get first half CIDR
	halfCidr, err := liqonet.SetMask(pool, mask)
	if err != nil {
		return err
	}

	err = ipam.AcquireReservedSubnet(halfCidr)
	if err != nil {
		return err
	}

	// Get second half CIDR
	halfCidr, err = liqonet.Next(halfCidr)
	if err != nil {
		return err
	}
	err = ipam.AcquireReservedSubnet(halfCidr)

	return err
}

var _ = Describe("Ipam", func() {

	BeforeEach(func() {
		dynClient = fake.NewSimpleDynamicClient(runtime.NewScheme())
		ipam = liqonet.NewIPAM()
		err := ipam.Init(liqonet.Pools, dynClient)
		gomega.Expect(err).To(gomega.BeNil())
	})

	Describe("AcquireReservedSubnet", func() {
		Context("When the reserved network equals a network pool", func() {
			It("Should successfully reserve the subnet", func() {
				// Reserve network
				err := ipam.AcquireReservedSubnet("10.0.0.0/8")
				gomega.Expect(err).To(gomega.BeNil())
				// Try to get a cluster network in that pool
				p, _, err := ipam.GetSubnetsPerCluster("10.0.2.0/24", "192.168.0.0/24", "cluster1")
				gomega.Expect(err).To(gomega.BeNil())
				// p should have been mapped to a new network belonging to a different pool
				gomega.Expect(p).ToNot(gomega.HavePrefix("10."))
			})
		})
		Context("When the reserved network belongs to a pool", func() {
			It("Should not be possible to acquire the same network for a cluster", func() {
				err := ipam.AcquireReservedSubnet("10.244.0.0/24")
				gomega.Expect(err).To(gomega.BeNil())
				p, e, err := ipam.GetSubnetsPerCluster("10.244.0.0/24", "192.168.0.0/24", "cluster1")
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(p).ToNot(gomega.Equal("10.0.2.0/24"))
				gomega.Expect(e).To(gomega.Equal("192.168.0.0/24"))
			})
			It("Should not be possible to acquire a larger network that contains it for a cluster", func() {
				err := ipam.AcquireReservedSubnet("10.0.0.0/24")
				gomega.Expect(err).To(gomega.BeNil())
				p, e, err := ipam.GetSubnetsPerCluster("10.0.0.0/16", "192.168.0.0/24", "cluster1")
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(p).ToNot(gomega.Equal("10.0.0.0/16"))
				gomega.Expect(e).To(gomega.Equal("192.168.0.0/24"))
			})
			It("Should not be possible to acquire a smaller network contained by it for a cluster", func() {
				err := ipam.AcquireReservedSubnet("10.0.2.0/24")
				gomega.Expect(err).To(gomega.BeNil())
				p, e, err := ipam.GetSubnetsPerCluster("10.0.2.0/25", "192.168.0.0/24", "cluster1")
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(p).ToNot(gomega.Equal("10.0.2.0/25"))
				gomega.Expect(e).To(gomega.Equal("192.168.0.0/24"))
			})
		})
	})

	Describe("GetSubnetsPerCluster", func() {
		Context("When the remote cluster asks for subnets not belonging to any pool", func() {
			Context("and the subnets have not already been assigned to any other cluster", func() {
				It("Should allocate the subnets without mapping", func() {
					p, e, err := ipam.GetSubnetsPerCluster("11.0.0.0/16", "11.1.0.0/16", "cluster1")
					gomega.Expect(err).To(gomega.BeNil())
					gomega.Expect(p).To(gomega.Equal("11.0.0.0/16"))
					gomega.Expect(e).To(gomega.Equal("11.1.0.0/16"))
				})
			})
			Context("and the subnets have already been assigned to another cluster", func() {
				Context("and there are available networks with the same mask length in one pool", func() {
					It("should map the requested networks", func() {
						p, e, err := ipam.GetSubnetsPerCluster("11.0.0.0/16", "11.1.0.0/16", "cluster1")
						gomega.Expect(err).To(gomega.BeNil())
						gomega.Expect(p).To(gomega.Equal("11.0.0.0/16"))
						gomega.Expect(e).To(gomega.Equal("11.1.0.0/16"))
						p, e, err = ipam.GetSubnetsPerCluster("11.0.0.0/16", "11.1.0.0/16", "cluster2")
						gomega.Expect(err).To(gomega.BeNil())
						gomega.Expect(p).ToNot(gomega.HavePrefix("11."))
						gomega.Expect(p).To(gomega.HaveSuffix("/16"))
						gomega.Expect(e).ToNot(gomega.HavePrefix("11."))
						gomega.Expect(e).To(gomega.HaveSuffix("/16"))
					})
				})
			})
		})
		Context("When the remote cluster asks for a subnet which is equal to a pool", func() {
			Context("and remaining network pools are not filled", func() {
				It("should map it to another network", func() {
					p, _, err := ipam.GetSubnetsPerCluster("172.16.0.0/12", "10.0.0.0/24", "cluster1")
					gomega.Expect(err).To(gomega.BeNil())
					gomega.Expect(p).ToNot(gomega.Equal("172.16.0.0/12"))
				})
			})
			Context("and remaining network pools are filled", func() {
				Context("and one of the 2 halves of the pool cannot be reserved", func() {
					It("should not allocate any network", func() {
						// Fill pool #1
						err := fillNetworkPool(liqonet.Pools[0], ipam)
						gomega.Expect(err).To(gomega.BeNil())

						// Fill pool #2
						err = fillNetworkPool(liqonet.Pools[1], ipam)
						gomega.Expect(err).To(gomega.BeNil())

						// Acquire a portion of the network pool
						p, e, err := ipam.GetSubnetsPerCluster("172.16.0.0/24", "172.16.1.0/24", "cluster5")
						gomega.Expect(err).To(gomega.BeNil())
						gomega.Expect(p).To(gomega.Equal("172.16.0.0/24"))
						gomega.Expect(e).To(gomega.Equal("172.16.1.0/24"))

						// Acquire network pool
						_, _, err = ipam.GetSubnetsPerCluster("172.16.0.0/12", "10.0.0.0/24", "cluster6")
						gomega.Expect(err).ToNot(gomega.BeNil())
					})
				})
			})
		})
		Context("When the remote cluster asks for a subnet belonging to a network in the pool", func() {
			Context("and all pools are full", func() {
				It("should not allocate the network (externalCidr not available: podCidr requested should be available after the call)", func() {
					// Fill pool #2
					err := fillNetworkPool(liqonet.Pools[1], ipam)
					gomega.Expect(err).To(gomega.BeNil())

					// Fill pool #3
					err = fillNetworkPool(liqonet.Pools[2], ipam)
					gomega.Expect(err).To(gomega.BeNil())

					// Fill 1st half of pool #1
					err = ipam.AcquireReservedSubnet("10.0.0.0/9")
					gomega.Expect(err).To(gomega.BeNil())

					// Cluster network request
					_, _, err = ipam.GetSubnetsPerCluster("10.128.0.0/9", "192.168.1.0/24", "cluster7")
					gomega.Expect(err).ToNot(gomega.BeNil())

					// Check if requested podCidr is available
					err = ipam.AcquireReservedSubnet("10.128.0.0/9")
					gomega.Expect(err).To(gomega.BeNil())
				})
				It("should not allocate the network (both)", func() {
					// Fill pool #1
					err := fillNetworkPool(liqonet.Pools[0], ipam)
					gomega.Expect(err).To(gomega.BeNil())

					// Fill pool #2
					err = fillNetworkPool(liqonet.Pools[1], ipam)
					gomega.Expect(err).To(gomega.BeNil())

					// Fill pool #3
					err = fillNetworkPool(liqonet.Pools[2], ipam)
					gomega.Expect(err).To(gomega.BeNil())

					// Cluster network request
					_, _, err = ipam.GetSubnetsPerCluster("10.1.0.0/16", "10.0.0.0/24", "cluster7")
					gomega.Expect(err).ToNot(gomega.BeNil())
				})
			})
			Context("and the subnet has not already been assigned to any other cluster", func() {
				It("Should allocate the subnet itself, without mapping", func() {
					p, e, err := ipam.GetSubnetsPerCluster("10.0.0.0/16", "10.1.0.0/16", "cluster1")
					gomega.Expect(err).To(gomega.BeNil())
					gomega.Expect(p).To(gomega.Equal("10.0.0.0/16"))
					gomega.Expect(e).To(gomega.Equal("10.1.0.0/16"))
				})
			})
			Context("and the subnet has already been assigned to another cluster", func() {
				Context("and there is an available network with the same mask length in one pool", func() {
					It("should map the requested network to another network taken by the pool", func() {
						p, e, err := ipam.GetSubnetsPerCluster("10.0.0.0/16", "10.1.0.0/16", "cluster1")
						gomega.Expect(err).To(gomega.BeNil())
						gomega.Expect(p).To(gomega.Equal("10.0.0.0/16"))
						gomega.Expect(e).To(gomega.Equal("10.1.0.0/16"))
						p, e, err = ipam.GetSubnetsPerCluster("10.0.0.0/16", "10.1.0.0/16", "clustere")
						gomega.Expect(err).To(gomega.BeNil())
						gomega.Expect(p).ToNot(gomega.Equal("10.0.0.0/16"))
						gomega.Expect(e).ToNot(gomega.Equal("10.1.0.0/16"))
					})
				})
				Context("and there is not an available network with the same mask length in any pool", func() {
					It("should fail to allocate the network", func() {

						p, _, err := ipam.GetSubnetsPerCluster("10.0.0.0/9", "10.1.0.0/16", "cluster1")
						gomega.Expect(err).To(gomega.BeNil())
						gomega.Expect(p).To(gomega.Equal("10.0.0.0/9"))

						_, _, err = ipam.GetSubnetsPerCluster("10.0.0.0/9", "10.3.0.0/16", "cluster2")
						gomega.Expect(err).ToNot(gomega.BeNil())
					})
				})
			})
		})
	})
	Describe("FreeSubnetPerCluster", func() {
		Context("Freeing cluster networks that exist", func() {
			It("Should successfully free the subnets", func() {
				p, e, err := ipam.GetSubnetsPerCluster("10.0.1.0/24", "10.0.2.0/24", "cluster1")
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(p).To(gomega.Equal("10.0.1.0/24"))
				gomega.Expect(e).To(gomega.Equal("10.0.2.0/24"))
				err = ipam.FreeSubnetsPerCluster("cluster1")
				gomega.Expect(err).To(gomega.BeNil())
				p, e, err = ipam.GetSubnetsPerCluster("10.0.1.0/24", "10.0.2.0/24", "cluster1")
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(p).To(gomega.Equal("10.0.1.0/24"))
				gomega.Expect(e).To(gomega.Equal("10.0.2.0/24"))
			})
		})
		Context("Freeing a cluster network that does not exists", func() {
			It("Should return no errors", func() {
				err := ipam.FreeSubnetsPerCluster("cluster1")
				gomega.Expect(err).To(gomega.BeNil())
			})
		})
	})
	Describe("FreeReservedSubnet", func() {
		Context("Freeing a network that has been reserved previously", func() {
			It("Should successfully free the subnet", func() {
				err := ipam.AcquireReservedSubnet("10.0.1.0/24")
				gomega.Expect(err).To(gomega.BeNil())
				err = ipam.FreeReservedSubnet("10.0.1.0/24")
				gomega.Expect(err).To(gomega.BeNil())
				err = ipam.AcquireReservedSubnet("10.0.1.0/24")
				gomega.Expect(err).To(gomega.BeNil())
			})
		})
		Context("Freeing a cluster network that does not exists", func() {
			It("Should return no errors", func() {
				err := ipam.FreeReservedSubnet("10.0.1.0/24")
				gomega.Expect(err).To(gomega.BeNil())
			})
		})
		Context("Freeing a reserved subnet equal to a network pool", func() {
			It("Should make available the network pool", func() {
				err := ipam.AcquireReservedSubnet("10.0.0.0/8")
				gomega.Expect(err).To(gomega.BeNil())
				err = ipam.FreeReservedSubnet("10.0.0.0/8")
				gomega.Expect(err).To(gomega.BeNil())
				p, e, err := ipam.GetSubnetsPerCluster("10.0.0.0/16", "10.2.0.0/16", "cluster1")
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(p).To(gomega.Equal("10.0.0.0/16"))
				gomega.Expect(e).To(gomega.Equal("10.2.0.0/16"))
			})
		})
	})
	Describe("Re-scheduling of network manager", func() {
		It("ipam should retrieve configuration by resource", func() {
			// Assign networks to cluster
			p, e, err := ipam.GetSubnetsPerCluster("10.0.1.0/24", "10.0.2.0/24", "cluster1")
			gomega.Expect(err).To(gomega.BeNil())
			gomega.Expect(p).To(gomega.Equal("10.0.1.0/24"))
			gomega.Expect(e).To(gomega.Equal("10.0.2.0/24"))

			// Simulate re-scheduling
			ipam = liqonet.NewIPAM()
			err = ipam.Init(liqonet.Pools, dynClient)
			gomega.Expect(err).To(gomega.BeNil())

			// Another cluster asks for the same networks
			p, e, err = ipam.GetSubnetsPerCluster("10.0.1.0/24", "10.0.2.0/24", "cluster2")
			gomega.Expect(err).To(gomega.BeNil())
			gomega.Expect(p).ToNot(gomega.Equal("10.0.1.0/24"))
			gomega.Expect(e).ToNot(gomega.Equal("10.0.2.0/24"))
		})
	})
	Describe("AddNetworkPool", func() {
		Context("Trying to add a default network pool", func() {
			It("Should generate an error", func() {
				err := ipam.AddNetworkPool("10.0.0.0/8")
				gomega.Expect(err).ToNot(gomega.BeNil())
			})
		})
		Context("Trying to add twice the same network pool", func() {
			It("Should generate an error", func() {
				err := ipam.AddNetworkPool("11.0.0.0/8")
				gomega.Expect(err).To(gomega.BeNil())
				err = ipam.AddNetworkPool("11.0.0.0/8")
				gomega.Expect(err).ToNot(gomega.BeNil())
			})
		})
		Context("After adding a new network pool", func() {
			It("Should be possible to use that pool for cluster networks", func() {
				// Reserve default network pools
				for _, network := range liqonet.Pools {
					err := fillNetworkPool(network, ipam)
					gomega.Expect(err).To(gomega.BeNil())
				}

				// Add new network pool
				err := ipam.AddNetworkPool("11.0.0.0/8")
				gomega.Expect(err).To(gomega.BeNil())

				// Reserve a given network
				err = ipam.AcquireReservedSubnet("12.0.0.0/24")
				gomega.Expect(err).To(gomega.BeNil())

				// Reserve a given network
				err = ipam.AcquireReservedSubnet("12.0.1.0/24")
				gomega.Expect(err).To(gomega.BeNil())

				// IPAM should use 11.0.0.0/8 to map the cluster network
				p, e, err := ipam.GetSubnetsPerCluster("12.0.0.0/24", "12.0.1.0/24", "cluster1")
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(p).To(gomega.HavePrefix("11"))
				gomega.Expect(p).To(gomega.HaveSuffix("/24"))
				gomega.Expect(e).To(gomega.HavePrefix("11"))
				gomega.Expect(e).To(gomega.HaveSuffix("/24"))
			})
		})
		Context("Trying to add a network pool that overlaps with a reserved network", func() {
			It("Should generate an error", func() {
				err := ipam.AcquireReservedSubnet("11.0.0.0/8")
				gomega.Expect(err).To(gomega.BeNil())
				err = ipam.AddNetworkPool("11.0.0.0/16")
				gomega.Expect(err).ToNot(gomega.BeNil())
			})
		})
	})
	Describe("RemoveNetworkPool", func() {
		Context("Remove a network pool that does not exist", func() {
			It("Should return an error", func() {
				err := ipam.RemoveNetworkPool("11.0.0.0/8")
				gomega.Expect(err).ToNot(gomega.BeNil())
			})
		})
		Context("Remove a network pool that exists", func() {
			It("Should successfully remove the network pool", func() {
				// Reserve default network pools
				for _, network := range liqonet.Pools {
					err := ipam.AcquireReservedSubnet(network)
					gomega.Expect(err).To(gomega.BeNil())
				}

				// Add new network pool
				err := ipam.AddNetworkPool("11.0.0.0/8")
				gomega.Expect(err).To(gomega.BeNil())

				// Remove network pool
				err = ipam.RemoveNetworkPool("11.0.0.0/8")
				gomega.Expect(err).To(gomega.BeNil())

				// Reserve a given network
				err = ipam.AcquireReservedSubnet("12.0.0.0/24")
				gomega.Expect(err).To(gomega.BeNil())

				// Reserve a given network
				err = ipam.AcquireReservedSubnet("12.0.1.0/24")
				gomega.Expect(err).To(gomega.BeNil())

				// Should fail to assign a network to cluster
				_, _, err = ipam.GetSubnetsPerCluster("12.0.0.0/24", "12.0.1.0/24", "cluster1")
				gomega.Expect(err).ToNot(gomega.BeNil())
			})
		})
		Context("Remove a network pool that is a default one", func() {
			It("Should generate an error", func() {
				err := ipam.RemoveNetworkPool("10.0.0.0/8")
				gomega.Expect(err).ToNot(gomega.BeNil())
			})
		})
		Context("Remove a network pool that is used for a cluster", func() {
			It("Should generate an error", func() {
				// Reserve default network pools
				for _, network := range liqonet.Pools {
					err := ipam.AcquireReservedSubnet(network)
					gomega.Expect(err).To(gomega.BeNil())
				}

				// Add new network pool
				err := ipam.AddNetworkPool("11.0.0.0/8")
				gomega.Expect(err).To(gomega.BeNil())

				// Reserve a network
				err = ipam.AcquireReservedSubnet("12.0.0.0/24")
				gomega.Expect(err).To(gomega.BeNil())

				// Reserve a network
				err = ipam.AcquireReservedSubnet("12.0.1.0/24")
				gomega.Expect(err).To(gomega.BeNil())

				// IPAM should use 11.0.0.0/8 to map the cluster network
				p, e, err := ipam.GetSubnetsPerCluster("12.0.0.0/24", "12.0.1.0/24", "cluster1")
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(p).To(gomega.HavePrefix("11"))
				gomega.Expect(p).To(gomega.HaveSuffix("/24"))
				gomega.Expect(e).To(gomega.HavePrefix("11"))
				gomega.Expect(e).To(gomega.HaveSuffix("/24"))

				err = ipam.RemoveNetworkPool("11.0.0.0/8")
				gomega.Expect(err).ToNot(gomega.BeNil())
			})
		})
	})
	Describe("AddExternalCIDRPerCluster", func() {
		Context("If the remote externalCIDR does not exist yet", func() {
			It("should return no errors", func() {
				err := ipam.AddExternalCIDRPerCluster("10.0.0.0/24", "cluster1")
				gomega.Expect(err).To(gomega.BeNil())
			})
		})
		Context("If the remote ExternalCIDR already exists", func() {
			It("should return no errors", func() {
				err := ipam.AddExternalCIDRPerCluster("10.0.0.0/24", "cluster1")
				gomega.Expect(err).To(gomega.BeNil())
				err = ipam.AddExternalCIDRPerCluster("10.0.0.0/24", "cluster1")
				gomega.Expect(err).To(gomega.BeNil())
			})
		})
	})

	Describe("RemoveExternalCIDRPerCluster", func() {
		Context("If the remote externalCIDR does not exist", func() {
			It("should return no errors", func() {
				err := ipam.RemoveExternalCIDRPerCluster("cluster1")
				gomega.Expect(err).To(gomega.BeNil())
			})
		})
		Context("If the remote ExternalCIDR already exists", func() {
			It("should return no errors", func() {
				err := ipam.AddExternalCIDRPerCluster("10.0.0.0/24", "cluster1")
				gomega.Expect(err).To(gomega.BeNil())
				err = ipam.RemoveExternalCIDRPerCluster("cluster1")
				gomega.Expect(err).To(gomega.BeNil())
			})
		})
	})
})
