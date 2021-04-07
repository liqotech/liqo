package liqonet_test

import (
	"github.com/liqotech/liqo/pkg/liqonet"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
)

var ipam *liqonet.IPAM
var dynClient dynamic.Interface

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
				p, err := ipam.GetSubnetPerCluster("10.0.2.0/24", "cluster1")
				gomega.Expect(err).To(gomega.BeNil())
				// It should have been mapped to a new network belonging to a different pool
				gomega.Expect(p).ToNot(gomega.HavePrefix("10."))
			})
		})
		Context("When the reserved network belongs to a pool", func() {
			It("Should not be possible to acquire the same network for a cluster", func() {
				err := ipam.AcquireReservedSubnet("10.244.0.0/24")
				gomega.Expect(err).To(gomega.BeNil())
				err = ipam.AcquireReservedSubnet("10.0.2.0/24")
				gomega.Expect(err).To(gomega.BeNil())
				p, err := ipam.GetSubnetPerCluster("10.0.2.0/24", "cluster1")
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(p).ToNot(gomega.Equal("10.0.2.0/24"))
			})
			It("Should not be possible to acquire a larger network that contains it for a cluster", func() {
				err := ipam.AcquireReservedSubnet("10.244.0.0/24")
				gomega.Expect(err).To(gomega.BeNil())
				err = ipam.AcquireReservedSubnet("10.0.2.0/24")
				gomega.Expect(err).To(gomega.BeNil())
				p, err := ipam.GetSubnetPerCluster("10.0.0.0/16", "cluster1")
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(p).ToNot(gomega.Equal("10.0.0.0/16"))
			})
			It("Should not be possible to acquire a smaller network contained by it for a cluster", func() {
				err := ipam.AcquireReservedSubnet("10.244.0.0/24")
				gomega.Expect(err).To(gomega.BeNil())
				err = ipam.AcquireReservedSubnet("10.0.2.0/24")
				gomega.Expect(err).To(gomega.BeNil())
				p, err := ipam.GetSubnetPerCluster("10.0.2.0/25", "cluster1")
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(p).ToNot(gomega.Equal("10.0.2.0/25"))
			})
		})
	})

	Describe("AcquireSubnetPerCluster", func() {
		Context("When the remote cluster asks for a subnet not belonging to any pool", func() {
			Context("and the subnet has not already been assigned to any other cluster", func() {
				It("Should allocate the subnet itself, without mapping", func() {
					network, err := ipam.GetSubnetPerCluster("11.0.0.0/16", "cluster1")
					gomega.Expect(err).To(gomega.BeNil())
					gomega.Expect(network).To(gomega.Equal("11.0.0.0/16"))
				})
			})
			Context("and the subnet has already been assigned to another cluster", func() {
				Context("and there is an available network with the same mask length in one pool", func() {
					It("should map the requested network to another network taken by the pool", func() {
						network, err := ipam.GetSubnetPerCluster("11.0.0.0/16", "cluster1")
						gomega.Expect(err).To(gomega.BeNil())
						gomega.Expect(network).To(gomega.Equal("11.0.0.0/16"))
						network, err = ipam.GetSubnetPerCluster("11.0.0.0/16", "cluster2")
						gomega.Expect(err).To(gomega.BeNil())
						gomega.Expect(network).To(gomega.HavePrefix("10."))
						gomega.Expect(network).To(gomega.HaveSuffix("/16"))
					})
				})
				Context("and there is not an available network with the same mask length in any pool", func() {
					It("should fail to allocate a network", func() {
						// Fill pool #1
						network, err := ipam.GetSubnetPerCluster("10.0.0.0/9", "cluster1")
						gomega.Expect(err).To(gomega.BeNil())
						gomega.Expect(network).To(gomega.Equal("10.0.0.0/9"))
						network, err = ipam.GetSubnetPerCluster("10.128.0.0/9", "cluster2")
						gomega.Expect(err).To(gomega.BeNil())
						gomega.Expect(network).To(gomega.Equal("10.128.0.0/9"))

						// Fill pool #2
						network, err = ipam.GetSubnetPerCluster("192.168.0.0/17", "cluster3")
						gomega.Expect(err).To(gomega.BeNil())
						gomega.Expect(network).To(gomega.Equal("192.168.0.0/17"))
						network, err = ipam.GetSubnetPerCluster("192.168.128.0/17", "cluster4")
						gomega.Expect(err).To(gomega.BeNil())
						gomega.Expect(network).To(gomega.Equal("192.168.128.0/17"))

						// Fill pool #3
						network, err = ipam.GetSubnetPerCluster("172.16.0.0/13", "cluster5")
						gomega.Expect(err).To(gomega.BeNil())
						gomega.Expect(network).To(gomega.Equal("172.16.0.0/13"))
						network, err = ipam.GetSubnetPerCluster("172.24.0.0/13", "cluster6")
						gomega.Expect(err).To(gomega.BeNil())
						gomega.Expect(network).To(gomega.Equal("172.24.0.0/13"))

						// Cluster network request
						network, err = ipam.GetSubnetPerCluster("11.0.0.0/16", "cluster7")
						gomega.Expect(err).To(gomega.BeNil())
						gomega.Expect(network).To(gomega.Equal("11.0.0.0/16"))

						// Another cluster asks for the same network
						_, err = ipam.GetSubnetPerCluster("11.0.0.0/16", "cluster8")
						gomega.Expect(err).ToNot(gomega.BeNil())
					})
				})
			})
		})
		Context("When the remote clusters asks for a subnet which is equal to a pool", func() {
			It("should map it to another network", func() {
				network, err := ipam.GetSubnetPerCluster("172.16.0.0/12", "cluster1")
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(network).ToNot(gomega.Equal("172.16.0.0/12"))
			})
		})
		Context("When the remote cluster asks for a subnet belonging to a network in the pool", func() {
			Context("and all pools are full", func() {
				It("should not allocate the network", func() {
					// Fill pool #1
					network, err := ipam.GetSubnetPerCluster("10.0.0.0/9", "cluster1")
					gomega.Expect(err).To(gomega.BeNil())
					gomega.Expect(network).To(gomega.Equal("10.0.0.0/9"))
					network, err = ipam.GetSubnetPerCluster("10.128.0.0/9", "cluster2")
					gomega.Expect(err).To(gomega.BeNil())
					gomega.Expect(network).To(gomega.Equal("10.128.0.0/9"))

					// Fill pool #2
					network, err = ipam.GetSubnetPerCluster("192.168.0.0/17", "cluster3")
					gomega.Expect(err).To(gomega.BeNil())
					gomega.Expect(network).To(gomega.Equal("192.168.0.0/17"))
					network, err = ipam.GetSubnetPerCluster("192.168.128.0/17", "cluster4")
					gomega.Expect(err).To(gomega.BeNil())
					gomega.Expect(network).To(gomega.Equal("192.168.128.0/17"))

					// Fill pool #3
					network, err = ipam.GetSubnetPerCluster("172.16.0.0/13", "cluster5")
					gomega.Expect(err).To(gomega.BeNil())
					gomega.Expect(network).To(gomega.Equal("172.16.0.0/13"))
					network, err = ipam.GetSubnetPerCluster("172.24.0.0/13", "cluster6")
					gomega.Expect(err).To(gomega.BeNil())
					gomega.Expect(network).To(gomega.Equal("172.24.0.0/13"))

					// Cluster network request
					_, err = ipam.GetSubnetPerCluster("10.1.0.0/16", "cluster7")
					gomega.Expect(err).ToNot(gomega.BeNil())
				})
			})
			Context("and the subnet has not already been assigned to any other cluster", func() {
				It("Should allocate the subnet itself, without mapping", func() {
					network, err := ipam.GetSubnetPerCluster("10.0.0.0/16", "cluster1")
					gomega.Expect(err).To(gomega.BeNil())
					gomega.Expect(network).To(gomega.Equal("10.0.0.0/16"))
				})
			})
			Context("and the subnet has already been assigned to another cluster", func() {
				Context("and there is an available network with the same mask length in one pool", func() {
					It("should map the requested network to another network taken by the pool", func() {
						network, err := ipam.GetSubnetPerCluster("10.0.0.0/16", "cluster1")
						gomega.Expect(err).To(gomega.BeNil())
						gomega.Expect(network).To(gomega.Equal("10.0.0.0/16"))
						network, err = ipam.GetSubnetPerCluster("10.0.0.0/16", "cluster2")
						gomega.Expect(err).To(gomega.BeNil())
						gomega.Expect(network).To(gomega.HavePrefix("10."))
						gomega.Expect(network).To(gomega.HaveSuffix("/16"))
					})
				})
				Context("and there is not an available network with the same mask length in any pool", func() {
					It("should allocate it as a new prefix", func() {
						network, err := ipam.GetSubnetPerCluster("10.0.0.0/16", "cluster1")
						gomega.Expect(err).To(gomega.BeNil())
						gomega.Expect(network).To(gomega.Equal("10.0.0.0/16"))
						_, err = ipam.GetSubnetPerCluster("10.0.0.0/16", "cluster2")
						gomega.Expect(err).To(gomega.BeNil())
					})
				})
			})
		})
	})
	Describe("FreeSubnetPerCluster", func() {
		Context("Freeing a cluster network that exists", func() {
			It("Should successfully free the subnet", func() {
				network, err := ipam.GetSubnetPerCluster("10.0.1.0/24", "cluster1")
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(network).To(gomega.Equal("10.0.1.0/24"))
				err = ipam.FreeSubnetPerCluster("cluster1")
				gomega.Expect(err).To(gomega.BeNil())
				network, err = ipam.GetSubnetPerCluster("10.0.1.0/24", "cluster2")
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(network).To(gomega.Equal("10.0.1.0/24"))
			})
		})
		Context("Freeing a cluster network that does not exists", func() {
			It("Should return an error", func() {
				err := ipam.FreeSubnetPerCluster("cluster1")
				gomega.Expect(err.Error()).To(gomega.Equal("network is not assigned to any cluster"))
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
			It("Should return an error", func() {
				err := ipam.FreeReservedSubnet("10.0.1.0/24")
				gomega.Expect(err.Error()).To(gomega.Equal("network 10.0.1.0/24 is already available"))
			})
		})
	})
	Describe("Re-scheduling of network manager", func() {
		It("ipam should retrieve configuration by resource", func() {
			// Assign network to cluster
			network, err := ipam.GetSubnetPerCluster("10.0.1.0/24", "cluster1")
			gomega.Expect(err).To(gomega.BeNil())
			gomega.Expect(network).To(gomega.Equal("10.0.1.0/24"))

			// Simulate re-scheduling
			ipam = liqonet.NewIPAM()
			err = ipam.Init(liqonet.Pools, dynClient)
			gomega.Expect(err).To(gomega.BeNil())

			// Another cluster asks for the same network
			network, err = ipam.GetSubnetPerCluster("10.0.1.0/24", "cluster2")
			gomega.Expect(err).To(gomega.BeNil())
			gomega.Expect(network).ToNot(gomega.Equal("10.0.1.0/24"))
		})
	})
})
