package liqonet_test

import (
	"context"
	"fmt"
	"strings"

	"math/rand"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"

	"github.com/liqotech/liqo/pkg/liqonet"

	liqonetapi "github.com/liqotech/liqo/apis/net/v1alpha1"
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
	rand.Seed(1)

	BeforeEach(func() {
		scheme := runtime.NewScheme()
		scheme.AddKnownTypeWithName(schema.GroupVersionKind{
			Group:   "net.liqo.io",
			Version: "v1alpha1",
			Kind:    "ipamstorages",
		}, &liqonetapi.IpamStorage{})
		s := schema.GroupVersionResource{
			Group:    "net.liqo.io",
			Version:  "v1alpha1",
			Resource: "ipamstorages",
		}
		var m = make(map[schema.GroupVersionResource]string)
		m[s] = "ipamstoragesList"
		dynClient = fake.NewSimpleDynamicClientWithCustomListKinds(scheme, m, &liqonetapi.IpamStorage{})
		ipam = liqonet.NewIPAM()
		err := ipam.Init(liqonet.Pools, dynClient, 2000+rand.Intn(2000))
		Expect(err).To(BeNil())
	})
	AfterEach(func() {
		ipam.StopGRPCServer()
	})

	Describe("AcquireReservedSubnet", func() {
		Context("When the reserved network equals a network pool", func() {
			It("Should successfully reserve the subnet", func() {
				// Reserve network
				err := ipam.AcquireReservedSubnet("10.0.0.0/8")
				Expect(err).To(BeNil())
				// Try to get a cluster network in that pool
				p, _, err := ipam.GetSubnetsPerCluster("10.0.2.0/24", "192.168.0.0/24", "cluster1")
				Expect(err).To(BeNil())
				// p should have been mapped to a new network belonging to a different pool
				Expect(p).ToNot(HavePrefix("10."))
			})
		})
		Context("When the reserved network belongs to a pool", func() {
			It("Should not be possible to acquire the same network for a cluster", func() {
				err := ipam.AcquireReservedSubnet("10.244.0.0/24")
				Expect(err).To(BeNil())
				p, e, err := ipam.GetSubnetsPerCluster("10.244.0.0/24", "192.168.0.0/24", "cluster1")
				Expect(err).To(BeNil())
				Expect(p).ToNot(Equal("10.0.2.0/24"))
				Expect(e).To(Equal("192.168.0.0/24"))
			})
			It("Should not be possible to acquire a larger network that contains it for a cluster", func() {
				err := ipam.AcquireReservedSubnet("10.0.0.0/24")
				Expect(err).To(BeNil())
				p, e, err := ipam.GetSubnetsPerCluster("10.0.0.0/16", "192.168.0.0/24", "cluster1")
				Expect(err).To(BeNil())
				Expect(p).ToNot(Equal("10.0.0.0/16"))
				Expect(e).To(Equal("192.168.0.0/24"))
			})
			It("Should not be possible to acquire a smaller network contained by it for a cluster", func() {
				err := ipam.AcquireReservedSubnet("10.0.2.0/24")
				Expect(err).To(BeNil())
				p, e, err := ipam.GetSubnetsPerCluster("10.0.2.0/25", "192.168.0.0/24", "cluster1")
				Expect(err).To(BeNil())
				Expect(p).ToNot(Equal("10.0.2.0/25"))
				Expect(e).To(Equal("192.168.0.0/24"))
			})
		})
	})

	Describe("GetSubnetsPerCluster", func() {
		Context("When the remote cluster asks for subnets not belonging to any pool", func() {
			Context("and the subnets have not already been assigned to any other cluster", func() {
				It("Should allocate the subnets without mapping", func() {
					p, e, err := ipam.GetSubnetsPerCluster("11.0.0.0/16", "11.1.0.0/16", "cluster1")
					Expect(err).To(BeNil())
					Expect(p).To(Equal("11.0.0.0/16"))
					Expect(e).To(Equal("11.1.0.0/16"))
				})
			})
			Context("and the subnets have already been assigned to another cluster", func() {
				Context("and there are available networks with the same mask length in one pool", func() {
					It("should map the requested networks", func() {
						p, e, err := ipam.GetSubnetsPerCluster("11.0.0.0/16", "11.1.0.0/16", "cluster1")
						Expect(err).To(BeNil())
						Expect(p).To(Equal("11.0.0.0/16"))
						Expect(e).To(Equal("11.1.0.0/16"))
						p, e, err = ipam.GetSubnetsPerCluster("11.0.0.0/16", "11.1.0.0/16", "cluster2")
						Expect(err).To(BeNil())
						Expect(p).ToNot(HavePrefix("11."))
						Expect(p).To(HaveSuffix("/16"))
						Expect(e).ToNot(HavePrefix("11."))
						Expect(e).To(HaveSuffix("/16"))
					})
				})
			})
		})
		Context("When the remote cluster asks for a subnet which is equal to a pool", func() {
			Context("and remaining network pools are not filled", func() {
				It("should map it to another network", func() {
					p, _, err := ipam.GetSubnetsPerCluster("172.16.0.0/12", "10.0.0.0/24", "cluster1")
					Expect(err).To(BeNil())
					Expect(p).ToNot(Equal("172.16.0.0/12"))
				})
			})
			Context("and remaining network pools are filled", func() {
				Context("and one of the 2 halves of the pool cannot be reserved", func() {
					It("should not allocate any network", func() {
						// Fill pool #1
						err := fillNetworkPool(liqonet.Pools[0], ipam)
						Expect(err).To(BeNil())

						// Fill pool #2
						err = fillNetworkPool(liqonet.Pools[1], ipam)
						Expect(err).To(BeNil())

						// Acquire a portion of the network pool
						p, e, err := ipam.GetSubnetsPerCluster("172.16.0.0/24", "172.16.1.0/24", "cluster5")
						Expect(err).To(BeNil())
						Expect(p).To(Equal("172.16.0.0/24"))
						Expect(e).To(Equal("172.16.1.0/24"))

						// Acquire network pool
						_, _, err = ipam.GetSubnetsPerCluster("172.16.0.0/12", "10.0.0.0/24", "cluster6")
						Expect(err).ToNot(BeNil())
					})
				})
			})
		})
		Context("When the remote cluster asks for a subnet belonging to a network in the pool", func() {
			Context("and all pools are full", func() {
				It("should not allocate the network (externalCidr not available: podCidr requested should be available after the call)", func() {
					// Fill pool #2
					err := fillNetworkPool(liqonet.Pools[1], ipam)
					Expect(err).To(BeNil())

					// Fill pool #3
					err = fillNetworkPool(liqonet.Pools[2], ipam)
					Expect(err).To(BeNil())

					// Fill 1st half of pool #1
					err = ipam.AcquireReservedSubnet("10.0.0.0/9")
					Expect(err).To(BeNil())

					// Cluster network request
					_, _, err = ipam.GetSubnetsPerCluster("10.128.0.0/9", "192.168.1.0/24", "cluster7")
					Expect(err).ToNot(BeNil())

					// Check if requested podCidr is available
					err = ipam.AcquireReservedSubnet("10.128.0.0/9")
					Expect(err).To(BeNil())
				})
				It("should not allocate the network (both)", func() {
					// Fill pool #1
					err := fillNetworkPool(liqonet.Pools[0], ipam)
					Expect(err).To(BeNil())

					// Fill pool #2
					err = fillNetworkPool(liqonet.Pools[1], ipam)
					Expect(err).To(BeNil())

					// Fill pool #3
					err = fillNetworkPool(liqonet.Pools[2], ipam)
					Expect(err).To(BeNil())

					// Cluster network request
					_, _, err = ipam.GetSubnetsPerCluster("10.1.0.0/16", "10.0.0.0/24", "cluster7")
					Expect(err).ToNot(BeNil())
				})
			})
			Context("and the subnet has not already been assigned to any other cluster", func() {
				It("Should allocate the subnet itself, without mapping", func() {
					p, e, err := ipam.GetSubnetsPerCluster("10.0.0.0/16", "10.1.0.0/16", "cluster1")
					Expect(err).To(BeNil())
					Expect(p).To(Equal("10.0.0.0/16"))
					Expect(e).To(Equal("10.1.0.0/16"))
				})
			})
			Context("and the subnet has already been assigned to another cluster", func() {
				Context("and there is an available network with the same mask length in one pool", func() {
					It("should map the requested network to another network taken by the pool", func() {
						p, e, err := ipam.GetSubnetsPerCluster("10.0.0.0/16", "10.1.0.0/16", "cluster1")
						Expect(err).To(BeNil())
						Expect(p).To(Equal("10.0.0.0/16"))
						Expect(e).To(Equal("10.1.0.0/16"))
						p, e, err = ipam.GetSubnetsPerCluster("10.0.0.0/16", "10.1.0.0/16", "clustere")
						Expect(err).To(BeNil())
						Expect(p).ToNot(Equal("10.0.0.0/16"))
						Expect(e).ToNot(Equal("10.1.0.0/16"))
					})
				})
				Context("and there is not an available network with the same mask length in any pool", func() {
					It("should fail to allocate the network", func() {

						p, _, err := ipam.GetSubnetsPerCluster("10.0.0.0/9", "10.1.0.0/16", "cluster1")
						Expect(err).To(BeNil())
						Expect(p).To(Equal("10.0.0.0/9"))

						_, _, err = ipam.GetSubnetsPerCluster("10.0.0.0/9", "10.3.0.0/16", "cluster2")
						Expect(err).ToNot(BeNil())
					})
				})
			})
		})
	})
	Describe("FreeSubnetPerCluster", func() {
		Context("Freeing cluster networks that exist", func() {
			It("Should successfully free the subnets", func() {
				p, e, err := ipam.GetSubnetsPerCluster("10.0.1.0/24", "10.0.2.0/24", "cluster1")
				Expect(err).To(BeNil())
				Expect(p).To(Equal("10.0.1.0/24"))
				Expect(e).To(Equal("10.0.2.0/24"))
				err = ipam.FreeSubnetsPerCluster("cluster1")
				Expect(err).To(BeNil())
				p, e, err = ipam.GetSubnetsPerCluster("10.0.1.0/24", "10.0.2.0/24", "cluster1")
				Expect(err).To(BeNil())
				Expect(p).To(Equal("10.0.1.0/24"))
				Expect(e).To(Equal("10.0.2.0/24"))
			})
		})
		Context("Freeing a cluster network that does not exists", func() {
			It("Should return no errors", func() {
				err := ipam.FreeSubnetsPerCluster("cluster1")
				Expect(err).To(BeNil())
			})
		})
	})
	Describe("FreeReservedSubnet", func() {
		Context("Freeing a network that has been reserved previously", func() {
			It("Should successfully free the subnet", func() {
				err := ipam.AcquireReservedSubnet("10.0.1.0/24")
				Expect(err).To(BeNil())
				err = ipam.FreeReservedSubnet("10.0.1.0/24")
				Expect(err).To(BeNil())
				err = ipam.AcquireReservedSubnet("10.0.1.0/24")
				Expect(err).To(BeNil())
			})
		})
		Context("Freeing a cluster network that does not exists", func() {
			It("Should return no errors", func() {
				err := ipam.FreeReservedSubnet("10.0.1.0/24")
				Expect(err).To(BeNil())
			})
		})
		Context("Freeing a reserved subnet equal to a network pool", func() {
			It("Should make available the network pool", func() {
				err := ipam.AcquireReservedSubnet("10.0.0.0/8")
				Expect(err).To(BeNil())
				err = ipam.FreeReservedSubnet("10.0.0.0/8")
				Expect(err).To(BeNil())
				p, e, err := ipam.GetSubnetsPerCluster("10.0.0.0/16", "10.2.0.0/16", "cluster1")
				Expect(err).To(BeNil())
				Expect(p).To(Equal("10.0.0.0/16"))
				Expect(e).To(Equal("10.2.0.0/16"))
			})
		})
	})
	Describe("Re-scheduling of network manager", func() {
		It("ipam should retrieve configuration by resource", func() {
			// Assign networks to cluster
			p, e, err := ipam.GetSubnetsPerCluster("10.0.1.0/24", "10.0.2.0/24", "cluster1")
			Expect(err).To(BeNil())
			Expect(p).To(Equal("10.0.1.0/24"))
			Expect(e).To(Equal("10.0.2.0/24"))

			// Simulate re-scheduling
			ipam.StopGRPCServer()
			ipam = liqonet.NewIPAM()
			err = ipam.Init(liqonet.Pools, dynClient, 2000+rand.Intn(2000))
			Expect(err).To(BeNil())

			// Another cluster asks for the same networks
			p, e, err = ipam.GetSubnetsPerCluster("10.0.1.0/24", "10.0.2.0/24", "cluster2")
			Expect(err).To(BeNil())
			Expect(p).ToNot(Equal("10.0.1.0/24"))
			Expect(e).ToNot(Equal("10.0.2.0/24"))
		})
	})
	Describe("AddNetworkPool", func() {
		Context("Trying to add a default network pool", func() {
			It("Should generate an error", func() {
				err := ipam.AddNetworkPool("10.0.0.0/8")
				Expect(err).ToNot(BeNil())
			})
		})
		Context("Trying to add twice the same network pool", func() {
			It("Should generate an error", func() {
				err := ipam.AddNetworkPool("11.0.0.0/8")
				Expect(err).To(BeNil())
				err = ipam.AddNetworkPool("11.0.0.0/8")
				Expect(err).ToNot(BeNil())
			})
		})
		Context("After adding a new network pool", func() {
			It("Should be possible to use that pool for cluster networks", func() {
				// Reserve default network pools
				for _, network := range liqonet.Pools {
					err := fillNetworkPool(network, ipam)
					Expect(err).To(BeNil())
				}

				// Add new network pool
				err := ipam.AddNetworkPool("11.0.0.0/8")
				Expect(err).To(BeNil())

				// Reserve a given network
				err = ipam.AcquireReservedSubnet("12.0.0.0/24")
				Expect(err).To(BeNil())

				// Reserve a given network
				err = ipam.AcquireReservedSubnet("12.0.1.0/24")
				Expect(err).To(BeNil())

				// IPAM should use 11.0.0.0/8 to map the cluster network
				p, e, err := ipam.GetSubnetsPerCluster("12.0.0.0/24", "12.0.1.0/24", "cluster1")
				Expect(err).To(BeNil())
				Expect(p).To(HavePrefix("11"))
				Expect(p).To(HaveSuffix("/24"))
				Expect(e).To(HavePrefix("11"))
				Expect(e).To(HaveSuffix("/24"))
			})
		})
		Context("Trying to add a network pool that overlaps with a reserved network", func() {
			It("Should generate an error", func() {
				err := ipam.AcquireReservedSubnet("11.0.0.0/8")
				Expect(err).To(BeNil())
				err = ipam.AddNetworkPool("11.0.0.0/16")
				Expect(err).ToNot(BeNil())
			})
		})
	})
	Describe("RemoveNetworkPool", func() {
		Context("Remove a network pool that does not exist", func() {
			It("Should return an error", func() {
				err := ipam.RemoveNetworkPool("11.0.0.0/8")
				Expect(err).ToNot(BeNil())
			})
		})
		Context("Remove a network pool that exists", func() {
			It("Should successfully remove the network pool", func() {
				// Reserve default network pools
				for _, network := range liqonet.Pools {
					err := ipam.AcquireReservedSubnet(network)
					Expect(err).To(BeNil())
				}

				// Add new network pool
				err := ipam.AddNetworkPool("11.0.0.0/8")
				Expect(err).To(BeNil())

				// Remove network pool
				err = ipam.RemoveNetworkPool("11.0.0.0/8")
				Expect(err).To(BeNil())

				// Reserve a given network
				err = ipam.AcquireReservedSubnet("12.0.0.0/24")
				Expect(err).To(BeNil())

				// Reserve a given network
				err = ipam.AcquireReservedSubnet("12.0.1.0/24")
				Expect(err).To(BeNil())

				// Should fail to assign a network to cluster
				_, _, err = ipam.GetSubnetsPerCluster("12.0.0.0/24", "12.0.1.0/24", "cluster1")
				Expect(err).ToNot(BeNil())
			})
		})
		Context("Remove a network pool that is a default one", func() {
			It("Should generate an error", func() {
				err := ipam.RemoveNetworkPool("10.0.0.0/8")
				Expect(err).ToNot(BeNil())
			})
		})
		Context("Remove a network pool that is used for a cluster", func() {
			It("Should generate an error", func() {
				// Reserve default network pools
				for _, network := range liqonet.Pools {
					err := ipam.AcquireReservedSubnet(network)
					Expect(err).To(BeNil())
				}

				// Add new network pool
				err := ipam.AddNetworkPool("11.0.0.0/8")
				Expect(err).To(BeNil())

				// Reserve a network
				err = ipam.AcquireReservedSubnet("12.0.0.0/24")
				Expect(err).To(BeNil())

				// Reserve a network
				err = ipam.AcquireReservedSubnet("12.0.1.0/24")
				Expect(err).To(BeNil())

				// IPAM should use 11.0.0.0/8 to map the cluster network
				p, e, err := ipam.GetSubnetsPerCluster("12.0.0.0/24", "12.0.1.0/24", "cluster1")
				Expect(err).To(BeNil())
				Expect(p).To(HavePrefix("11"))
				Expect(p).To(HaveSuffix("/24"))
				Expect(e).To(HavePrefix("11"))
				Expect(e).To(HaveSuffix("/24"))

				err = ipam.RemoveNetworkPool("11.0.0.0/8")
				Expect(err).ToNot(BeNil())
			})
		})
	})

	Describe("AddLocalSubnetsPerCluster", func() {
		Context("If the networks do not exist yet", func() {
			It("should return no errors", func() {
				err := ipam.AddLocalSubnetsPerCluster("10.0.0.0/24", "192.168.0.0/24", "cluster1")
				Expect(err).To(BeNil())
			})
		})
		Context("If the networks already exist", func() {
			It("should return no errors", func() {
				err := ipam.AddLocalSubnetsPerCluster("10.0.0.0/24", "192.168.0.0/24", "cluster1")
				Expect(err).To(BeNil())
				err = ipam.AddLocalSubnetsPerCluster("10.0.0.0/24", "192.168.0.0/24", "cluster1")
				Expect(err).To(BeNil())
			})
		})
	})

	Describe("RemoveLocalSubnetsPerCluster", func() {
		Context("If the networks do not exist", func() {
			It("should return no errors", func() {
				err := ipam.RemoveLocalSubnetsPerCluster("cluster1")
				Expect(err).To(BeNil())
			})
		})
		Context("If the networks exist", func() {
			It("should return no errors", func() {
				err := ipam.AddLocalSubnetsPerCluster("10.0.0.0/24", "192.168.0.0/24", "cluster1")
				Expect(err).To(BeNil())
				err = ipam.RemoveLocalSubnetsPerCluster("cluster1")
				Expect(err).To(BeNil())
			})
		})
	})

	Describe("GetExternalCIDR", func() {
		Context("Invoking it twice", func() {
			It("should return no errors", func() {
				e, err := ipam.GetExternalCIDR(24)
				Expect(err).To(BeNil())
				Expect(e).To(HaveSuffix("/24"))
				_, err = ipam.GetExternalCIDR(24)
				Expect(err).To(BeNil())
			})
		})
		Context("Using a valid mask length", func() {
			It("should return no errors", func() {
				e, err := ipam.GetExternalCIDR(24)
				Expect(err).To(BeNil())
				Expect(e).To(HaveSuffix("/24"))
			})
		})
		Context("Using an invalid mask length", func() {
			It("should return an error", func() {
				_, err := ipam.GetExternalCIDR(33)
				Expect(err).ToNot(BeNil())
			})
		})
	})

	Describe("SetPodCIDR", func() {
		Context("Invoking func for the first time", func() {
			It("should return no errors", func() {
				err := ipam.SetPodCIDR("10.0.0.0/24")
				Expect(err).To(BeNil())
			})
		})
		Context("Later invocation with the same PodCIDR", func() {
			It("should return no errors", func() {
				err := ipam.SetPodCIDR("10.0.0.0/24")
				Expect(err).To(BeNil())
				err = ipam.SetPodCIDR("10.0.0.0/24")
				Expect(err).To(BeNil())
			})
		})
		Context("Later invocation with a different PodCIDR", func() {
			It("should return no errors", func() {
				err := ipam.SetPodCIDR("10.0.0.0/24")
				Expect(err).To(BeNil())
				err = ipam.SetPodCIDR("10.0.1.0/24")
				Expect(err).ToNot(BeNil())
			})
		})
		Context("Using a reserved network", func() {
			It("should return an error", func() {
				err := ipam.AcquireReservedSubnet("10.0.1.0/24")
				Expect(err).To(BeNil())
				err = ipam.SetPodCIDR("10.0.1.0/24")
				Expect(err).ToNot(BeNil())
			})
		})
	})
	Describe("SetServiceCIDR", func() {
		Context("Invoking func for the first time", func() {
			It("should return no errors", func() {
				err := ipam.SetServiceCIDR("10.0.0.0/24")
				Expect(err).To(BeNil())
			})
		})
		Context("Later invocation with the same ServiceCIDR", func() {
			It("should return no errors", func() {
				err := ipam.SetServiceCIDR("10.0.0.0/24")
				Expect(err).To(BeNil())
				err = ipam.SetServiceCIDR("10.0.0.0/24")
				Expect(err).To(BeNil())
			})
		})
		Context("Later invocation with a different ServiceCIDR", func() {
			It("should return no errors", func() {
				err := ipam.SetServiceCIDR("10.0.0.0/24")
				Expect(err).To(BeNil())
				err = ipam.SetServiceCIDR("10.0.1.0/24")
				Expect(err).ToNot(BeNil())
			})
		})
		Context("Using a reserved network", func() {
			It("should return an error", func() {
				err := ipam.AcquireReservedSubnet("10.0.1.0/24")
				Expect(err).To(BeNil())
				err = ipam.SetServiceCIDR("10.0.1.0/24")
				Expect(err).ToNot(BeNil())
			})
		})
	})
	Describe("MapEndpointIP", func() {
		Context("If the endpoint IP belongs to local PodCIDR", func() {
			Context("and the remote cluster has not remapped the local PodCIDR", func() {
				It("should return the same IP address", func() {
					// Set PodCIDR
					err := ipam.SetPodCIDR("10.0.0.0/24")
					Expect(err).To(BeNil())

					// Remote cluster has not remapped local PodCIDR
					err = ipam.AddLocalSubnetsPerCluster("None", "None", "cluster1")
					Expect(err).To(BeNil())

					response, err := ipam.MapEndpointIP(context.Background(), &liqonet.MapRequest{
						ClusterID: "cluster1",
						Ip:        "10.0.0.1",
					})
					Expect(err).To(BeNil())
					Expect(response.GetIp()).To(Equal("10.0.0.1"))
				})
			})
			Context("and the remote cluster has remapped the local PodCIDR", func() {
				It("should map the endpoint IP using the remapped PodCIDR", func() {
					// Set PodCIDR
					err := ipam.SetPodCIDR("10.0.0.0/24")
					Expect(err).To(BeNil())

					// Remote cluster has remapped local PodCIDR
					err = ipam.AddLocalSubnetsPerCluster("192.168.0.0/24", "None", "cluster1")
					Expect(err).To(BeNil())

					response, err := ipam.MapEndpointIP(context.Background(), &liqonet.MapRequest{
						ClusterID: "cluster1",
						Ip:        "10.0.0.1",
					})
					Expect(err).To(BeNil())
					Expect(response.GetIp()).To(Equal("192.168.0.1"))
				})
			})
		})
		Context("If the endpoint IP does not belong to local PodCIDR", func() {
			Context("and the remote cluster has not remapped the local ExternalCIDR", func() {
				It("should map the endpoint IP to a new IP belonging to local ExternalCIDR", func() {
					// Set PodCIDR
					err := ipam.SetPodCIDR("10.0.0.0/24")
					Expect(err).To(BeNil())

					// Set ExternalCIDR
					externalCIDR, err := ipam.GetExternalCIDR(24)
					Expect(err).To(BeNil())
					Expect(externalCIDR).To(HaveSuffix("/24"))

					// Remote cluster has not remapped local ExternalCIDR
					err = ipam.AddLocalSubnetsPerCluster("None", "None", "cluster1")
					Expect(err).To(BeNil())

					response, err := ipam.MapEndpointIP(context.Background(), &liqonet.MapRequest{
						ClusterID: "cluster1",
						Ip:        "20.0.0.1",
					})
					Expect(err).To(BeNil())
					slicedPrefix := strings.SplitN(externalCIDR, ".", 4)
					slicedPrefix = slicedPrefix[:len(slicedPrefix)-1]
					Expect(response.GetIp()).To(HavePrefix(strings.Join(slicedPrefix, ".")))
				})
				It("should return the same IP if more remote clusters ask for the same endpoint", func() {
					// Set PodCIDR
					err := ipam.SetPodCIDR("10.0.0.0/24")
					Expect(err).To(BeNil())

					// Set ExternalCIDR
					externalCIDR, err := ipam.GetExternalCIDR(24)
					Expect(err).To(BeNil())
					Expect(externalCIDR).To(HaveSuffix("/24"))

					err = ipam.AddLocalSubnetsPerCluster("None", "None", "cluster1")
					Expect(err).To(BeNil())
					err = ipam.AddLocalSubnetsPerCluster("None", "None", "cluster2")
					Expect(err).To(BeNil())

					// Reflection cluster1
					response, err := ipam.MapEndpointIP(context.Background(), &liqonet.MapRequest{
						ClusterID: "cluster1",
						Ip:        "20.0.0.1",
					})
					Expect(err).To(BeNil())
					slicedPrefix := strings.SplitN(externalCIDR, ".", 4)
					slicedPrefix = slicedPrefix[:len(slicedPrefix)-1]
					Expect(response.GetIp()).To(HavePrefix(strings.Join(slicedPrefix, ".")))
					expectedIp := response.GetIp()

					// Reflection cluster2
					response, err = ipam.MapEndpointIP(context.Background(), &liqonet.MapRequest{
						ClusterID: "cluster1",
						Ip:        "20.0.0.1",
					})
					Expect(err).To(BeNil())
					Expect(response.GetIp()).To(Equal(expectedIp))
				})
			})
			Context("and the remote cluster has remapped the local ExternalCIDR", func() {
				It("should map the endpoint IP to a new IP belonging to the remapped ExternalCIDR", func() {
					// Set PodCIDR
					err := ipam.SetPodCIDR("10.0.0.0/24")
					Expect(err).To(BeNil())

					// Set ExternalCIDR
					e, err := ipam.GetExternalCIDR(24)
					Expect(err).To(BeNil())
					Expect(e).To(HaveSuffix("/24"))

					// Remote cluster has remapped local ExternalCIDR
					err = ipam.AddLocalSubnetsPerCluster("None", "192.168.0.0/24", "cluster1")
					Expect(err).To(BeNil())

					response, err := ipam.MapEndpointIP(context.Background(), &liqonet.MapRequest{
						ClusterID: "cluster1",
						Ip:        "20.0.0.1",
					})
					Expect(err).To(BeNil())
					Expect(response.GetIp()).To(HavePrefix("192.168.0."))
				})
			})
			Context("and the ExternalCIDR has not any more available IPs", func() {
				It("should return an error", func() {
					var response *liqonet.MapResponse
					var err error
					// Set PodCIDR
					err = ipam.SetPodCIDR("10.0.0.0/24")
					Expect(err).To(BeNil())

					// Set ExternalCIDR
					externalCIDR, err := ipam.GetExternalCIDR(24)
					Expect(err).To(BeNil())
					Expect(externalCIDR).To(HaveSuffix("/24"))
					slicedPrefix := strings.SplitN(externalCIDR, ".", 4)
					slicedPrefix = slicedPrefix[:len(slicedPrefix)-1]

					// Remote cluster has not remapped local ExternalCIDR
					err = ipam.AddLocalSubnetsPerCluster("None", "None", "cluster1")
					Expect(err).To(BeNil())

					// Fill up ExternalCIDR
					for i := 0; i < 254; i++ {
						response, err = ipam.MapEndpointIP(context.Background(), &liqonet.MapRequest{
							ClusterID: "cluster1",
							Ip:        fmt.Sprintf("20.0.0.%d", i),
						})
						Expect(err).To(BeNil())
						Expect(response.GetIp()).To(HavePrefix(strings.Join(slicedPrefix, ".")))
					}

					_, err = ipam.MapEndpointIP(context.Background(), &liqonet.MapRequest{
						ClusterID: "cluster1",
						Ip:        "3.100.0.9",
					})
					Expect(err).ToNot(BeNil())
				})
			})
			Context("Using an invalid endpointIP", func() {
				It("should return an error", func() {
					_, err := ipam.MapEndpointIP(context.Background(), &liqonet.MapRequest{
						ClusterID: "cluster1",
						Ip:        "30.0.9",
					})
					Expect(err).ToNot(BeNil())
				})
			})
			Context("If the local PodCIDR is not set", func() {
				It("should return an error", func() {
					_, err := ipam.MapEndpointIP(context.Background(), &liqonet.MapRequest{
						ClusterID: "cluster1",
						Ip:        "30.0.4.9",
					})
					Expect(err.Error()).To(ContainSubstring("cluster PodCIDR is not set"))
				})
			})
			Context("If the remote cluster has not a PodCIDR set", func() {
				It("should return an error", func() {
					// Set PodCIDR
					err := ipam.SetPodCIDR("10.0.0.0/24")
					Expect(err).To(BeNil())

					_, err = ipam.MapEndpointIP(context.Background(), &liqonet.MapRequest{
						ClusterID: "cluster1",
						Ip:        "10.0.0.9",
					})
					Expect(err.Error()).To(ContainSubstring("remote cluster cluster1 has not a local NAT PodCIDR"))
				})
			})
			Context("If the remote cluster has not an ExternalCIDR set", func() {
				It("should return an error", func() {
					// Set PodCIDR
					err := ipam.SetPodCIDR("10.0.0.0/24")
					Expect(err).To(BeNil())

					_, err = ipam.MapEndpointIP(context.Background(), &liqonet.MapRequest{
						ClusterID: "cluster1",
						Ip:        "30.0.4.9",
					})
					Expect(err.Error()).To(ContainSubstring("remote cluster cluster1 has not a Local NAT ExternalCIDR"))
				})
			})
		})
	})
	Describe("UnmapEndpointIP", func() {
		Context("If there are no more clusters using an endpointIP", func() {
			It("should free the relative IP", func() {
				// Set PodCIDR
				err := ipam.SetPodCIDR("10.0.0.0/24")
				Expect(err).To(BeNil())

				// Set ExternalCIDR
				externalCIDR, err := ipam.GetExternalCIDR(24)
				Expect(err).To(BeNil())
				Expect(externalCIDR).To(HaveSuffix("/24"))
				slicedPrefix := strings.SplitN(externalCIDR, ".", 4)
				slicedPrefix = slicedPrefix[:len(slicedPrefix)-1]

				err = ipam.AddLocalSubnetsPerCluster("None", "None", "cluster1")
				Expect(err).To(BeNil())
				err = ipam.AddLocalSubnetsPerCluster("None", "None", "cluster2")
				Expect(err).To(BeNil())

				// Reflection in cluster1
				response, err := ipam.MapEndpointIP(context.Background(), &liqonet.MapRequest{
					ClusterID: "cluster1",
					Ip:        "20.0.0.1",
				})
				Expect(err).To(BeNil())
				Expect(response.GetIp()).To(HavePrefix(strings.Join(slicedPrefix, ".")))
				ip := response.GetIp()

				// Reflection in cluster2
				_, err = ipam.MapEndpointIP(context.Background(), &liqonet.MapRequest{
					ClusterID: "cluster2",
					Ip:        "20.0.0.1",
				})
				Expect(err).To(BeNil())

				// Terminate reflection in cluster1
				_, err = ipam.UnmapEndpointIP(context.Background(), &liqonet.UnmapRequest{
					ClusterID: "cluster1",
					Ip:        "20.0.0.1",
				})
				Expect(err).To(BeNil())

				// Terminate reflection in cluster2
				_, err = ipam.UnmapEndpointIP(context.Background(), &liqonet.UnmapRequest{
					ClusterID: "cluster2",
					Ip:        "20.0.0.1",
				})
				Expect(err).To(BeNil())

				/* In order to check if the IP has been freed, simulate further reflections
				till the ExternalCIDR has no more IPs and check if the returned IP is equal to
				the freed IP.
				An alternative could be to overwrite the stdout and check
				existence of the log "IP has been freed".*/
				var found bool
				for i := 0; i < 254; i++ {
					err = ipam.AddLocalSubnetsPerCluster("None", "None", fmt.Sprintf("c%d", i))
					Expect(err).To(BeNil())
					r, err := ipam.MapEndpointIP(context.Background(), &liqonet.MapRequest{
						ClusterID: fmt.Sprintf("c%d", i),
						Ip:        fmt.Sprintf("30.0.0.%d", i),
					})
					Expect(err).To(BeNil())
					if r.GetIp() == ip {
						found = true
						break
					}
				}
				if !found {
					Fail("ip has not been freed")
				}
			})
		})
		Context("If there are other clusters using an endpointIP", func() {
			It("should not free the relative IP", func() {
				// Set PodCIDR
				err := ipam.SetPodCIDR("10.0.0.0/24")
				Expect(err).To(BeNil())

				// Set ExternalCIDR
				externalCIDR, err := ipam.GetExternalCIDR(24)
				Expect(err).To(BeNil())
				Expect(externalCIDR).To(HaveSuffix("/24"))
				slicedPrefix := strings.SplitN(externalCIDR, ".", 4)
				slicedPrefix = slicedPrefix[:len(slicedPrefix)-1]

				err = ipam.AddLocalSubnetsPerCluster("None", "None", "cluster1")
				Expect(err).To(BeNil())
				err = ipam.AddLocalSubnetsPerCluster("None", "None", "cluster2")
				Expect(err).To(BeNil())

				// Reflection in cluster1
				response, err := ipam.MapEndpointIP(context.Background(), &liqonet.MapRequest{
					ClusterID: "cluster1",
					Ip:        "20.0.0.1",
				})
				Expect(err).To(BeNil())
				Expect(response.GetIp()).To(HavePrefix(strings.Join(slicedPrefix, ".")))
				ip := response.GetIp()

				// Reflection in cluster2
				_, err = ipam.MapEndpointIP(context.Background(), &liqonet.MapRequest{
					ClusterID: "cluster2",
					Ip:        "20.0.0.1",
				})
				Expect(err).To(BeNil())

				// Terminate reflection in cluster2
				_, err = ipam.UnmapEndpointIP(context.Background(), &liqonet.UnmapRequest{
					ClusterID: "cluster2",
					Ip:        "20.0.0.1",
				})
				Expect(err).To(BeNil())

				/* In order to check if the IP has been freed, simulate further reflections
				till the ExternalCIDR has no more IPs and check if the returned IP is equal to
				the freed IP.
				An alternative could be to overwrite the stdout and check
				existence of the log "IP has been freed".*/
				var found bool
				for i := 0; i < 254; i++ {
					err = ipam.AddLocalSubnetsPerCluster("None", "None", fmt.Sprintf("c%d", i))
					Expect(err).To(BeNil())
					r, _ := ipam.MapEndpointIP(context.Background(), &liqonet.MapRequest{
						ClusterID: fmt.Sprintf("c%d", i),
						Ip:        fmt.Sprintf("30.0.0.%d", i),
					})
					if r != nil && r.GetIp() == ip {
						found = true
						break
					}
				}
				if found {
					Fail("ip has been freed")
				}
			})
		})
	})
})
