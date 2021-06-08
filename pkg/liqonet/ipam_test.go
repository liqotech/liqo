package liqonet_test

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"math/rand"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqonet"
	liqoneterrors "github.com/liqotech/liqo/pkg/liqonet/errors"
	"github.com/liqotech/liqo/pkg/liqonet/natmappinginflater"
	"github.com/liqotech/liqo/pkg/liqonet/utils"

	liqonetapi "github.com/liqotech/liqo/apis/net/v1alpha1"
)

var ipam *liqonet.IPAM
var dynClient *fake.FakeDynamicClient

const (
	clusterID1         = "cluster1"
	clusterID2         = "cluster2"
	podCIDR            = "10.0.0.0/24"
	externalCIDR       = "10.0.1.0/24"
	externalEndpointIP = "10.0.50.6"
	internalEndpointIP = "10.0.0.6"
)

const invalidValue = "invalid value"
const clusterName = "cluster1"

func fillNetworkPool(pool string, ipam *liqonet.IPAM) error {

	// Get halves mask length
	mask := utils.GetMask(pool)
	mask += 1

	// Get first half CIDR
	halfCidr, err := utils.SetMask(pool, mask)
	if err != nil {
		return err
	}

	err = ipam.AcquireReservedSubnet(halfCidr)
	if err != nil {
		return err
	}

	// Get second half CIDR
	halfCidr, err = utils.Next(halfCidr)
	if err != nil {
		return err
	}
	err = ipam.AcquireReservedSubnet(halfCidr)

	return err
}

func setDynClient() error {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{
		Group:   "net.liqo.io",
		Version: "v1alpha1",
		Kind:    "ipamstorages",
	}, &liqonetapi.IpamStorage{})
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{
		Group:   "net.liqo.io",
		Version: "v1alpha1",
		Kind:    "natmappings",
	}, &liqonetapi.NatMapping{})

	var m = make(map[schema.GroupVersionResource]string)

	m[schema.GroupVersionResource{
		Group:    "net.liqo.io",
		Version:  "v1alpha1",
		Resource: "ipamstorages",
	}] = "ipamstoragesList"

	m[schema.GroupVersionResource{
		Group:    "net.liqo.io",
		Version:  "v1alpha1",
		Resource: "natmappings",
	}] = "natmappingsList"

	// Init fake dynamic client with objects in order to avoid errors in InitNatMappings func
	// due to the lack of support of fake.dynamicClient for creation of more than 2 resources of the same Kind.
	nm1, err := natmappinginflater.ForgeNatMapping("cluster1", "10.0.0.0/24", "10.0.1.0/24", make(map[string]string))
	if err != nil {
		return err
	}
	nm2, err := natmappinginflater.ForgeNatMapping("cluster2", "10.0.0.0/24", "10.0.1.0/24", make(map[string]string))
	if err != nil {
		return err
	}

	dynClient = fake.NewSimpleDynamicClientWithCustomListKinds(scheme, m, nm1, nm2)
	return nil
}

var _ = Describe("Ipam", func() {
	rand.Seed(1)

	BeforeEach(func() {
		ipam = liqonet.NewIPAM()
		err := setDynClient()
		Expect(err).To(BeNil())
		err = ipam.Init(liqonet.Pools, dynClient, 2000+rand.Intn(2000))
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
		Context("Call after SetPodCIDR", func() {
			It("should return no errors", func() {
				err := ipam.SetPodCIDR("10.0.0.0/24")
				Expect(err).To(BeNil())
				externalCIDR, err := ipam.GetExternalCIDR(24)
				Expect(err).To(BeNil())
				Expect(externalCIDR).To(Equal("10.0.1.0/24"))
			})
		})
		Context("Call before SetPodCIDR", func() {
			It("should produce an error in SetPodCIDR", func() {
				externalCIDR, err := ipam.GetExternalCIDR(24)
				Expect(err).To(BeNil())
				Expect(externalCIDR).To(Equal("10.0.0.0/24"))
				// ExternalCIDR has been assigned "10.0.0.0/24", so the network
				// is not available anymore.
				err = ipam.SetPodCIDR("10.0.0.0/24")
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

					// Set ExternalCIDR
					_, err = ipam.GetExternalCIDR(24)
					Expect(err).To(BeNil())

					_, _, err = ipam.GetSubnetsPerCluster("10.0.1.0/24", "10.0.2.0/24", "cluster1")
					Expect(err).To(BeNil())

					// Remote cluster has not remapped local PodCIDR
					err = ipam.AddLocalSubnetsPerCluster("None", "None", "cluster1")
					Expect(err).To(BeNil())

					err = ipam.InitNatMappingsPerCluster("cluster1")
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

					// Set ExternalCIDR
					_, err = ipam.GetExternalCIDR(24)
					Expect(err).To(BeNil())

					_, _, err = ipam.GetSubnetsPerCluster("10.0.1.0/24", "10.0.2.0/24", "cluster1")
					Expect(err).To(BeNil())

					// Remote cluster has remapped local PodCIDR
					err = ipam.AddLocalSubnetsPerCluster("192.168.0.0/24", "None", "cluster1")
					Expect(err).To(BeNil())

					err = ipam.InitNatMappingsPerCluster("cluster1")
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

					_, _, err = ipam.GetSubnetsPerCluster("10.0.1.0/24", "10.0.2.0/24", "cluster1")
					Expect(err).To(BeNil())

					// Remote cluster has not remapped local ExternalCIDR
					err = ipam.AddLocalSubnetsPerCluster("None", "None", "cluster1")
					Expect(err).To(BeNil())

					err = ipam.InitNatMappingsPerCluster("cluster1")
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

					_, _, err = ipam.GetSubnetsPerCluster("10.0.1.0/24", "10.0.2.0/24", "cluster1")
					Expect(err).To(BeNil())
					_, _, err = ipam.GetSubnetsPerCluster("10.0.1.0/24", "10.0.2.0/24", "cluster2")
					Expect(err).To(BeNil())

					err = ipam.AddLocalSubnetsPerCluster("None", "None", "cluster1")
					Expect(err).To(BeNil())
					err = ipam.AddLocalSubnetsPerCluster("None", "None", "cluster2")
					Expect(err).To(BeNil())

					err = ipam.InitNatMappingsPerCluster("cluster1")
					Expect(err).To(BeNil())
					err = ipam.InitNatMappingsPerCluster("cluster2")
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

					_, _, err = ipam.GetSubnetsPerCluster("10.0.1.0/24", "10.0.2.0/24", "cluster1")
					Expect(err).To(BeNil())

					// Remote cluster has remapped local ExternalCIDR
					err = ipam.AddLocalSubnetsPerCluster("None", "192.168.0.0/24", "cluster1")
					Expect(err).To(BeNil())

					err = ipam.InitNatMappingsPerCluster("cluster1")
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

					_, _, err = ipam.GetSubnetsPerCluster("10.0.1.0/24", "10.0.2.0/24", "cluster1")
					Expect(err).To(BeNil())

					// Remote cluster has not remapped local ExternalCIDR
					err = ipam.AddLocalSubnetsPerCluster("None", "None", "cluster1")
					Expect(err).To(BeNil())

					err = ipam.InitNatMappingsPerCluster("cluster1")
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
					Expect(err.Error()).To(ContainSubstring("cannot parse network"))
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

	Describe("GetHomePodIP", func() {
		Context("Pass function an invalid IP address", func() {
			It("should return WrongParameter error", func() {
				_, err := ipam.GetHomePodIP(context.Background(),
					&liqonet.GetHomePodIPRequest{
						Ip:        invalidValue,
						ClusterID: clusterName,
					})
				err = errors.Unwrap(err)
				Expect(err).To(MatchError(fmt.Sprintf("%s must be %s", invalidValue, liqoneterrors.ValidIP)))
			})
		})
		Context("Pass function an empty cluster ID", func() {
			It("should return WrongParameter error", func() {
				_, err := ipam.GetHomePodIP(context.Background(),
					&liqonet.GetHomePodIPRequest{
						Ip:        invalidValue,
						ClusterID: "",
					})
				err = errors.Unwrap(err)
				Expect(err).To(MatchError(fmt.Sprintf("%s must be %s", consts.ClusterIDLabelName, liqoneterrors.StringNotEmpty)))
			})
		})
		Context("Invoking func without subnets init", func() {
			It("should return WrongParameter error", func() {
				_, err := ipam.GetHomePodIP(context.Background(),
					&liqonet.GetHomePodIPRequest{
						Ip:        "10.0.0.1",
						ClusterID: clusterName,
					})
				err = errors.Unwrap(err)
				Expect(err).To(MatchError(fmt.Sprintf("cluster %s subnets are not set", clusterName)))
			})
		})
		Context(`When the remote Pod CIDR has not been remapped by home cluster
			and the call refers to a remote Pod`, func() {
			It("should return the same IP", func() {
				ip := "10.0.10.1"
				podCIDR := "10.0.10.0/24"
				externalCIDR := "10.0.50.0/24"

				// Home cluster has not remapped remote PodCIDR
				mappedPodCIDR, _, err := ipam.GetSubnetsPerCluster(podCIDR, externalCIDR, clusterName)
				Expect(err).To(BeNil())
				Expect(mappedPodCIDR).To(Equal(podCIDR))

				response, err := ipam.GetHomePodIP(context.Background(),
					&liqonet.GetHomePodIPRequest{
						Ip:        ip,
						ClusterID: clusterName,
					})
				Expect(err).To(BeNil())
				Expect(response.GetHomeIP()).To(Equal(ip))
			})
		})
		Context(`When the remote Pod CIDR has been remapped by home cluster
			and the call refers to a remote Pod`, func() {
			It("should return the remapped IP", func() {
				ip := "10.0.10.1" // Original Pod IP
				podCIDR := "10.0.10.0/24"
				externalCIDR := "10.0.50.0/24"

				// Reserve original PodCIDR so that home cluster will remap it
				err := ipam.AcquireReservedSubnet(podCIDR)
				Expect(err).To(BeNil())

				// Home cluster has remapped remote PodCIDR
				mappedPodCIDR, _, err := ipam.GetSubnetsPerCluster(podCIDR, externalCIDR, clusterName)
				Expect(err).To(BeNil())
				Expect(mappedPodCIDR).ToNot(Equal(podCIDR))

				response, err := ipam.GetHomePodIP(context.Background(),
					&liqonet.GetHomePodIPRequest{
						Ip:        ip,
						ClusterID: clusterName,
					})
				Expect(err).To(BeNil())

				// IP should be mapped to remoteNATPodCIDR
				remappedIP, err := utils.MapIPToNetwork(mappedPodCIDR, ip)
				Expect(response.GetHomeIP()).To(Equal(remappedIP))
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

				_, _, err = ipam.GetSubnetsPerCluster("10.0.1.0/24", "10.0.2.0/24", "cluster1")
				Expect(err).To(BeNil())
				_, _, err = ipam.GetSubnetsPerCluster("10.0.1.0/24", "10.0.2.0/24", "cluster2")
				Expect(err).To(BeNil())

				err = ipam.AddLocalSubnetsPerCluster("None", "None", "cluster1")
				Expect(err).To(BeNil())
				err = ipam.AddLocalSubnetsPerCluster("None", "None", "cluster2")
				Expect(err).To(BeNil())

				err = ipam.InitNatMappingsPerCluster("cluster1")
				Expect(err).To(BeNil())
				err = ipam.InitNatMappingsPerCluster("cluster2")
				Expect(err).To(BeNil())

				// Reflection in cluster1
				response, err := ipam.MapEndpointIP(context.Background(), &liqonet.MapRequest{
					ClusterID: "cluster1",
					Ip:        "20.0.0.1",
				})
				Expect(err).To(BeNil())
				Expect(response.GetIp()).To(HavePrefix(strings.Join(slicedPrefix, ".")))

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

				// Get Ipam configuration
				unstructuredObj, err := dynClient.Resource(liqonetapi.IpamGroupResource).Get(context.Background(), "", v1.GetOptions{})
				Expect(err).To(BeNil())
				var ipamConfig liqonetapi.IpamStorage
				err = runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, &ipamConfig)
				Expect(err).To(BeNil())

				// Check if IP is freed
				Expect(ipamConfig.Spec.EndpointMappings).To(HaveLen(0))
			})
		})
		Context("If there are other clusters using an endpointIP", func() {
			It("should not free the relative IP", func() {
				endpointIP := "20.0.0.1"
				// Set PodCIDR
				err := ipam.SetPodCIDR("10.0.0.0/24")
				Expect(err).To(BeNil())

				// Set ExternalCIDR
				externalCIDR, err := ipam.GetExternalCIDR(24)
				Expect(err).To(BeNil())
				Expect(externalCIDR).To(HaveSuffix("/24"))
				slicedPrefix := strings.SplitN(externalCIDR, ".", 4)
				slicedPrefix = slicedPrefix[:len(slicedPrefix)-1]

				_, _, err = ipam.GetSubnetsPerCluster("10.0.1.0/24", "10.0.2.0/24", "cluster1")
				Expect(err).To(BeNil())
				_, _, err = ipam.GetSubnetsPerCluster("10.0.1.0/24", "10.0.2.0/24", "cluster2")
				Expect(err).To(BeNil())

				err = ipam.AddLocalSubnetsPerCluster("None", "None", "cluster1")
				Expect(err).To(BeNil())
				err = ipam.AddLocalSubnetsPerCluster("None", "None", "cluster2")
				Expect(err).To(BeNil())

				err = ipam.InitNatMappingsPerCluster("cluster1")
				Expect(err).To(BeNil())
				err = ipam.InitNatMappingsPerCluster("cluster2")
				Expect(err).To(BeNil())

				// Reflection in cluster1
				response, err := ipam.MapEndpointIP(context.Background(), &liqonet.MapRequest{
					ClusterID: "cluster1",
					Ip:        endpointIP,
				})
				Expect(err).To(BeNil())
				Expect(response.GetIp()).To(HavePrefix(strings.Join(slicedPrefix, ".")))
				ip := response.GetIp()

				// Reflection in cluster2
				_, err = ipam.MapEndpointIP(context.Background(), &liqonet.MapRequest{
					ClusterID: "cluster2",
					Ip:        endpointIP,
				})
				Expect(err).To(BeNil())

				// Terminate reflection in cluster2
				_, err = ipam.UnmapEndpointIP(context.Background(), &liqonet.UnmapRequest{
					ClusterID: "cluster2",
					Ip:        endpointIP,
				})
				Expect(err).To(BeNil())

				// Get Ipam configuration
				unstructuredObj, err := dynClient.Resource(liqonetapi.IpamGroupResource).Get(context.Background(), "", v1.GetOptions{})
				Expect(err).To(BeNil())
				var ipamConfig liqonetapi.IpamStorage
				err = runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, &ipamConfig)
				Expect(err).To(BeNil())

				// Check if IP is not freed
				Expect(ipamConfig.Spec.EndpointMappings).To(HaveLen(1))
				Expect(ipamConfig.Spec.EndpointMappings[endpointIP].IP).To(Equal(ip))
			})
		})
	})
	Describe("InitNatMappingsPerCluster", func() {
		// Function is a wrapper for the homonymous function in NatMappingInflater.
		// More tests do exist in natMappingInflater_test.go
		Context("Pass an empty cluster ID", func() {
			It("should return a WrongParameter error", func() {
				err := ipam.InitNatMappingsPerCluster("")
				Expect(err).To(MatchError(fmt.Sprintf("%s must be %s", consts.ClusterIDLabelName, liqoneterrors.StringNotEmpty)))
			})
		})
		Context("Call func without previous call to GetSubnetsPerCluster", func() {
			It("should return a WrongParameter error", func() {
				_, err := ipam.GetExternalCIDR(24)
				Expect(err).To(BeNil())
				err = ipam.AddLocalSubnetsPerCluster(consts.DefaultCIDRValue, "10.0.1.0/24", clusterID1)
				Expect(err).To(BeNil())
				err = ipam.InitNatMappingsPerCluster(clusterID1)
				Expect(err).To(MatchError(fmt.Sprintf("PodCIDR must be %s", liqoneterrors.StringNotEmpty)))
			})
		})
		Context("Call func without previous call to AddLocalSubnetsPerCluster", func() {
			It("should return a WrongParameter error", func() {
				_, err := ipam.GetExternalCIDR(24)
				Expect(err).To(BeNil())
				_, _, err = ipam.GetSubnetsPerCluster("10.0.1.0/24", "10.0.2.0/24", clusterID1)
				Expect(err).To(BeNil())
				err = ipam.InitNatMappingsPerCluster(clusterID1)
				Expect(err).To(MatchError(fmt.Sprintf("ExternalCIDR must be %s", liqoneterrors.StringNotEmpty)))
			})
		})
	})
	Describe("TerminateNatMappingsPerCluster", func() {
		Context("Pass an empty cluster ID", func() {
			It("should return a WrongParameter error", func() {
				err := ipam.TerminateNatMappingsPerCluster("")
				Expect(err).To(MatchError(fmt.Sprintf("%s must be %s", consts.ClusterIDLabelName, liqoneterrors.StringNotEmpty)))
			})
		})
		Context("Terminate mappings without init", func() {
			It("should return no errors", func() {
				err := ipam.TerminateNatMappingsPerCluster(clusterID1)
				Expect(err).To(BeNil())
			})
		})
		Context("Terminate mappings when the cluster has not any mapping active", func() {
			It("should return no errors", func() {
				// Following invocation are necessary to next call InitNatMappingsPerCluster
				_, err := ipam.GetExternalCIDR(24)
				Expect(err).To(BeNil())

				_, _, err = ipam.GetSubnetsPerCluster("10.0.0.0/24", "10.0.1.0/24", clusterID1)
				Expect(err).To(BeNil())

				err = ipam.AddLocalSubnetsPerCluster("None", "None", clusterID1)
				Expect(err).To(BeNil())

				// Init mappings
				err = ipam.InitNatMappingsPerCluster(clusterID1)
				Expect(err).To(BeNil())

				// Do not perform any mapping and terminate mappings
				err = ipam.TerminateNatMappingsPerCluster(clusterID1)
				Expect(err).To(BeNil())

				// Check if resource exists
				list, err := dynClient.Resource(liqonetapi.NatMappingGroupResource).List(
					context.Background(),
					v1.ListOptions{
						LabelSelector: fmt.Sprintf("%s=%s,%s=%s",
							consts.NatMappingResourceLabelKey,
							consts.NatMappingResourceLabelValue,
							consts.ClusterIDLabelName,
							clusterID1),
					},
				)
				Expect(err).To(BeNil())
				Expect(list.Items).To(BeEmpty())
			})
		})
		Context("Terminate mappings when the cluster has an active mapping and"+
			"the endpoint is not reflected in any other cluster", func() {
			It("should delete the endpoint mapping", func() {
				err := ipam.SetPodCIDR(podCIDR)
				Expect(err).To(BeNil())

				_, err = ipam.GetExternalCIDR(24)
				Expect(err).To(BeNil())

				_, _, err = ipam.GetSubnetsPerCluster("10.0.1.0/24", "10.0.2.0/24", clusterID1)
				Expect(err).To(BeNil())

				// Remote cluster has not remapped local ExternalCIDR
				err = ipam.AddLocalSubnetsPerCluster("None", "None", clusterID1)
				Expect(err).To(BeNil())

				response, err := ipam.MapEndpointIP(context.Background(),
					&liqonet.MapRequest{
						ClusterID: clusterID1,
						Ip:        externalEndpointIP,
					})
				Expect(err).To(BeNil())
				// It should have mapped the IP
				newIP := response.GetIp()
				Expect(newIP).ToNot(Equal(externalEndpointIP))

				// Add mapping to resource NatMapping
				// It is necessary to do it manually because AddMapping of NatMappingInflater
				// does not exist yet
				natMappingResource, err := getNatMappingResourcePerCluster(clusterID1)
				Expect(err).To(BeNil())
				err = unstructured.SetNestedField(natMappingResource.Object, map[string]interface{}{
					externalEndpointIP: newIP,
				}, "spec", "clusterMappings")
				Expect(err).To(BeNil())
				dynClient.Resource(liqonetapi.NatMappingGroupResource).Update(
					context.Background(),
					natMappingResource,
					v1.UpdateOptions{},
				)
				// Init mappings
				err = ipam.InitNatMappingsPerCluster(clusterID1)
				Expect(err).To(BeNil())

				// Terminate mappings with active mapping
				err = ipam.TerminateNatMappingsPerCluster(clusterID1)
				Expect(err).To(BeNil())

				// Check if resource exists
				natMappingResource, err = getNatMappingResourcePerCluster(clusterID1)
				Expect(k8serrors.IsNotFound(err)).To(BeTrue())

				// Check if cluster has been deleted from cluster list of endpoint
				ipamStorageUnstructured, err := getIpamStorageResource()

				// Get endpointMappings
				endpointMappings, found, err := unstructured.NestedStringMap(ipamStorageUnstructured.Object, "spec", "endpointMappings")
				Expect(err).To(BeNil())
				Expect(found).To(BeTrue())
				// Since the endpoint had only one mapping, the terminate should have deleted it.
				Expect(endpointMappings).ToNot(HaveKey(externalEndpointIP))
			})
		})
		Context("Terminate mappings when the cluster has an active mapping and"+
			"the endpoint is reflected in more clusters", func() {
			It("should not remove the mapping", func() {
				// Set PodCIDR
				err := ipam.SetPodCIDR(podCIDR)
				Expect(err).To(BeNil())

				_, err = ipam.GetExternalCIDR(24)
				Expect(err).To(BeNil())

				_, _, err = ipam.GetSubnetsPerCluster("10.0.1.0/24", "10.0.2.0/24", clusterID1)
				Expect(err).To(BeNil())
				_, _, err = ipam.GetSubnetsPerCluster("10.0.1.0/24", "10.0.2.0/24", clusterID2)
				Expect(err).To(BeNil())

				err = ipam.AddLocalSubnetsPerCluster("None", "None", clusterID1)
				Expect(err).To(BeNil())
				err = ipam.AddLocalSubnetsPerCluster("None", "None", clusterID2)
				Expect(err).To(BeNil())

				response, err := ipam.MapEndpointIP(context.Background(),
					&liqonet.MapRequest{
						ClusterID: clusterID1,
						Ip:        externalEndpointIP,
					})
				Expect(err).To(BeNil())
				// It should have mapped the IP
				newIPInCluster1 := response.GetIp()
				Expect(newIPInCluster1).ToNot(Equal(externalEndpointIP))

				response, err = ipam.MapEndpointIP(context.Background(),
					&liqonet.MapRequest{
						ClusterID: clusterID2,
						Ip:        externalEndpointIP,
					})
				Expect(err).To(BeNil())
				// It should have mapped the IP
				newIPInCluster2 := response.GetIp()
				Expect(newIPInCluster2).ToNot(Equal(externalEndpointIP))

				// Add mapping to resource NatMapping
				// It is necessary to do it manually because AddMapping of NatMappingInflater
				// does not exist yet
				natMappingResource, err := getNatMappingResourcePerCluster(clusterID1)
				Expect(err).To(BeNil())

				err = unstructured.SetNestedField(natMappingResource.Object, map[string]interface{}{
					externalEndpointIP: newIPInCluster1,
				}, "spec", "clusterMappings")
				Expect(err).To(BeNil())
				_, err = dynClient.Resource(liqonetapi.NatMappingGroupResource).Update(
					context.Background(),
					natMappingResource,
					v1.UpdateOptions{},
				)
				Expect(err).To(BeNil())

				// Cluster2
				natMappingResource, err = getNatMappingResourcePerCluster(clusterID2)
				Expect(err).To(BeNil())
				err = unstructured.SetNestedField(natMappingResource.Object, map[string]interface{}{
					externalEndpointIP: newIPInCluster2,
				}, "spec", "clusterMappings")
				Expect(err).To(BeNil())
				_, err = dynClient.Resource(liqonetapi.NatMappingGroupResource).Update(
					context.Background(),
					natMappingResource,
					v1.UpdateOptions{},
				)
				Expect(err).To(BeNil())

				// Init mappings.
				// The inflater will recover from resource.
				err = ipam.InitNatMappingsPerCluster(clusterID1)
				Expect(err).To(BeNil())
				// Init mappings
				err = ipam.InitNatMappingsPerCluster(clusterID2)
				Expect(err).To(BeNil())

				// Terminate mappings with active mapping
				err = ipam.TerminateNatMappingsPerCluster(clusterID1)
				Expect(err).To(BeNil())

				// Check if resource exists
				natMappingResource, err = getNatMappingResourcePerCluster(clusterID1)
				Expect(k8serrors.IsNotFound(err)).To(BeTrue())

				// Check if cluster has been deleted from cluster list of endpoint
				ipamStorageUnstructured, err := getIpamStorageResource()
				Expect(err).To(BeNil())
				ipamStorage := &liqonetapi.IpamStorage{}
				err = runtime.DefaultUnstructuredConverter.FromUnstructured(ipamStorageUnstructured.Object, ipamStorage)
				Expect(err).To(BeNil())

				// Get endpointMappings
				endpointMappings := ipamStorage.Spec.EndpointMappings
				Expect(err).To(BeNil())
				// Since the endpoint had more than one mapping, the terminate should not have deleted it.
				Expect(endpointMappings).To(HaveKey(externalEndpointIP))

				// Get endpoint
				endpointMapping := endpointMappings[externalEndpointIP]
				// Check if cluster exists in clusterMappings
				clusterMappings := endpointMapping.ClusterMappings
				Expect(clusterMappings).ToNot(HaveKey(clusterID1))
			})
		})
	})
})

func getNatMappingResourcePerCluster(clusterID string) (*unstructured.Unstructured, error) {
	list, err := dynClient.Resource(liqonetapi.NatMappingGroupResource).List(
		context.Background(),
		v1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s,%s=%s",
				consts.NatMappingResourceLabelKey,
				consts.NatMappingResourceLabelValue,
				consts.ClusterIDLabelName,
				clusterID),
		},
	)
	if err != nil {
		return nil, err
	}
	if len(list.Items) == 0 {
		return nil, k8serrors.NewNotFound(liqonetapi.NatMappingGroupResource.GroupResource(), "")
	}
	return &unstructured.Unstructured{Object: list.Items[0].Object}, nil
}

func getIpamStorageResource() (*unstructured.Unstructured, error) {
	list, err := dynClient.Resource(liqonetapi.IpamGroupResource).List(
		context.Background(),
		v1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s",
				consts.IpamStorageResourceLabelKey,
				consts.IpamStorageResourceLabelValue),
		},
	)
	if err != nil {
		return nil, err
	}
	if len(list.Items) == 0 {
		return nil, k8serrors.NewNotFound(liqonetapi.IpamGroupResource.GroupResource(), "")
	}
	return &unstructured.Unstructured{Object: list.Items[0].Object}, nil
}
